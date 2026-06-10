package golden

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("golden.update", false, "update golden files")

// Load reads testdata/<path>.input. Fails the test if file is not found.
func Load(t testing.TB, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", path+".input"))
	if err != nil {
		t.Fatalf("loading %s: %v", path, err)
	}
	return data
}

// Assert compares got against testdata/<name>.golden.
func Assert(t testing.TB, name, got string) {
	t.Helper()

	normalized := strings.NewReplacer("/", "__", " ", "_").Replace(name)
	path := filepath.Join("testdata", normalized+".golden")

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("creating testdata dir: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		return
	}

	golden, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Fatalf("no golden file %s, run with -golden.update:\n%s", path, got)
	}
	if err != nil {
		t.Fatalf("reading golden file: %v", err)
	}

	if string(golden) != got {
		t.Fatalf("golden mismatch for %s\nwant:\n%s\ngot:\n%s", path, string(golden), got)
	}
}
