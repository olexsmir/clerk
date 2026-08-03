//go:build aix || dragonfly || freebsd || (js && wasm) || linux || netbsd || openbsd || solaris

package xdg

import "path/filepath"

func defaultStateDir(home string) string {
	return filepath.Join(home, ".local", "state")
}
