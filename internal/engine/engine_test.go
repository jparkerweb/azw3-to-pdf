package engine

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveOutputDefaults(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "A Book.azw3")

	got, err := ResolveOutput(input, OutputOptions{})
	if err != nil {
		t.Fatalf("ResolveOutput returned %v", err)
	}
	if want := filepath.Join(dir, "A Book.pdf"); got != want {
		t.Errorf("output is %q, want %q", got, want)
	}
}

func TestResolveOutputSuffixAndDir(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "pdfs")
	input := filepath.Join(dir, "book.azw3")

	got, err := ResolveOutput(input, OutputOptions{Dir: out, Suffix: "-print"})
	if err != nil {
		t.Fatalf("ResolveOutput returned %v", err)
	}
	if want := filepath.Join(out, "book-print.pdf"); got != want {
		t.Errorf("output is %q, want %q", got, want)
	}
	if _, err := os.Stat(out); err != nil {
		t.Errorf("the output directory was not created: %v", err)
	}
}

func TestResolveOutputConflicts(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "book.azw3")
	existing := filepath.Join(dir, "book.pdf")
	if err := os.WriteFile(existing, []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := ResolveOutput(input, OutputOptions{Conflict: ConflictFail}); err == nil {
		t.Error("ConflictFail should refuse to overwrite")
	}

	if _, err := ResolveOutput(input, OutputOptions{Conflict: ConflictSkip}); !errors.Is(err, ErrSkipped) {
		t.Errorf("ConflictSkip returned %v, want ErrSkipped", err)
	}

	got, err := ResolveOutput(input, OutputOptions{Conflict: ConflictOverwrite})
	if err != nil || got != existing {
		t.Errorf("ConflictOverwrite returned (%q, %v), want the existing path", got, err)
	}

	got, err = ResolveOutput(input, OutputOptions{Conflict: ConflictRename})
	if err != nil {
		t.Fatalf("ConflictRename returned %v", err)
	}
	if want := filepath.Join(dir, "book (2).pdf"); got != want {
		t.Errorf("ConflictRename chose %q, want %q", got, want)
	}
}

func TestSanitizeBase(t *testing.T) {
	cases := map[string]string{
		"Book: A Subtitle": "Book - A Subtitle",
		`Odd/Name\Here`:    "Odd-Name-Here",
		"  spaced  ":       "spaced",
		"...":              "book",
	}
	for in, want := range cases {
		if got := sanitizeBase(in); got != want {
			t.Errorf("sanitizeBase(%q) = %q, want %q", in, got, want)
		}
	}
	if got := sanitizeBase(strings.Repeat("x", 400)); len(got) > 150 {
		t.Errorf("sanitizeBase produced a %d character name", len(got))
	}
}

func TestIsBookFile(t *testing.T) {
	for _, path := range []string{"a.azw3", "b.MOBI", "c.azw", "d.prc"} {
		if !IsBookFile(path) {
			t.Errorf("IsBookFile(%q) = false", path)
		}
	}
	for _, path := range []string{"a.epub", "b.pdf", "c", "d.txt"} {
		if IsBookFile(path) {
			t.Errorf("IsBookFile(%q) = true", path)
		}
	}
}

func TestDiscoverBooks(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"one.azw3", "two.mobi", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(nested, "three.azw"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if got := DiscoverBooks(dir, false); len(got) != 2 {
		t.Errorf("a flat search found %d books, want 2: %v", len(got), got)
	}
	if got := DiscoverBooks(dir, true); len(got) != 3 {
		t.Errorf("a recursive search found %d books, want 3: %v", len(got), got)
	}
}
