//go:build windows

package xdg

import (
	"os"
	"path/filepath"
)

func defaultStateDir(home string) string {
	if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
		return dir
	}
	return filepath.Join(home, "AppData", "Local")
}
