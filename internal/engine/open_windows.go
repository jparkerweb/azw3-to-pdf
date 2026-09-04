//go:build windows

package engine

// openCommand returns the shell command that opens a path on Windows.
func openCommand(path string) (string, []string) {
	// "start" is a cmd builtin; the empty string is the window title, which
	// stops cmd from treating a quoted path as one.
	return "cmd", []string{"/c", "start", "", path}
}
