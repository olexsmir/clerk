//go:build plan9

package xdg

import "path/filepath"

func defaultStateDir(home string) string {
	return filepath.Join(home, "lib", "state")
}
