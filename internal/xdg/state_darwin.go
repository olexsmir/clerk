//go:build darwin

package xdg

import "path/filepath"

func defaultStateDir(home string) string {
	return filepath.Join(home, "Library", "Application Support")
}
