package ebook

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestZipSkipsEscapingEntries(t *testing.T) {
	data := buildZip(t,
		zipEntry{Name: "../evil.txt", Data: []byte("evil")},
		zipEntry{Name: "/abs.txt", Data: []byte("abs")},
		zipEntry{Name: `OEBPS\text\ch1.xhtml`, Data: []byte("ch1")},
	)
	a, err := openZip(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := a.names(); !slices.Equal(got, []string{"OEBPS/text/ch1.xhtml"}) {
		t.Errorf("names() = %q, want only the backslash entry, cleaned", got)
	}
	for _, name := range []string{"../evil.txt", "evil.txt", "/abs.txt", "abs.txt"} {
		if a.has(name) {
			t.Errorf("has(%q) is true", name)
		}
		if _, err := a.read(name); err == nil {
			t.Errorf("read(%q) succeeded", name)
		}
	}
	got, err := a.read("OEBPS/text/ch1.xhtml")
	if err != nil || string(got) != "ch1" {
		t.Errorf("read of the backslash entry = %q, %v", got, err)
	}
	if got, err := a.read("oebps/TEXT/ch1.xhtml"); err != nil || string(got) != "ch1" {
		t.Errorf("case-insensitive read = %q, %v", got, err)
	}
}

func TestZipResolve(t *testing.T) {
	a, err := openZip(buildZip(t, zipEntry{Name: "x", Data: []byte("x")}))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ base, href, want string }{
		{"OEBPS/text/ch1.xhtml", "../images/a%20b.png#x", "OEBPS/images/a b.png"},
		{"OEBPS/content.opf", "text/ch1.xhtml?v=2", "OEBPS/text/ch1.xhtml"},
		{"content.opf", "ch1.xhtml", "ch1.xhtml"},
	}
	for _, c := range cases {
		if got := a.resolve(c.base, c.href); got != c.want {
			t.Errorf("resolve(%q, %q) = %q, want %q", c.base, c.href, got, c.want)
		}
	}
}

func TestZipRefusesZipBomb(t *testing.T) {
	data := buildZip(t, zipEntry{Name: "bomb.xhtml", Data: bytes.Repeat([]byte{'a'}, 2<<20)})
	a, err := openZip(data)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.read("bomb.xhtml")
	if err == nil {
		t.Fatal("a 2 MiB entry that compresses to almost nothing was read")
	}
	if !strings.Contains(err.Error(), "bomb.xhtml") || !strings.Contains(err.Error(), "zip bomb") {
		t.Errorf("error %q should name the entry and call it a zip bomb", err)
	}

	// The same size stored without compression is fine.
	stored := buildZip(t, zipEntry{Name: "big.bin", Data: bytes.Repeat([]byte{'a'}, 2<<20), Store: true})
	if a, err = openZip(stored); err != nil {
		t.Fatal(err)
	}
	if got, err := a.read("big.bin"); err != nil || len(got) != 2<<20 {
		t.Errorf("stored 2 MiB entry: %d bytes, %v", len(got), err)
	}
}

func TestZipRejectsDamagedArchive(t *testing.T) {
	data := buildZip(t, zipEntry{Name: "a.txt", Data: []byte("hello")})
	_, err := openZip(data[:len(data)-10])
	if !errors.Is(err, errBrokenArchive) {
		t.Errorf("openZip of a truncated archive = %v, want errBrokenArchive", err)
	}
}

// lowerZipLimits shrinks the unpacked-size caps for one test, so the
// refusals can be tested without building archives of hundreds of MiB.
func lowerZipLimits(t *testing.T, entry, total int64) {
	t.Helper()
	oldEntry, oldTotal := maxEntryBytes, maxTotalBytes
	maxEntryBytes, maxTotalBytes = entry, total
	t.Cleanup(func() { maxEntryBytes, maxTotalBytes = oldEntry, oldTotal })
}

func TestZipRefusesOversizedEntry(t *testing.T) {
	lowerZipLimits(t, 2<<20, 1<<30)
	a, err := openZip(buildZip(t, zipEntry{Name: "huge.png", Data: bytes.Repeat([]byte{'a'}, 3<<20), Store: true}))
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.read("huge.png")
	if err == nil || !strings.Contains(err.Error(), "huge.png") || !strings.Contains(err.Error(), "larger than") {
		t.Errorf("reading an entry over the cap = %v, want a refusal naming huge.png", err)
	}
}

func TestZipRefusesOversizedBook(t *testing.T) {
	lowerZipLimits(t, 256<<20, 3<<20)
	page := bytes.Repeat([]byte{'a'}, 2<<20)
	a, err := openZip(buildZip(t,
		zipEntry{Name: "01.png", Data: page, Store: true},
		zipEntry{Name: "02.png", Data: page, Store: true}))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.read("01.png"); err != nil {
		t.Fatalf("the first entry is under the total: %v", err)
	}
	_, err = a.read("02.png")
	if err == nil || !strings.Contains(err.Error(), "02.png") || !strings.Contains(err.Error(), "larger than 3 MiB") {
		t.Errorf("reading past the total = %v, want a refusal naming 02.png", err)
	}
}
