//go:build !windows

package engine

import "runtime"

// openCommand returns the command that opens a path on this platform.
func openCommand(path string) (string, []string) {
	if runtime.GOOS == "darwin" {
		return "open", []string{path}
	}
	return "xdg-open", []string{path}
}
