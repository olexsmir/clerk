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

	"olexsmir.xyz/clerk/internal/settings"
	"olexsmir.xyz/clerk/internal/xdg"
	"olexsmir.xyz/clerk/journal"
)

type Server struct{ server *server }

func NewServer(version string, configPath string) (Server, error) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if logFile, err := openLogFile(); err == nil {
		logger = slog.New(slog.NewTextHandler(logFile, nil))
	}

	srv := &server{
		name:    "clerk",
		version: version,

		openDocs: make(map[uri.URI]docState),

		settings:   settings.DefaultConfig,
		configPath: configPath,
		loader:     journal.NewLoader(),

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
	conn := jsonrpc2.NewConn(stream, jsonrpc2.WithCodec(lspCodec{}))
	s.server.client = protocol.ClientDispatcher(conn)
	s.server.conn = conn
	conn.Go(ctx, protocol.Handlers(s.server.lifecycle(protocol.ServerHandler(s.server, jsonrpc2.MethodNotFoundHandler))))

	<-conn.Done()
	s.server.stateMu.Lock()
	st := s.server.state
	s.server.stateMu.Unlock()
	if st >= stateExited {
		return &ExitError{Code: st.ExitCode()}
	}
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

// lspCodec mirros go.lsp.dev/protocol wire codec, so Run can install the lifecycle guard.
type lspCodec struct{}

func (lspCodec) Marshal(v any) ([]byte, error) {
	switch m := v.(type) {
	case jsonrpc2.RawMessage:
		if m == nil {
			return []byte("null"), nil
		}
		return m, nil
	case *jsonrpc2.RawMessage:
		if m == nil || *m == nil {
			return []byte("null"), nil
		}
		return *m, nil
	}
	return protocol.Marshal(v)
}

func (lspCodec) Unmarshal(data []byte, v any) error {
	if p, ok := v.(*jsonrpc2.RawMessage); ok {
		b := make(jsonrpc2.RawMessage, len(data))
		copy(b, data)
		*p = b
		return nil
	}
	return protocol.Unmarshal(data, v)
}
