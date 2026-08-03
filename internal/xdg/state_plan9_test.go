//go:build plan9

package xdg

import (
	"path/filepath"
	"testing"
)

func TestStateDirDefault(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("home", "/home/test")

	want := filepath.Join("/home/test", "lib", "state")
	got, err := StateDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("StateDir() = %q, want %q", got, want)
	}
}
