package lsp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/go-json-experiment/json"
	"github.com/pelletier/go-toml/v2"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/internal/settings"
	"olexsmir.xyz/clerk/journal"
)

type server struct {
	protocol.UnimplementedServer

	client protocol.Client
	log    *slog.Logger
	conn   jsonrpc2.Conn // set in Run; Exit closes it to end the session

	stateMu       sync.Mutex
	state         serverState
	version, name string

	settings settings.Settings
	loader   *journal.Loader

	mu             sync.RWMutex
	openDocs       map[uri.URI]docState
	diagCancel     context.CancelFunc
	dynFileWatcher bool
	configPath     string
}

func parsedFileFor(an *analyzer.Analysis, path string) *journal.ParsedFile {
	for _, pf := range an.Files {
		if pf.Path == path {
			return pf
		}
	}
	return nil
}

// analysisFor returns the cached analysis for an open doc, rebuilding when the doc or a file it includes changed.
func (s *server) analysisFor(u uri.URI) *analyzer.Analysis {
	s.mu.RLock()
	state, ok := s.openDocs[u]
	if !ok {
		s.mu.RUnlock()
		return nil
	}
	if !state.dirty {
		an := state.cache.analysis
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
		// editor closed or edited while building; the doc stays dirty so the next request rebuilds
		s.mu.Unlock()
		return an
	}
	state.cache = newAnalysisCache(an)
	state.dirty = false
	s.openDocs[u] = state
	s.mu.Unlock()
	return an
}

func (s *server) Initialize(ctx context.Context, params *protocol.InitializeParams) (*protocol.InitializeResult, error) {
	if w := params.Capabilities.Workspace; w != nil {
		if wf := w.DidChangeWatchedFiles; wf != nil {
			s.dynFileWatcher = wf.DynamicRegistration != nil && *wf.DynamicRegistration
		}
	}

	if err := s.applySettings(ctx, params.InitializationOptions); err != nil {
		return nil, err
	}
	td := params.Capabilities.TextDocument
	full := protocol.SemanticTokensOptionsFull(protocol.Boolean(true))
	if td != nil {
		if fd, ok := td.SemanticTokens.Requests.Full.(*protocol.ClientSemanticTokensRequestFullDelta); ok && fd.Delta != nil && *fd.Delta {
			full = &protocol.SemanticTokensFullDelta{Delta: new(true)}
		}
	}

	// RenameOptions may only be specified when the client states prepare support.
	renameProvider := protocol.RenameProvider(&protocol.RenameOptions{PrepareProvider: new(true)})
	if td == nil || td.Rename == nil || td.Rename.PrepareSupport == nil || !*td.Rename.PrepareSupport {
		renameProvider = protocol.Boolean(true)
	}

	s.stateMu.Lock()
	if s.state != stateNew {
		s.stateMu.Unlock()
		return nil, errInitializeOnce
	}
	s.state = stateInitialized
	s.stateMu.Unlock()
	if p := params.ProcessID; p != nil {
		s.watchParent(*p)
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
			FoldingRangeProvider:       protocol.Boolean(true),
			SelectionRangeProvider:     protocol.Boolean(true),
			RenameProvider:             renameProvider,
			CompletionProvider: &protocol.CompletionOptions{
				TriggerCharacters: []string{":", "@"},
			},
			TextDocumentSync: &protocol.TextDocumentSyncOptions{
				OpenClose: new(true),
				Change:    new(protocol.TextDocumentSyncKindIncremental),
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
	if s.dynFileWatcher {
		go s.registerFileWatchers(context.Background())
	}
	s.applyConfigFile(ctx)
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
	return s.applySettings(ctx, params.Settings)
}

func (s *server) Shutdown(ctx context.Context) error {
	s.stateMu.Lock()
	if s.state == stateInitialized {
		s.state = stateShutdownRequested
	}
	s.stateMu.Unlock()
	return nil
}

func (s *server) Exit(context.Context) error {
	s.stateMu.Lock()
	if s.state < stateExited {
		if s.state == stateShutdownRequested {
			s.state = stateExitedAfterShutdown
		} else {
			s.state = stateExited
		}
	}
	s.stateMu.Unlock()
	go s.conn.Close()
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

func (s *server) applySettings(ctx context.Context, v protocol.LSPAny) error {
	if len(v) == 0 {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(v, &raw); err != nil {
		return fmt.Errorf("invalid settings: %w", err)
	}
	s.mu.Lock()
	warns, err := s.settings.ApplyLSP(raw)
	s.mu.Unlock()
	for _, w := range warns {
		s.reportConfigProblem(ctx, protocol.MessageTypeWarning, w)
	}
	return err
}

func (s *server) applyConfigFile(ctx context.Context) {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		s.reportConfigError(ctx, err)
		return
	}
	var raw map[string]any
	if uerr := toml.Unmarshal(data, &raw); uerr != nil {
		s.reportConfigError(ctx, uerr)
		return
	}
	s.mu.Lock()
	warns, err := s.settings.Apply(raw)
	s.mu.Unlock()
	if err != nil {
		s.reportConfigError(ctx, err)
	}
	for _, w := range warns {
		s.reportConfigProblem(ctx, protocol.MessageTypeWarning, w)
	}
}

func (s *server) reportConfigError(ctx context.Context, err error) {
	s.reportConfigProblem(ctx, protocol.MessageTypeError, "config "+s.configPath+": "+err.Error())
}

func (s *server) reportConfigProblem(ctx context.Context, typ protocol.MessageType, msg string) {
	lvl := slog.LevelWarn
	if typ == protocol.MessageTypeError {
		lvl = slog.LevelError
	}
	s.log.Log(ctx, lvl, "config", "message", msg)
	if s.client == nil {
		return
	}
	if err := s.client.ShowMessage(ctx, &protocol.ShowMessageParams{Type: typ, Message: msg}); err != nil {
		s.log.Warn("window/showMessage failed", "err", err)
	}
}

func (s *server) semanticHighlightingEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings.SemanticHighlighting
}

func (s *server) latinToCyrillicCompletionEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings.LatinToCyrillicCompletion
}
