package ebook

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetect(t *testing.T) {
	mobi := buildMinimalBook(t, "<p>Hello</p>")
	cases := []struct {
		label string
		name  string
		data  []byte
		want  Format
	}{
		{"MOBI named .epub", "book.epub", mobi, FormatMOBI},
		{"EPUB by mimetype", "book.epub", buildZip(t,
			zipEntry{Name: "mimetype", Data: []byte("application/epub+zip"), Store: true},
			zipEntry{Name: "OEBPS/ch1.xhtml", Data: []byte("<p>x</p>")}), FormatEPUB},
		{"EPUB by container", "book.zip", buildZip(t,
			zipEntry{Name: "META-INF/container.xml", Data: []byte("<container/>")}), FormatEPUB},
		{"FB2 in a zip", "book.fb2.zip", buildZip(t,
			zipEntry{Name: "book.fb2", Data: []byte("<FictionBook/>")}), FormatFB2Zip},
		{"comic archive", "comic.cbz", buildZip(t,
			zipEntry{Name: "01.jpg", Data: []byte("jpg")},
			zipEntry{Name: "02.png", Data: []byte("png")},
			zipEntry{Name: "ComicInfo.xml", Data: []byte("<ComicInfo/>")}), FormatCBZ},
		{"zip of documents", "docs.zip", buildZip(t,
			zipEntry{Name: "notes.txt", Data: []byte("notes")},
			zipEntry{Name: "a.doc", Data: []byte("doc")}), FormatUnknown},
		{"FictionBook XML", "book.xml", []byte("\xef\xbb\xbf  <?xml version=\"1.0\"?>\n<FictionBook xmlns=\"x\">"), FormatFB2},
		{"text by name", "x.txt", []byte("Chapter 1"), FormatText},
		{"HTML by name", "x.HTM", []byte("<p>hi</p>"), FormatHTML},
		{"nothing", "x.bin", []byte("nothing to see"), FormatUnknown},
	}
	for _, c := range cases {
		if got := Detect(c.name, c.data); got != c.want {
			t.Errorf("%s: Detect = %v, want %v", c.label, got, c.want)
		}

		path := filepath.Join(t.TempDir(), c.name)
		if err := os.WriteFile(path, c.data, 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := DetectFile(path)
		if err != nil {
			t.Errorf("%s: DetectFile: %v", c.label, err)
		}
		if got != c.want {
			t.Errorf("%s: DetectFile = %v, want %v", c.label, got, c.want)
		}
	}
}

func TestOpenRejectsUnknownFiles(t *testing.T) {
	junk := make([]byte, 512)
	if _, err := rand.Read(junk); err != nil {
		t.Fatal(err)
	}
	junk[0] = 0 // never mistaken for text or a zip
	path := filepath.Join(t.TempDir(), "mystery.bin")
	if err := os.WriteFile(path, junk, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Open(path)
	if err == nil {
		t.Fatal("Open accepted random bytes")
	}
	if !strings.Contains(err.Error(), "mystery.bin") || !strings.Contains(err.Error(), "not a supported ebook") {
		t.Errorf("error %q should name the file and say it is not a supported ebook", err)
	}
}

func TestOpenRejectsDamagedArchive(t *testing.T) {
	data := buildZip(t, zipEntry{Name: "mimetype", Data: []byte("application/epub+zip"), Store: true})
	path := filepath.Join(t.TempDir(), "broken.epub")
	if err := os.WriteFile(path, data[:len(data)-10], 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Open(path)
	if err == nil {
		t.Fatal("Open accepted a truncated archive")
	}
	if !strings.Contains(err.Error(), "broken.epub") || !strings.Contains(err.Error(), "damaged") {
		t.Errorf("error %q should name the file and say the archive is damaged", err)
	}
}

func TestOpenReachesFormatReaders(t *testing.T) {
	dir := t.TempDir()
	files := map[string]struct {
		data   []byte
		format string
	}{
		"b.cbz":  {buildZip(t, zipEntry{Name: "01.png", Data: pngOf(t, 4, 4)}), "Comic archive (CBZ)"},
		"c.fb2":  {[]byte("<FictionBook><body><section><p>Some text.</p></section></body></FictionBook>"), "FictionBook 2"},
		"d.txt":  {[]byte("Some text."), "Plain text"},
		"e.html": {[]byte("<p>Some text.</p>"), "HTML"},
	}
	for name, f := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, f.data, 0o644); err != nil {
			t.Fatal(err)
		}
		b, err := Open(path)
		if err != nil {
			t.Errorf("Open(%s): %v", name, err)
			continue
		}
		if b.Format != f.format {
			t.Errorf("Open(%s) read it as %q, want %q", name, b.Format, f.format)
		}
	}
}
