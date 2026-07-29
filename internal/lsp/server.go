package lsp

import (
	"context"
	"log/slog"
	"sync"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/internal/linter"
	"olexsmir.xyz/clerk/journal"
	"olexsmir.xyz/clerk/journal/ast"
	"olexsmir.xyz/clerk/journal/printer"
)

type server struct {
	protocol.UnimplementedServer

	client protocol.Client
	log    *slog.Logger

	version, name string

	linter  *linter.Linter
	loader  *journal.Loader
	printer *printer.Config

	mu       sync.Mutex
	openDocs map[uri.URI]docState
	current  *analyzer.Analysis

	diagMu     sync.Mutex
	diagCancel context.CancelFunc
}

type docState struct {
	text       string
	version    int32
	languageID protocol.LanguageKind
	journal    *ast.Journal // cached parse; nil on parse failure
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
			SemanticTokensProvider: &protocol.SemanticTokensOptions{
				Legend: getSemanticTokensLegend(),
				Range:  protocol.Boolean(true),
				Full:   protocol.Boolean(true),
			},
		},
	}, nil
}

func (s *server) Initialized(ctx context.Context, params *protocol.InitializedParams) error {
	s.scheduleDiagnostics(ctx)
	return nil
}

func (s *server) Shutdown(ctx context.Context) error {
	return nil
}

func (s *server) Exit(ctx context.Context) error {
	return nil
}
