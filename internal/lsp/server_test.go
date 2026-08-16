package lsp

import (
	"context"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"olexsmir.xyz/clerk/internal/testutil"
)

func TestAnalysisFor_CachedAndRebuilt(t *testing.T) {
	u := uri.File(filepath.Join(t.TempDir(), "a.journal"))
	srv := NewServer("test")
	srv.server.openDoc(u, "2024-01-01 t\n    expenses:food  $10\n    assets:cash\n", 1, "journal")

	a1 := srv.server.analysisFor(u)
	if a1 == nil {
		t.Fatal("analysisFor returned nil")
	}
	if a2 := srv.server.analysisFor(u); a2 != a1 {
		t.Error("cached analysis not reused")
	}

	srv.server.updateDoc(u, 2, []protocol.TextDocumentContentChangeEvent{
		&protocol.TextDocumentContentChangeWholeDocument{Text: "2024-01-02 t\n    expenses:travel  $20\n    assets:cash\n"},
	})
	a3 := srv.server.analysisFor(u)
	if a3 == a1 {
		t.Error("edit did not rebuild the analysis")
	}
	if !slices.Contains(a3.AccountNames, "expenses:travel") || slices.Contains(a3.AccountNames, "expenses:food") {
		t.Errorf("stale accounts after edit: %v", a3.AccountNames)
	}
}

func TestAnalysisFor_DependentDirty(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.journal")
	main := filepath.Join(dir, "main.journal")
	testutil.WriteFile(t, base, []byte("2024-01-01 t\n    expenses:food  $10\n    assets:cash\n"))
	testutil.WriteFile(t, main, []byte("include base.journal\n"))

	srv := NewServer("test")
	uMain, uBase := uri.File(main), uri.File(base)
	srv.server.openDoc(uMain, "include base.journal\n", 1, "journal")
	srv.server.openDoc(uBase, "2024-01-01 t\n    expenses:food  $10\n    assets:cash\n", 1, "journal")

	aMain := srv.server.analysisFor(uMain)
	if !slices.Contains(aMain.AccountNames, "assets:cash") {
		t.Fatalf("main analysis missing included account: %v", aMain.AccountNames)
	}

	srv.server.updateDoc(uBase, 2, []protocol.TextDocumentContentChangeEvent{
		&protocol.TextDocumentContentChangeWholeDocument{Text: "2024-01-01 t\n    expenses:food  $10\n    assets:bank\n"},
	})
	aMain2 := srv.server.analysisFor(uMain)
	if !slices.Contains(aMain2.AccountNames, "assets:bank") {
		t.Errorf("dependent analysis not rebuilt with new buffer content: %v", aMain2.AccountNames)
	}
}

func TestServer_Diagnostics(t *testing.T) {
	dir := t.TempDir()
	a := uri.File(filepath.Join(dir, "a.journal"))
	b := uri.File(filepath.Join(dir, "b.journal"))

	aContent := "account expenses:food\naccount assets:cash\ncommodity $\npayee test\n\n2024-01-01 * test\n    expenses:food  $10.00\n    assets:cash  $5.00\n"
	bContent := "include a.journal\n"
	testutil.WriteFile(t, a.Path(), []byte(aContent))
	testutil.WriteFile(t, b.Path(), []byte(bContent))

	srv := NewServer("test")
	capture := &captureClient{}
	srv.server.client = capture

	open := func(u uri.URI, content string) {
		t.Helper()
		if err := srv.server.DidOpen(t.Context(), &protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{URI: u, LanguageID: "journal", Version: 1, Text: content},
		}); err != nil {
			t.Fatalf("didOpen %s: %v", u, err)
		}
	}
	open(a, aContent)
	open(b, bContent)

	waitFor(t, "diagnostics for the unbalanced transaction", func() bool {
		da, _ := capture.lastDiags(a)
		return len(da) != 0
	})

	aEdited := "account expenses:food\naccount assets:cash\naccount assets:bank\ncommodity $\npayee test\n\n2024-01-01 * test\n    expenses:food  $20.00\n    assets:cash  $-10.00\n    assets:bank  $-10.00\n"
	if err := srv.server.DidChange(t.Context(), &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: a},
			Version:                2,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			&protocol.TextDocumentContentChangeWholeDocument{Text: aEdited},
		},
	}); err != nil {
		t.Fatalf("didChange a: %v", err)
	}

	waitFor(t, "a diagnostics to clear after the edit", func() bool {
		da, _ := capture.lastDiags(a)
		return len(da) == 0
	})

	if err := srv.server.DidClose(t.Context(), &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: a},
	}); err != nil {
		t.Fatalf("didClose a: %v", err)
	}
	if da, _ := capture.lastDiags(a); len(da) != 0 {
		t.Errorf("diagnostics not cleared on close: %v", da)
	}
}

func TestServer_DidChangeWatchedFiles_SkipsOpenDocuments(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.journal")
	testutil.WriteFile(t, base, []byte("2024-01-01 t\n    expenses:food  $10\n    assets:cash\n"))

	srv := NewServer("test")
	uBase := uri.File(base)
	srv.server.openDoc(uBase, "2024-01-01 t\n    expenses:food  $10\n    assets:cash\n", 1, "journal")

	a1 := srv.server.analysisFor(uBase)

	testutil.WriteFile(t, base, []byte("2024-01-01 t\n    expenses:food  $10\n    assets:bank\n"))
	if err := srv.server.DidChangeWatchedFiles(context.Background(), &protocol.DidChangeWatchedFilesParams{
		Changes: []protocol.FileEvent{{URI: uBase, Type: protocol.FileChangeTypeChanged}},
	}); err != nil {
		t.Fatalf("didChangeWatchedFiles: %v", err)
	}

	if a2 := srv.server.analysisFor(uBase); a2 != a1 {
		t.Error("open document rebuilt from disk; buffer is authoritative")
	}
}

func TestServer_DidChangeWatchedFiles_DiskChangeDirtiesDependents(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.journal")
	main := filepath.Join(dir, "main.journal")
	testutil.WriteFile(t, base, []byte("2024-01-01 t\n    expenses:food  $10\n    assets:cash\n"))
	testutil.WriteFile(t, main, []byte("include base.journal\n"))

	srv := NewServer("test")
	srv.server.client = &captureClient{}
	uMain := uri.File(main)
	if err := srv.server.DidOpen(context.Background(), &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{URI: uMain, LanguageID: "journal", Version: 1, Text: "include base.journal\n"},
	}); err != nil {
		t.Fatalf("didOpen: %v", err)
	}

	a1 := srv.server.analysisFor(uMain)
	if !slices.Contains(a1.AccountNames, "assets:cash") {
		t.Fatalf("initial analysis missing included account: %v", a1.AccountNames)
	}

	// base changes on disk, outside the editor
	testutil.WriteFile(t, base, []byte("2024-01-01 t\n    expenses:food  $10\n    assets:bank\n"))
	if err := srv.server.DidChangeWatchedFiles(context.Background(), &protocol.DidChangeWatchedFilesParams{
		Changes: []protocol.FileEvent{{URI: uri.File(base), Type: protocol.FileChangeTypeChanged}},
	}); err != nil {
		t.Fatalf("didChangeWatchedFiles: %v", err)
	}

	a2 := srv.server.analysisFor(uMain)
	if a2 == a1 {
		t.Error("disk change did not rebuild the dependent analysis")
	}
	if !slices.Contains(a2.AccountNames, "assets:bank") {
		t.Errorf("stale included content after disk change: %v", a2.AccountNames)
	}
}

type captureClient struct {
	protocol.Client
	mu   sync.Mutex
	diag []protocol.PublishDiagnosticsParams
}

func (c *captureClient) PublishDiagnostics(_ context.Context, params *protocol.PublishDiagnosticsParams) error {
	c.mu.Lock()
	c.diag = append(c.diag, *params)
	c.mu.Unlock()
	return nil
}

func (c *captureClient) lastDiags(u uri.URI) ([]protocol.Diagnostic, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, v := range slices.Backward(c.diag) {
		if v.URI == u {
			return v.Diagnostics, true
		}
	}
	return nil, false
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
