package lsp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/settings"
	"olexsmir.xyz/clerk/internal/xdg"
	"olexsmir.xyz/clerk/journal"
)

type Server struct{ server *server }

func NewServer(version, configPath string) (Server, error) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if logFile, err := openLogFile(); err == nil {
		logger = slog.New(slog.NewTextHandler(logFile, nil))
	}

	s, warns, err := settings.Load(configPath)
	if err != nil {
		return Server{}, fmt.Errorf("load config %q: %w", configPath, err)
	}
	for _, w := range warns {
		logger.Warn("config", "warning", w)
	}

	srv := &server{
		name:    "clerk",
		version: version,

		openDocs: make(map[uri.URI]docState),

		settings: s,
		loader:   journal.NewLoader(),

		log: logger,
	}
	srv.loader.ContentProvider = srv.bufferContent
	return Server{srv}, nil
}

func (s *Server) Run(ctx context.Context, stdin io.ReadCloser, stdout io.WriteCloser) error {
	stream := jsonrpc2.NewStream(readWriterCloser{
		Reader: stdin,
		Writer: stdout,
		Closer: stdin,
	})

	_, conn, client := protocol.NewServer(ctx, s.server, stream)
	defer conn.Close()

	s.server.client = client

	<-conn.Done()
	return conn.Err()
}

// bufferContent returns the open buffer text for a path, if any.
// called by the loader during include resolution; must NOT hold the loader lock.
func (s *server) bufferContent(path string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.openDocs[uri.File(path)]
	return []byte(st.text), ok
}

type readWriterCloser struct {
	io.Reader
	io.Writer
	io.Closer
}

func openLogFile() (*os.File, error) {
	dir, err := xdg.StateDir()
	if err != nil {
		return nil, err
	}
	dir = filepath.Join(dir, "clerk")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(dir, "lsp.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}
