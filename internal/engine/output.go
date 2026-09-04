package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ConflictMode decides what happens when the output file already exists.
type ConflictMode string

const (
	// ConflictFail refuses to touch an existing file.
	ConflictFail ConflictMode = "fail"
	// ConflictOverwrite replaces it.
	ConflictOverwrite ConflictMode = "overwrite"
	// ConflictRename picks the next free "name (2).pdf".
	ConflictRename ConflictMode = "rename"
	// ConflictSkip reports the file as already done.
	ConflictSkip ConflictMode = "skip"
)

// OutputOptions describe where a converted book is written.
type OutputOptions struct {
	// Path is an explicit destination file. When set it wins over everything
	// else, and is only valid for a single conversion.
	Path string
	// Dir places the PDF in another directory, keeping the book's base name.
	Dir string
	// Suffix is appended to the base name (before ".pdf").
	Suffix string
	// Conflict decides what to do about an existing file.
	Conflict ConflictMode
}

// ErrSkipped reports that an output already exists and skipping was requested.
var ErrSkipped = fmt.Errorf("output already exists")

// ResolveOutput computes the destination path for one conversion and makes
// sure its directory exists.
func ResolveOutput(input string, opts OutputOptions) (string, error) {
	path := opts.Path
	if path == "" {
		dir := opts.Dir
		if dir == "" {
			dir = filepath.Dir(input)
		}
		base := strings.TrimSuffix(filepath.Base(input), filepath.Ext(input))
		path = filepath.Join(dir, sanitizeBase(base)+opts.Suffix+".pdf")
	}
	if filepath.Ext(path) == "" {
		path += ".pdf"
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating %s: %w", dir, err)
	}

	if _, err := os.Stat(path); err == nil {
		switch opts.Conflict {
		case ConflictOverwrite:
		case ConflictSkip:
			return path, ErrSkipped
		case ConflictRename:
			return nextFreeName(path), nil
		default:
			return "", fmt.Errorf("%s already exists (use --overwrite or --auto-rename)", path)
		}
	}
	return path, nil
}

// nextFreeName appends " (2)", " (3)" and so on until the name is unused.
func nextFreeName(path string) string {
	ext := filepath.Ext(path)
	stem := strings.TrimSuffix(path, ext)
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s (%d)%s", stem, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
	return path
}

// sanitizeBase strips characters that are illegal in file names on Windows.
// Kindle file names routinely contain colons from the book's subtitle.
func sanitizeBase(name string) string {
	replacer := strings.NewReplacer(
		":", " -", "*", "", "?", "", "\"", "'", "<", "(", ">", ")", "|", "-",
		"\\", "-", "/", "-",
	)
	out := strings.TrimSpace(replacer.Replace(name))
	out = strings.Trim(out, ". ")
	if out == "" {
		out = "book"
	}
	if len(out) > 150 {
		out = strings.TrimSpace(out[:150])
	}
	return out
}

// BookExtensions are the input files this tool understands.
var BookExtensions = []string{".azw3", ".azw", ".mobi", ".prc"}

// IsBookFile reports whether a path looks like a Kindle book.
func IsBookFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	for _, e := range BookExtensions {
		if ext == e {
			return true
		}
	}
	return false
}

// DiscoverBooks lists the Kindle books in a directory, optionally recursing.
func DiscoverBooks(dir string, recursive bool) []string {
	var found []string
	if recursive {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if IsBookFile(path) {
				found = append(found, path)
			}
			return nil
		})
		return found
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		if IsBookFile(path) {
			found = append(found, path)
		}
	}
	return found
}
