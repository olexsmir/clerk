package xdg

import (
	"os"
	"path/filepath"
)

// StateDir returns the base directory for user-specific state data,
// per the XDG Base Directory Specification: $XDG_STATE_HOME if it is set to
// an absolute path, otherwise the platform-specific default.
// Vendored from github.com/adrg/xdg (MIT), StateHome.
func StateDir() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" && filepath.IsAbs(dir) {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return defaultStateDir(home), nil
}
