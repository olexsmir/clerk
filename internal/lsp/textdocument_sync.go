package lsp

import (
	"context"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

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
	s.closeDoc(params.TextDocument.URI)
	s.scheduleDiagnostics(ctx)
	return nil
}

func (s *server) DidSave(ctx context.Context, params *protocol.DidSaveTextDocumentParams) error {
	return nil
}

// Document management

func (s *server) openDoc(u uri.URI, text string, version int32, langID protocol.LanguageKind) {
	s.mu.Lock()
	s.openDocs[u] = docState{
		text:       text,
		version:    version,
		languageID: langID,
	}
	s.mu.Unlock()
}

func (s *server) updateDoc(u uri.URI, version int32, changes []protocol.TextDocumentContentChangeEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.openDocs[u]
	if !ok {
		return
	}
	state.version = version
	for _, ch := range changes {
		switch ev := ch.(type) {
		case *protocol.TextDocumentContentChangeWholeDocument:
			state.text = ev.Text
			state.semTokens = nil
		case *protocol.TextDocumentContentChangePartial:
			_ = ev // TODO: incremental edit support
		}
	}
	s.openDocs[u] = state
}

func (s *server) closeDoc(u uri.URI) {
	s.mu.Lock()
	delete(s.openDocs, u)
	s.mu.Unlock()
}

func (s *server) getDocText(u uri.URI) (string, bool) {
	s.mu.Lock()
	state, ok := s.openDocs[u]
	s.mu.Unlock()
	return state.text, ok
}

func (s *server) getDocState(u uri.URI) (docState, bool) {
	s.mu.Lock()
	state, ok := s.openDocs[u]
	s.mu.Unlock()
	return state, ok
}

// parseJournalStr parses a raw journal string into an AST.
func parseJournalStr(content string) *ast.Journal {
	l := lexer.New("", []byte(content))
	p := parser.New(l)
	return p.ParseJournal()
}
