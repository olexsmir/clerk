package golden

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("golden.update", false, "update golden files")

// Load reads testdata/<name>.input. Fails the test if the file is missing.
func Load(t testing.TB, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name+".input")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading input file %s: %v", path, err)
	}
	return data
}

func AssertInput(t testing.TB, got string, name string) {
	t.Helper()

	path := filepath.Join("testdata", name+".golden")
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

func Assert(t testing.TB, got string) {
	t.Helper()

	name := strings.NewReplacer("/", "__", " ", "_").Replace(t.Name())
	name = strings.TrimPrefix(name, "Test")
	path := filepath.Join("testdata", "golden", name+".golden")

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("creating golden dir: %v", err)
		}
		t.Logf("updated golden file: %s", path)
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
