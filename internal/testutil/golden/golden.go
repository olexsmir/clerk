package golden

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

func Assert(t *testing.T, got string) {
	t.Helper()

	name := strings.NewReplacer("/", "__", " ", "_").Replace(t.Name())
	name = strings.TrimLeft(name, "Test")
	path := filepath.Join("testdata", "golden", name+".golden")

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("creating golden dir: %v", err)
		}
		t.Logf("Saving golden file in %s", got)
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		return
	}

	golden, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Fatalf("no golden file, run with -update:\n%s", got)
	}
	if err != nil {
		t.Fatalf("reading golden file: %v", err)
	}

	if string(golden) != got {
		t.Fatalf("golden mismatch\nwant:\n%s\ngot:\n%s", string(golden), got)
	}
}
