package journal

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"olexsmir.xyz/clerk/internal/testutil/txtar"
	"olexsmir.xyz/clerk/journal/ast"
)

func TestLoader_Resolve_basic(t *testing.T) {
	rj := resolveTxtar(t, "root.journal", `
-- root.journal --
2024/01/01 t
  a  $1
`)
	if len(rj.Occurrences) != 1 || len(rj.Items) != 1 || rj.Items[0].IsInclude {
		t.Fatal("basic: expected 1 file, 1 non-include item")
	}
}

func TestLoader_Resolve_withInclude(t *testing.T) {
	rj := resolveTxtar(t, "parent.journal", `
-- parent.journal --
include child.journal
2024/01/01 t
  a  $1

-- child.journal --
account expenses:food
2024/06/15 lunch
  expenses:food  $5
  assets:cash
`)
	// 2 files: parent (include + tx), child (directive + tx) = 4 items
	if len(rj.Items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(rj.Items))
	}
	if !rj.Items[0].IsInclude {
		t.Fatal("items[0] should be include marker")
	}
	if len(rj.Occurrences) != 2 {
		t.Fatal("expected 2 occurrences")
	}
}

func TestLoader_Resolve_yearPropagation(t *testing.T) {
	rj := resolveTxtar(t, "parent.journal", `
-- parent.journal --
year 2025
include child.journal
12-01 dinner
  expenses:food  $10
  assets:cash

-- child.journal --
06-15 lunch
  expenses:food  $5
  assets:cash
`)
	dates := make([]string, 0, 2)
	for _, item := range rj.Items {
		if item.IsInclude {
			continue
		}
		tx, ok := item.Occurrence.Ast.Entries[item.EntryIndex].(*ast.Transaction)
		if !ok {
			continue
		}
		dates = append(dates, fmt.Sprintf("%d-%02d-%02d", tx.Date.Year, tx.Date.Month, tx.Date.Day))
	}
	if len(dates) != 2 || dates[0] != "2025-06-15" || dates[1] != "2025-12-01" {
		t.Fatalf("expected [2025-06-15 2025-12-01], got %v", dates)
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

func TestLoader_Resolve_repeatedInclude(t *testing.T) {
	rj := resolveTxtar(t, "parent.journal", `
-- parent.journal --
include shared.journal
include shared.journal

-- shared.journal --
account expenses:shared
`)
	if len(rj.Occurrences) != 3 {
		t.Fatalf("expected 3 occurrences, got %d", len(rj.Occurrences))
	}
	if len(rj.ByPath["shared.journal"]) != 2 {
		t.Fatalf("expected 2 ByPath entries for shared.journal, got %d",
			len(rj.ByPath["shared.journal"]))
	}
}

func TestLoader_ResolveBytes_parseErrors(t *testing.T) {
	rj := NewLoader().ResolveBytes("bad.journal", []byte("@@@ garbage\n"))
	if len(rj.Occurrences[0].Errors) == 0 {
		t.Fatal("expected parse errors for garbage input")
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

func TestLoader_ClearCache(t *testing.T) {
	l := NewLoader()
	if len(l.contentCache) != 0 {
		t.Fatal("expected empty cache")
	}
	l.contentCache["x"] = []byte("y")
	l.ClearCache()
	if len(l.contentCache) != 0 {
		t.Fatal("expected empty cache after clear")
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
