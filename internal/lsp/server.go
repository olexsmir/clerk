package lsp

import (
	"context"
	"log/slog"
	"maps"
	"sync"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/internal/linter"
	"olexsmir.xyz/clerk/journal"
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

	mu         sync.Mutex
	openDocs   map[uri.URI]docState
	current    *analyzer.Analysis
	diagCancel context.CancelFunc

	cfgMu  sync.RWMutex
	config Config
}

func (s *server) analysis() *analyzer.Analysis {
	s.mu.Lock()
	a := s.current
	s.mu.Unlock()
	if a != nil {
		return a
	}
	return s.buildAnalysis()
}

func (s *server) buildAnalysis() *analyzer.Analysis {
	s.mu.Lock()
	docs := make(map[uri.URI]docState, len(s.openDocs))
	maps.Copy(docs, s.openDocs)
	s.mu.Unlock()

	var a *analyzer.Analysis
	for duri, state := range docs {
		rj := s.loader.ResolveBytes(duri.Path(), []byte(state.text))
		if a == nil {
			a = analyzer.Build(rj)
		}
	}
	return a
}

func (s *server) Initialize(ctx context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	s.applySettings(params.InitializationOptions)
	return &protocol.InitializeResult{
		ServerInfo: protocol.ServerInfo{
			Name:    s.name,
			Version: protocol.NewOptional(s.version),
		},
		Capabilities: protocol.ServerCapabilities{
			DocumentFormattingProvider: &protocol.DocumentFormattingOptions{},
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{":", "@"},
			},
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

func (s *server) DidChangeConfiguration(ctx context.Context, params *protocol.DidChangeConfigurationParams) error {
	s.applySettings(params.Settings)
	return nil
}

func (s *server) Shutdown(ctx context.Context) error {
	return nil
}

func (s *server) Exit(ctx context.Context) error {
	return nil
}

func (s *server) applySettings(v protocol.LSPAny) {
	s.cfgMu.Lock()
	if err := s.config.merge(v); err != nil {
		s.log.Error("failed to merge config", "err", err)
	}
	s.cfgMu.Unlock()
}
