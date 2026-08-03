package lsp

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/linter"
	"olexsmir.xyz/clerk/internal/xdg"
	"olexsmir.xyz/clerk/journal"
	"olexsmir.xyz/clerk/journal/printer"
)

type Server struct{ server *server }

func NewServer(version string) Server {
	var logger *slog.Logger
	logFile, err := openLogFile()
	if err == nil {
		logger = slog.New(slog.NewTextHandler(logFile, nil))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}

	return Server{
		&server{
			name:    "clerk",
			version: version,

			openDocs: make(map[uri.URI]docState),

			config:  DefaultConfig,
			linter:  linter.NewLinter(linter.Rules),
			loader:  journal.NewLoader(),
			printer: printer.DefaultConfig,

			log: logger,
		},
	}
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

type readWriterCloser struct {
	io.Reader
	io.Writer
	io.Closer
}
