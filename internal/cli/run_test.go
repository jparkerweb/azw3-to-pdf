package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withInputFlags clears the flags collectInputs reads and puts them back
// afterwards, since they are package-level and shared by every test.
func withInputFlags(t *testing.T) {
	t.Helper()
	input, output, stdin, recursive, includeText := flagInput, flagOutput, flagStdin, flagRecursive, flagIncludeText
	t.Cleanup(func() {
		flagInput, flagOutput, flagStdin, flagRecursive, flagIncludeText = input, output, stdin, recursive, includeText
	})
	flagInput, flagOutput, flagStdin, flagRecursive, flagIncludeText = "", "", false, false, false
}

func TestCollectInputsTextFiles(t *testing.T) {
	dir := t.TempDir()
	notes := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(notes, []byte("Some notes.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("folder of text needs --include-text", func(t *testing.T) {
		withInputFlags(t)
		_, err := collectInputs([]string{dir})
		if err == nil {
			t.Fatal("expected an error for a folder holding only notes.txt")
		}
		if !strings.Contains(err.Error(), "--include-text") {
			t.Errorf("the error should suggest --include-text: %v", err)
		}
	})

	t.Run("folder of text with --include-text", func(t *testing.T) {
		withInputFlags(t)
		flagIncludeText = true
		got, err := collectInputs([]string{dir})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || filepath.Base(got[0]) != "notes.txt" {
			t.Errorf("got %v, want notes.txt", got)
		}
	})

	t.Run("text file named directly", func(t *testing.T) {
		withInputFlags(t)
		got, err := collectInputs([]string{notes})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0] != notes {
			t.Errorf("got %v, want %s", got, notes)
		}
	})
}
