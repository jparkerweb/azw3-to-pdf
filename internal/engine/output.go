package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jparkerweb/ebook-to-pdf/internal/ebook"
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
// Ebook file names routinely contain colons from the book's subtitle.
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

// BookExtensions are the input files this tool understands, matched as
// lower-cased suffixes so that ".fb2.zip" works while a plain ".zip" does not.
var BookExtensions = []string{".azw3", ".azw", ".mobi", ".prc", ".epub", ".cbz", ".fb2", ".fb2.zip", ".txt", ".html", ".htm", ".xhtml"}

// TextExtensions are the book files a folder scan skips unless asked: a
// folder of documents is full of them, and few are books.
var TextExtensions = []string{".txt", ".html", ".htm", ".xhtml"}

// IsBookFile reports whether a path has one of the ebook extensions.
func IsBookFile(path string) bool {
	return hasSuffixIn(path, BookExtensions)
}

// IsTextFile reports whether a path is a plain text or HTML file.
func IsTextFile(path string) bool {
	return hasSuffixIn(path, TextExtensions)
}

// IsConvertible reports whether a file named directly can be converted: by
// its extension, or failing that by its content, so a book with the wrong
// name still works.
func IsConvertible(path string) bool {
	if IsBookFile(path) {
		return true
	}
	f, err := ebook.DetectFile(path)
	return err == nil && f != ebook.FormatUnknown
}

func hasSuffixIn(path string, exts []string) bool {
	lower := strings.ToLower(path)
	for _, e := range exts {
		if strings.HasSuffix(lower, e) {
			return true
		}
	}
	return false
}

// DiscoverBooks lists the ebooks in a directory, optionally recursing. Text
// and HTML files are left out unless includeText is set.
func DiscoverBooks(dir string, recursive, includeText bool) []string {
	var found []string
	wanted := func(path string) bool {
		return IsBookFile(path) && (includeText || !IsTextFile(path))
	}
	if recursive {
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if wanted(path) {
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
		if wanted(path) {
			found = append(found, path)
		}
	}
	return found
}
