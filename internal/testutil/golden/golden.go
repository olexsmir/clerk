package golden

import (
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"olexsmir.xyz/clerk/internal/testutil/txtar"
)

var update = flag.Bool("golden.update", false, "update golden files")

var normalizer = strings.NewReplacer("/", "__", " ", "_")

func txtarPath(name string) string {
	return filepath.Join("testdata", normalizer.Replace(name)+".txtar")
}

// Archive wraps a txtar.Archive with its on-disk path for efficient updates.
type Archive struct {
	*txtar.Archive
	path string
}

// Read parses testdata/<name>.txtar and returns the archive.
// Fails the test if the file doesn't exist or the archive is empty.
func Read(t testing.TB, name string) *Archive {
	t.Helper()
	path := txtarPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("loading %s: %v", name+".txtar", err)
	}
	a := txtar.Parse(data)
	if len(a.Files) == 0 {
		t.Fatalf("txtar %s: no files", name)
	}
	return &Archive{Archive: a, path: path}
}

func (a *Archive) FS() (fs.FS, error) { return txtar.FS(a.Archive) }

// Assert compares got against the "expect" file in a txtar archive.
// With -golden.update, rewrites the expect section in-place, preserving other files.
func Assert(t testing.TB, a *Archive, got string) {
	t.Helper()

	if *update {
		for i, f := range a.Files {
			if f.Name == "expect" {
				a.Files[i].Data = []byte(got)
				goto write
			}
		}
		a.Files = append(a.Files, txtar.File{Name: "expect", Data: []byte(got)})
	write:
		if err := os.WriteFile(a.path, txtar.Format(a.Archive), 0o644); err != nil {
			t.Fatalf("writing updated txtar: %v", err)
		}
		return
	}

	for _, f := range a.Files {
		if f.Name == "expect" {
			if string(f.Data) != got {
				t.Fatalf("golden mismatch for %s\nwant:\n%s\ngot:\n%s", a.path, string(f.Data), got)
			}
			return
		}
	}
	t.Fatalf("txtar %s: no 'expect' file", a.path)
}
