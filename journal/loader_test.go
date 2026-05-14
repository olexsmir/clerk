package journal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoader_LoadBytes(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		l := NewLoader()
		pf, err := l.LoadBytes("empty.journal", []byte{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pf.Ast == nil {
			t.Fatal("expected non-nil AST")
		}
	})

	t.Run("transaction", func(t *testing.T) {
		l := NewLoader()
		src := []byte("2024/01/01 groceries\n    expenses:food  $10\n    assets:checking\n")
		pf, err := l.LoadBytes("t.journal", src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(pf.Errors) > 0 {
			t.Fatalf("unexpected parse errors: %v", pf.Errors)
		}
		if len(pf.FileErrors) > 0 {
			t.Fatalf("unexpected file errors: %v", pf.FileErrors)
		}
	})

	t.Run("parse errors", func(t *testing.T) {
		l := NewLoader()
		src := []byte("@@@ garbage\n")
		pf, err := l.LoadBytes("bad.journal", src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(pf.Errors) == 0 {
			t.Fatal("expected parse errors")
		}
	})

	t.Run("include not found", func(t *testing.T) {
		l := NewLoader()
		src := []byte("include nonexistent.journal\n")
		pf, err := l.LoadBytes("parent.journal", src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(pf.FileErrors) == 0 {
			t.Fatal("expected file errors for missing include")
		}
		if len(pf.Errors) > 0 {
			t.Fatalf("expected no parse errors, got %d", len(pf.Errors))
		}
		for _, fe := range pf.FileErrors {
			if fe.Path == "" {
				t.Error("expected non-empty path in FileError")
			}
			if fe.Message == "" {
				t.Error("expected non-empty message in FileError")
			}
		}
	})

	t.Run("self include dedup", func(t *testing.T) {
		l := NewLoader()
		src := []byte("include self.journal\n2024/01/01 t\n  a  $1\n")
		pf, err := l.LoadBytes("self.journal", src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// self-include is deduped (file already stored before includes resolved)
		if len(pf.Includes) != 0 {
			t.Fatalf("expected 0 includes (deduped), got %d", len(pf.Includes))
		}
	})

	t.Run("dedup", func(t *testing.T) {
		l := NewLoader()
		src := []byte("2024/01/01 t\n  a  $1\n")
		pf1, err := l.LoadBytes("same.journal", src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		pf2, err := l.LoadBytes("same.journal", src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pf1 != pf2 {
			t.Fatal("expected same pointer for deduplicated load")
		}
		// different path, same content — should NOT dedup
		pf3, err := l.LoadBytes("other.journal", src)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pf1 == pf3 {
			t.Fatal("expected different pointers for different paths")
		}
	})
}

func TestLoader_Load(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.journal")
	if err := os.WriteFile(mainPath, []byte("2024/01/01 t\n  a  $1\n"), 0o644); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}

	l := NewLoader()
	pf, err := l.Load(mainPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pf.Path != mainPath {
		t.Fatalf("expected path %q, got %q", mainPath, pf.Path)
	}
	if len(pf.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", pf.Errors)
	}

	t.Run("reuse on repeated load", func(t *testing.T) {
		pf2, err := l.Load(mainPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pf != pf2 {
			t.Fatal("expected same pointer on repeated load")
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := l.Load(filepath.Join(dir, "nonexistent.journal"))
		if err == nil {
			t.Fatal("expected error for nonexistent file")
		}
	})
}

func TestLoader_Load_withInclude(t *testing.T) {
	dir := t.TempDir()

	child := []byte("account expenses:food\n")
	if err := os.WriteFile(filepath.Join(dir, "child.journal"), child, 0o644); err != nil {
		t.Fatalf("writing child: %v", err)
	}

	parent := []byte("include child.journal\n")
	if err := os.WriteFile(filepath.Join(dir, "parent.journal"), parent, 0o644); err != nil {
		t.Fatalf("writing parent: %v", err)
	}

	l := NewLoader()
	pf, err := l.Load(filepath.Join(dir, "parent.journal"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.FileErrors) > 0 {
		t.Fatalf("unexpected file errors: %v", pf.FileErrors)
	}
	if len(pf.Includes) != 1 {
		t.Fatalf("expected 1 include, got %d", len(pf.Includes))
	}
	included := pf.Includes[0]
	if included.Path != filepath.Join(dir, "child.journal") {
		t.Fatalf("expected child path, got %q", included.Path)
	}
}

func TestLoader_Load_withGlobInclude(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "data")
	os.MkdirAll(sub, 0o755)

	for _, name := range []string{"a.journal", "b.journal", "c.journal"} {
		if err := os.WriteFile(filepath.Join(sub, name), []byte("account expenses:"+name[:1]+"\n"), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}

	parent := []byte("include data/*.journal\n")
	if err := os.WriteFile(filepath.Join(dir, "parent.journal"), parent, 0o644); err != nil {
		t.Fatalf("writing parent: %v", err)
	}

	l := NewLoader()
	pf, err := l.Load(filepath.Join(dir, "parent.journal"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.FileErrors) > 0 {
		t.Fatalf("unexpected file errors: %v", pf.FileErrors)
	}
	if len(pf.Includes) != 3 {
		t.Fatalf("expected 3 includes, got %d", len(pf.Includes))
	}
}

func TestLoader_Load_includeNotFound(t *testing.T) {
	dir := t.TempDir()

	parent := []byte("include data/*.journal\n")
	if err := os.WriteFile(filepath.Join(dir, "parent.journal"), parent, 0o644); err != nil {
		t.Fatalf("writing parent: %v", err)
	}

	l := NewLoader()
	pf, err := l.Load(filepath.Join(dir, "parent.journal"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.FileErrors) != 1 {
		t.Fatalf("expected 1 file error, got %d", len(pf.FileErrors))
	}
	if len(pf.Errors) != 0 {
		t.Fatalf("expected 0 parse errors, got %d", len(pf.Errors))
	}
}

func TestLoader_Load_cycleDedup(t *testing.T) {
	dir := t.TempDir()

	aPath := filepath.Join(dir, "a.journal")
	bPath := filepath.Join(dir, "b.journal")

	if err := os.WriteFile(aPath, []byte("include b.journal\n"), 0o644); err != nil {
		t.Fatalf("writing a: %v", err)
	}
	if err := os.WriteFile(bPath, []byte("include a.journal\n"), 0o644); err != nil {
		t.Fatalf("writing b: %v", err)
	}

	l := NewLoader()
	pf, err := l.Load(aPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// circular A→B→A: A is deduped (already stored before includes resolved)
	if len(pf.Includes) != 1 {
		t.Fatalf("expected 1 include, got %d", len(pf.Includes))
	}
	if pf.Includes[0].Path != bPath {
		t.Fatalf("expected include path %q, got %q", bPath, pf.Includes[0].Path)
	}
	// B's include of A should resolve to the same pf (dedup)
	if pf.Includes[0].Includes[0] != pf {
		t.Fatal("expected circular dedup: B's include of A should point to original A")
	}
}

func TestLoader_Ordered(t *testing.T) {
	dir := t.TempDir()

	leaf := []byte("2024/01/01 t\n  a  $1\n")
	if err := os.WriteFile(filepath.Join(dir, "leaf.journal"), leaf, 0o644); err != nil {
		t.Fatalf("writing leaf: %v", err)
	}

	middle := []byte("include leaf.journal\n")
	if err := os.WriteFile(filepath.Join(dir, "middle.journal"), middle, 0o644); err != nil {
		t.Fatalf("writing middle: %v", err)
	}

	root := []byte("include middle.journal\n")
	if err := os.WriteFile(filepath.Join(dir, "root.journal"), root, 0o644); err != nil {
		t.Fatalf("writing root: %v", err)
	}

	l := NewLoader()
	pf, err := l.Load(filepath.Join(dir, "root.journal"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.FileErrors) > 0 {
		t.Fatalf("unexpected file errors: %v", pf.FileErrors)
	}

	ordered := l.Ordered()
	names := make([]string, len(ordered))
	for i, f := range ordered {
		names[i] = filepath.Base(f.Path)
	}

	// leaf before middle before root — includes before includer
	if len(names) != 3 {
		t.Fatalf("expected 3 ordered files, got %d: %v", len(names), names)
	}
	if names[0] != "leaf.journal" {
		t.Fatalf("expected leaf first, got %q", names[0])
	}
	if names[1] != "middle.journal" {
		t.Fatalf("expected middle second, got %q", names[1])
	}
	if names[2] != "root.journal" {
		t.Fatalf("expected root last, got %q", names[2])
	}
}

func TestLoader_Load_fileErrorsSeparateFromParseErrors(t *testing.T) {
	l := NewLoader()
	src := []byte("@@@ garbage\ninclude nonexistent.journal\n")
	pf, err := l.LoadBytes("mixed.journal", src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pf.Errors) == 0 {
		t.Fatal("expected parse errors")
	}
	if len(pf.FileErrors) == 0 {
		t.Fatal("expected file errors")
	}
}
