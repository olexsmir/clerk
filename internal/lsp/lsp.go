package lsp

import (
	"context"
	"io"
	"log/slog"
	"os"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/linter"
	"olexsmir.xyz/clerk/journal"
)

type Server struct{ server *server }

func NewServer(version string) Server {
	return Server{
		&server{
			name:    "clerk",
			version: version,

			openDocs: make(map[uri.URI]docState),

			linter: linter.NewLinter(linter.Rules),
			loader: journal.NewLoader(),

			log: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{})),
		},
	}
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
