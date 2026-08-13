package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"olexsmir.xyz/clerk/internal/testutil"
	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/internal/testutil/txtar"
	"olexsmir.xyz/clerk/journal/ast"
)

func TestLoader_Resolve(t *testing.T) {
	for _, tname := range []string{"basic", "with-include", "year-propagation", "year-context", "repeated-include", "parse-errors"} {
		t.Run(tname, func(t *testing.T) {
			a := golden.Read(t, tname)
			fsys, err := a.FS()
			if err != nil {
				t.Fatal(err)
			}
			rj, err := NewLoader().ResolveFS(fsys, "in.journal")
			if err != nil {
				t.Fatalf("resolving in.journal: %v", err)
			}
			golden.Assert(t, a, dumpResolved(rj))
		})
	}
}

func TestLoader_Resolve_cycleDetection(t *testing.T) {
	rj := resolveTxtar(t, "a.journal", `
-- a.journal --
include b.journal

-- b.journal --
include a.journal
`)
	if len(rj.FileErrors()) == 0 {
		t.Fatal("expected cycle error")
	}
}

func dumpResolved(rj *ResolvedJournal) string {
	var b strings.Builder
	for _, pf := range rj.Occurrences {
		fmt.Fprintf(&b, "== %s ==\n", pf.Path)
		for i, e := range pf.Ast.Entries {
			if s := entrySummary(e); s != "" {
				fmt.Fprintf(&b, "  %d %s\n", i+1, s)
			}
		}
		for _, fe := range pf.FileErrors {
			fmt.Fprintf(&b, "  error: %s\n", fe.Message)
		}
		for _, pe := range pf.Errors {
			fmt.Fprintf(&b, "  parse-error: %s\n", pe.Message)
		}
	}
	return b.String()
}

func entrySummary(e ast.Entry) string {
	switch e := e.(type) {
	case *ast.BlankLine:
		return ""
	case *ast.IncludeDirective:
		return "include " + e.Path
	case *ast.YearDirective:
		return fmt.Sprintf("year %d", e.Year)
	case *ast.AccountDirective:
		return "account " + e.Account.String()
	case *ast.Transaction:
		s := fmt.Sprintf("%04d-%02d-%02d", e.Date.Year, e.Date.Month, e.Date.Day)
		if e.Payee != nil {
			s += " " + e.Payee.Name
		}
		return s
	default:
		return fmt.Sprintf("<%T>", e)
	}
}

func TestLoader_ContentCache(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "a.journal")

	os.WriteFile(fpath, []byte("2024/01/15 t\n  a  $1\n"), 0o644)
	l := NewLoader()

	// first resolve populates cache.
	rj1 := mustResolve(t, l, fpath)
	date1 := txDate(rj1)

	// modify file, resolve again - should return cached date.
	os.WriteFile(fpath, []byte("2024/06/01 t\n  a  $1\n"), 0o644)
	rj2 := mustResolve(t, l, fpath)
	if txDate(rj2) != date1 {
		t.Fatal("expected cached date before invalidation")
	}

	// invalidate and resolve again - should see new date.
	l.InvalidateFile(fpath)
	rj3 := mustResolve(t, l, fpath)
	if txDate(rj3) != "2024-06-01" {
		t.Fatalf("expected 2024-06-01 after invalidation, got %s", txDate(rj3))
	}
}

func TestLoader_ContentCache_LineEndings(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "crlf.journal")
	os.WriteFile(fpath, []byte("2024/01/01 t\r\n  a  $1\r\n"), 0o644)

	rj, err := NewLoader().Resolve(fpath)
	if err != nil || len(rj.Occurrences[0].Errors) > 0 {
		t.Fatal("CRLF file should parse without errors")
	}
}

func TestResolveIncludePath(t *testing.T) {
	tests := []struct {
		parent, pattern string
		wantPath        string
		wantErr         bool
	}{
		{"main.journal", "child.journal", "child.journal", false},
		{"main.journal", "sub/child.journal", "sub/child.journal", false},
		{"main.journal", "../other.journal", "", true},
		{"sub/main.journal", "child.journal", "sub/child.journal", false},
		{"sub/main.journal", "../other.journal", "", true},
	}
	for _, tt := range tests {
		got, err := resolveIncludePath(tt.parent, tt.pattern)
		if tt.wantErr {
			if err == nil {
				t.Errorf("resolveIncludePath(%q, %q): want error, got %q", tt.parent, tt.pattern, got)
			}
			continue
		}
		if err != nil || got != tt.wantPath {
			t.Errorf("resolveIncludePath(%q, %q) = %q, %v; want %q", tt.parent, tt.pattern, got, err, tt.wantPath)
		}
	}
}

// helpers

func resolveTxtar(t *testing.T, rootFile, archive string) *ResolvedJournal {
	t.Helper()
	a := txtar.Parse([]byte(archive))
	fsys, err := txtar.FS(a)
	if err != nil {
		t.Fatal(err)
	}
	l := NewLoader()
	rj, err := l.ResolveFS(fsys, rootFile)
	if err != nil {
		t.Fatal(err)
	}
	return rj
}

func mustResolve(t *testing.T, l *Loader, fpath string) *ResolvedJournal {
	t.Helper()
	rj, err := l.Resolve(fpath)
	if err != nil {
		t.Fatal(err)
	}
	return rj
}

func txDate(rj *ResolvedJournal) string {
	tx := rj.Occurrences[0].Ast.Entries[0].(*ast.Transaction)
	return fmt.Sprintf("%d-%02d-%02d", tx.Date.Year, tx.Date.Month, tx.Date.Day)
}

// writeFiles writes the given files into dir.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		testutil.WriteFile(t, filepath.Join(dir, name), []byte(content))
	}
}

// newLoaderDir writes files into a temp dir and returns a loader over it.
func newLoaderDir(t *testing.T, files map[string]string) (*Loader, string) {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, files)
	return NewLoader(), dir
}

// mustOccurrence returns the occurrence of path in the resolve.
func mustOccurrence(t *testing.T, rj *ResolvedJournal, path string) *ParsedFile {
	t.Helper()
	got := rj.ByPath[path]
	if len(got) == 0 {
		t.Fatalf("file not resolved: %s", path)
	}
	return got[0]
}

// TestLoader_ParseCache_SharedPointer: two roots including the same file get
// the same *ast.Journal for it (one parse, shared).
func TestLoader_ParseCache_SharedPointer(t *testing.T) {
	l, dir := newLoaderDir(t, map[string]string{
		"a.journal": "include c.journal\n",
		"b.journal": "include c.journal\n",
		"c.journal": "account expenses:food\n",
	})
	c := filepath.Join(dir, "c.journal")

	ca := mustOccurrence(t, mustResolve(t, l, filepath.Join(dir, "a.journal")), c)
	cb := mustOccurrence(t, mustResolve(t, l, filepath.Join(dir, "b.journal")), c)
	if ca.Ast != cb.Ast {
		t.Error("c.journal parsed twice: Ast pointers differ")
	}
	if ca == cb {
		t.Error("ParsedFile wrapper must be per-resolve")
	}
}

// TestLoader_ParseCache_ContentChange: new content forces a new parse.
func TestLoader_ParseCache_ContentChange(t *testing.T) {
	l, dir := newLoaderDir(t, map[string]string{
		"a.journal": "include c.journal\n",
		"c.journal": "account expenses:food\n",
	})
	a, c := filepath.Join(dir, "a.journal"), filepath.Join(dir, "c.journal")

	first := mustOccurrence(t, mustResolve(t, l, a), c).Ast
	writeFiles(t, dir, map[string]string{"c.journal": "account expenses:travel\n"})
	l.InvalidateFile(c) // the content cache would serve stale bytes
	second := mustOccurrence(t, mustResolve(t, l, a), c).Ast
	if first == second {
		t.Error("edited file served the old parse")
	}
}

// TestLoader_ContentProvider: open-buffer text wins over disk for includes.
func TestLoader_ContentProvider(t *testing.T) {
	l, dir := newLoaderDir(t, map[string]string{
		"a.journal": "include c.journal\n",
		"c.journal": "account expenses:old\n",
	})
	l.ContentProvider = func(path string) ([]byte, bool) {
		if filepath.Base(path) == "c.journal" {
			return []byte("account expenses:new\n"), true
		}
		return nil, false
	}

	pf := mustOccurrence(t, mustResolve(t, l, filepath.Join(dir, "a.journal")), filepath.Join(dir, "c.journal"))
	ad := pf.Ast.Entries[0].(*ast.AccountDirective)
	if ad.Account.String() != "expenses:new" {
		t.Errorf("included content = %q, want expenses:new (buffer wins over disk)", ad.Account.String())
	}
}
