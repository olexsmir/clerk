//go:build darwin

package xdg

import (
	"path/filepath"
	"testing"
)

func TestStateDirDefault(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/Users/test")

	want := filepath.Join("/Users/test", "Library", "Application Support")
	got, err := StateDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("StateDir() = %q, want %q", got, want)
	}
}
