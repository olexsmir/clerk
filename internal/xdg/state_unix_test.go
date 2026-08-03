//go:build aix || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris

package xdg

import (
	"path/filepath"
	"testing"
)

func TestStateDirDefault(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/home/test")

	want := filepath.Join("/home/test", ".local", "state")
	got, err := StateDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("StateDir() = %q, want %q", got, want)
	}
}
