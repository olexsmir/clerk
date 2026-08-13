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

// analysisFor returns the cached analysis for an open doc, rebuilds when the doc or a file it inclues changed.
func (s *server) analysisFor(u uri.URI) *analyzer.Analysis {
	s.mu.Lock()
	state, ok := s.openDocs[u]
	if !ok {
		s.mu.Unlock()
		return nil
	}
	if !state.dirty {
		an := state.analysis
		s.mu.Unlock()
		return an
	}
	text := state.text
	version := state.version
	s.mu.Unlock()

	an := analyzer.Build(s.loader.ResolveBytes(u.Path(), []byte(text)))

	s.mu.Lock()
	state, ok = s.openDocs[u]
	if !ok || state.version != version {
		// editot or closed while building. doc stays dirty so te request rebuilds
		s.mu.Unlock()
		return an
	}
	state.analysis = an
	state.dirty = false
	state.paths = make(map[string]bool, len(an.Files))
	for _, pf := range an.Files {
		state.paths[journal.CanonicalPath(pf.Path)] = true
	}
	s.openDocs[u] = state
	s.mu.Unlock()
	return an
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
			DefinitionProvider:         protocol.Boolean(true),
			HoverProvider:              protocol.Boolean(true),
			RenameProvider: &protocol.RenameOptions{
				PrepareProvider: new(true),
			},
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
