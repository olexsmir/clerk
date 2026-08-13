package lsp

import (
	"context"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/analyzer"
	"olexsmir.xyz/clerk/internal/lsp/lsputil"
	"olexsmir.xyz/clerk/journal"
	"olexsmir.xyz/clerk/journal/ast"
	"olexsmir.xyz/clerk/journal/lexer"
	"olexsmir.xyz/clerk/journal/parser"
)

func (s *server) DidOpen(ctx context.Context, params *protocol.DidOpenTextDocumentParams) error {
	s.openDoc(params.TextDocument.URI, params.TextDocument.Text, params.TextDocument.Version, params.TextDocument.LanguageID)
	s.scheduleDiagnostics(ctx)
	return nil
}

func (s *server) DidChange(ctx context.Context, params *protocol.DidChangeTextDocumentParams) error {
	s.updateDoc(params.TextDocument.URI, params.TextDocument.Version, params.ContentChanges)
	s.scheduleDiagnostics(ctx)
	return nil
}

func (s *server) DidClose(ctx context.Context, params *protocol.DidCloseTextDocumentParams) error {
	u := params.TextDocument.URI
	s.closeDoc(u)

	// clear closed doc's diagnostics; dependents rebuild from disk since the buffer text is gone.
	s.markDependentsDirty(u)
	if err := s.client.PublishDiagnostics(ctx, &protocol.PublishDiagnosticsParams{
		URI:         u,
		Diagnostics: []protocol.Diagnostic{},
	}); err != nil {
		s.log.Warn("clear diagnostics failed", "uri", u, "err", err)
	}

	s.scheduleDiagnostics(ctx)
	return nil
}

func (s *server) DidSave(ctx context.Context, params *protocol.DidSaveTextDocumentParams) error {
	return nil
}

// Document management

type docState struct {
	text       string
	version    int32
	languageID protocol.LanguageKind
	semTokens  []semanticToken    // cached semantic tokens
	lineIdx    *lsputil.LineIndex // cached line index for the text
	analysis   *analyzer.Analysis // cached analysis, nil until first build
	paths      map[string]bool    // canonical paths of every file in the cached analysis
	dirty      bool               // true while the cached analysis may not reflect the current text
}

func (s *server) openDoc(u uri.URI, text string, version int32, langID protocol.LanguageKind) {
	s.mu.Lock()
	s.openDocs[u] = docState{
		text:       text,
		version:    version,
		languageID: langID,
		lineIdx:    lsputil.NewLineIndex(text),
		dirty:      true,
	}
	s.mu.Unlock()
}

func (s *server) updateDoc(u uri.URI, version int32, changes []protocol.TextDocumentContentChangeEvent) {
	s.mu.Lock()
	state, ok := s.openDocs[u]
	if !ok {
		s.mu.Unlock()
		return
	}
	state.version = version
	for _, ch := range changes {
		switch ev := ch.(type) {
		case *protocol.TextDocumentContentChangeWholeDocument:
			state.text = ev.Text
			state.semTokens = nil
			state.lineIdx = lsputil.NewLineIndex(ev.Text)
		case *protocol.TextDocumentContentChangePartial:
			_ = ev // TODO: incremental edit support
		}
	}
	state.analysis = nil
	state.dirty = true
	s.openDocs[u] = state
	s.mu.Unlock()

	s.markDependentsDirty(u)
}

// markDependentsDirty dirties every open doc whose cached analysis includes u,
// e.g. after u is edited or closed.
func (s *server) markDependentsDirty(u uri.URI) {
	canon := journal.CanonicalPath(u.Path())
	s.mu.Lock()
	for du, dstate := range s.openDocs {
		if dstate.paths[canon] {
			dstate.dirty = true
			s.openDocs[du] = dstate
		}
	}
	s.mu.Unlock()
}

func (s *server) getDocState(u uri.URI) (docState, bool) {
	s.mu.Lock()
	state, ok := s.openDocs[u]
	s.mu.Unlock()
	return state, ok
}

func (s *server) closeDoc(u uri.URI) {
	s.mu.Lock()
	delete(s.openDocs, u)
	s.mu.Unlock()
}

func parseJournalStr(content string) *ast.Journal {
	l := lexer.New("", []byte(content))
	p := parser.New(l)
	return p.ParseJournal()
}
