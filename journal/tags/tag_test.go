package tags

import (
	"bytes"
	"path/filepath"
	"testing"

	"olexsmir.xyz/clerk/internal/testutil"
	"olexsmir.xyz/clerk/internal/testutil/golden"
	"olexsmir.xyz/clerk/journal"
)

func TestTagsGeneration(t *testing.T) {
	tests := []string{"basic", "kinds", "duplicates"}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			inp := golden.Load(t, tt)
			out := generateTags(t, inp, tt+".journal")
			golden.AssertInput(t, out, tt)
		})
	}
}

func TestCrossFileResolution(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	declSrc := golden.Load(t, "crossfile1")
	jrnlSrc := golden.Load(t, "crossfile2")
	testutil.WriteFile(t, filepath.Join(tmp, "decls.journal"), declSrc)
	testutil.WriteFile(t, filepath.Join(tmp, "2026.journal"), jrnlSrc)

	loader := journal.NewLoader()
	if _, err := loader.Load(filepath.Join(tmp, "2026.journal")); err != nil {
		t.Fatal(err)
	}

	tagger := New(loader, tmp)
	var buf bytes.Buffer
	if err := tagger.Write(&buf); err != nil {
		t.Fatal(err)
	}

	golden.AssertInput(t, buf.String(), "crossfile")
}

func generateTags(t *testing.T, src []byte, fname string) string {
	t.Helper()

	l := journal.NewLoader()
	_, err := l.LoadBytes(fname, src)
	if err != nil {
		t.Fatalf("loading journal: %v", err)
	}
	absPath, _ := filepath.Abs(fname)
	tagger := New(l, filepath.Dir(absPath))
	var buf bytes.Buffer
	if err := tagger.Write(&buf); err != nil {
		t.Fatalf("writing tags: %v", err)
	}
	return buf.String()
}
