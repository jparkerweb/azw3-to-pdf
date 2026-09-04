package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// OpenFolder reveals a directory in the system file manager.
func OpenFolder(dir string) error {
	if dir == "" {
		return fmt.Errorf("no folder to open")
	}
	if info, err := os.Stat(dir); err != nil {
		return err
	} else if !info.IsDir() {
		dir = filepath.Dir(dir)
	}

	name, args := openCommand(dir)
	if name == "" {
		return fmt.Errorf("opening folders is not supported on %s", runtime.GOOS)
	}
	cmd := exec.Command(name, args...)
	return cmd.Start()
}

// OpenFile opens a file with whatever the system considers its default
// application, which for a PDF is a reader.
func OpenFile(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	name, args := openCommand(path)
	if name == "" {
		return fmt.Errorf("opening files is not supported on %s", runtime.GOOS)
	}
	return exec.Command(name, args...).Start()
}
