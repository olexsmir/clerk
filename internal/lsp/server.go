package lsp

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/linter"
	"olexsmir.xyz/clerk/journal"
	"olexsmir.xyz/clerk/journal/semantic"
)

type server struct {
	protocol.UnimplementedServer

	client protocol.Client
	log    *slog.Logger

	version, name string

	linter    *linter.Linter
	loader    *journal.Loader
	debouncer *time.Timer

	mu          sync.Mutex
	openDocs    map[uri.URI]docState
	semanticCtx *semantic.Context
}

type docState struct {
	text       string
	version    int32
	languageID protocol.LanguageKind
}

func (s *server) Initialize(ctx context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	return &protocol.InitializeResult{
		ServerInfo: protocol.ServerInfo{
			Name:    s.name,
			Version: protocol.NewOptional(s.version),
		},
		Capabilities: protocol.ServerCapabilities{
			DocumentFormattingProvider: &protocol.DocumentFormattingOptions{},
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				OpenClose: new(true),
				Change:    new(protocol.TextDocumentSyncKindFull),
			},
		},
	}, nil
}

func (s *server) Initialized(ctx context.Context, params *protocol.InitializedParams) error {
	s.scheduleWorkspaceRebuild(ctx)
	return nil
}

func (s *server) Shutdown(ctx context.Context) error {
	s.cancelWorkspaceRebuild()
	return nil
}

func (s *server) Exit(ctx context.Context) error {
	return nil
}
