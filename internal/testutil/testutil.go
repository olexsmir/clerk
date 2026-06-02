package testutil

import (
	"os"
	"testing"
)

func WriteFile(t testing.TB, fpath string, src []byte) {
	t.Helper()
	if err := os.WriteFile(fpath, src, 0o644); err != nil {
		t.Fatalf("failed to write '%s': %v", fpath, err)
	}
}
