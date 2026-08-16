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

	mu            sync.RWMutex
	config        Config
	openDocs      map[uri.URI]docState
	diagCancel    context.CancelFunc
	dynFileWather bool
}

// analysisFor returns the cached analysis for an open doc, rebuilds when the doc or a file it inclues changed.
func (s *server) analysisFor(u uri.URI) *analyzer.Analysis {
	s.mu.RLock()
	state, ok := s.openDocs[u]
	if !ok {
		s.mu.RUnlock()
		return nil
	}
	if !state.dirty {
		an := state.analysis
		s.mu.RUnlock()
		return an
	}
	text := state.text
	version := state.version
	s.mu.RUnlock()

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
	if w := params.Capabilities.Workspace; w != nil {
		if wf := w.DidChangeWatchedFiles; wf != nil {
			s.dynFileWather = wf.DynamicRegistration != nil && *wf.DynamicRegistration
		}
	}

	s.applySettings(params.InitializationOptions)
	full := protocol.SemanticTokensOptionsFull(protocol.Boolean(true))
	if td := params.Capabilities.TextDocument; td != nil {
		if fd, ok := td.SemanticTokens.Requests.Full.(*protocol.ClientSemanticTokensRequestFullDelta); ok && fd.Delta != nil && *fd.Delta {
			full = &protocol.SemanticTokensFullDelta{Delta: new(true)}
		}
	}

	return &protocol.InitializeResult{
		ServerInfo: protocol.ServerInfo{
			Name:    s.name,
			Version: protocol.NewOptional(s.version),
		},
		Capabilities: protocol.ServerCapabilities{
			DocumentFormattingProvider: &protocol.DocumentFormattingOptions{},
			DefinitionProvider:         protocol.Boolean(true),
			HoverProvider:              protocol.Boolean(true),
			ReferencesProvider:         protocol.Boolean(true),
			WorkspaceSymbolProvider:    protocol.Boolean(true),
			DocumentSymbolProvider:     protocol.Boolean(true),
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
				Full:   full,
			},
		},
	}, nil
}

func (s *server) Initialized(ctx context.Context, params *protocol.InitializedParams) error {
	if s.dynFileWather {
		go s.registerFileWatchers(context.Background())
	}
	s.scheduleDiagnostics(ctx)
	return nil
}

func (s *server) DidChangeWatchedFiles(ctx context.Context, params *protocol.DidChangeWatchedFilesParams) error {
	for _, change := range params.Changes {
		u := change.URI
		path := u.Path()
		if path == "" {
			continue
		}
		if _, isOpen := s.getDocState(u); isOpen {
			continue // editor buffer is authoritative for open docs
		}
		s.loader.InvalidateFile(path)
		s.markDependentsDirty(u)
	}
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

func (s *server) registerFileWatchers(ctx context.Context) {
	if s.client == nil {
		return
	}
	watchers := make([]protocol.FileSystemWatcher, 0, len(journal.SupportedExtensions))
	for _, ext := range journal.SupportedExtensions {
		watchers = append(watchers, protocol.FileSystemWatcher{GlobPattern: protocol.Pattern("**/*" + ext)})
	}
	options, err := protocol.Marshal(protocol.DidChangeWatchedFilesRegistrationOptions{Watchers: watchers})
	if err != nil {
		return
	}
	if err := s.client.RegisterCapability(ctx, &protocol.RegistrationParams{
		Registrations: []protocol.Registration{{
			ID:              "clerk.watchedFiles",
			Method:          protocol.MethodWorkspaceDidChangeWatchedFiles,
			RegisterOptions: protocol.LSPAny(options),
		}},
	}); err != nil {
		s.log.Warn("registering file watchers failed", "err", err)
	}
}

func (s *server) applySettings(v protocol.LSPAny) {
	s.mu.Lock()
	if err := s.config.merge(v); err != nil {
		s.log.Error("failed to merge config", "err", err)
	}
	s.mu.Unlock()
}
