//go:build windows

package xdg

import (
	"path/filepath"
	"testing"
)

func TestStateDirDefault(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("LOCALAPPDATA", "")
	t.Setenv("USERPROFILE", `C:\Users\test`)

	want := filepath.Join(`C:\Users\test`, "AppData", "Local")
	got, err := StateDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("StateDir() = %q, want %q", got, want)
	}
}
