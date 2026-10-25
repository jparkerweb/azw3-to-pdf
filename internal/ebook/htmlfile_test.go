package ebook

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testPNG is enough of a PNG for sniffImage.
var testPNG = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR")

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHTMLFileImagesStayInFolder(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "outside.png"), testPNG)
	writeTestFile(t, filepath.Join(dir, "book", "img", "a.png"), testPNG)
	writeTestFile(t, filepath.Join(dir, "book", "style.css"), []byte("p { text-indent: 1em }"))
	page := `<!DOCTYPE html>
<html lang="en"><head>
<title>The Test Book</title>
<meta name="author" content="Ada Writer">
<meta name="description" content="A book for tests.">
<meta name="keywords" content="tests, fixtures">
<link rel="stylesheet" href="style.css">
<style>h1 { text-align: center }</style>
<script>alert("no")</script>
</head><body>
<h1>Chapter</h1>
<img src="img/a.png" alt="kept">
<img src="../outside.png" alt="outside">
<img src="https://example.com/x.png" alt="remote">
<img src="data:image/png;base64,` + base64.StdEncoding.EncodeToString(testPNG) + `" alt="inline">
<img src="img/missing.png" alt="missing">
<img src="img/a.png" alt="again">
</body></html>`
	path := filepath.Join(dir, "book", "index.html")
	writeTestFile(t, path, []byte(page))

	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if b.Format != "HTML" {
		t.Errorf("Format = %q, want HTML", b.Format)
	}
	if b.TextBytes != len(b.HTML) {
		t.Errorf("TextBytes = %d, want the markup length %d", b.TextBytes, len(b.HTML))
	}
	if b.Title != "The Test Book" {
		t.Errorf("Title = %q", b.Title)
	}
	if len(b.Authors) != 1 || b.Authors[0] != "Ada Writer" {
		t.Errorf("Authors = %q", b.Authors)
	}
	if b.Description != "A book for tests." || b.Language != "en" {
		t.Errorf("Description = %q, Language = %q", b.Description, b.Language)
	}
	if strings.Join(b.Subjects, "|") != "tests|fixtures" {
		t.Errorf("Subjects = %q", b.Subjects)
	}
	if len(b.Resources) != 2 || b.Resources[1].Kind != "png" || b.Resources[2].Kind != "png" {
		t.Fatalf("Resources = %v, want the local and the data: PNG", b.Resources)
	}
	for _, alt := range []string{"outside", "remote", "missing"} {
		if strings.Contains(b.HTML, `alt="`+alt+`"`) {
			t.Errorf("the %s image should have been dropped:\n%s", alt, b.HTML)
		}
	}
	for _, want := range []string{
		`<img alt="kept" recindex="1"/>`,
		`<img alt="inline" recindex="2"/>`,
		`<img alt="again" recindex="1"/>`,
	} {
		if !strings.Contains(b.HTML, want) {
			t.Errorf("HTML is missing %s:\n%s", want, b.HTML)
		}
	}
	if strings.Contains(b.HTML, "<script") || strings.Contains(b.HTML, "src=") {
		t.Errorf("scripts and src attributes should be gone:\n%s", b.HTML)
	}
	css := strings.Join(b.Flows, "\n")
	if !strings.Contains(css, "text-indent: 1em") || !strings.Contains(css, "text-align: center") {
		t.Errorf("Flows = %q, want the linked and the inline stylesheet", b.Flows)
	}
}

func TestHTMLFileDecodesDeclaredCharset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quotes.html")
	// 0x93 and 0x94 are curly double quotes in windows-1252.
	page := "<html><head><meta charset=\"windows-1252\"><title>Quotes</title></head>" +
		"<body><p>\x93Hello\x94</p></body></html>"
	writeTestFile(t, path, []byte(page))

	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.HTML, "\u201cHello\u201d") {
		t.Errorf("quotes were not decoded:\n%s", b.HTML)
	}
	if b.Encoding != "windows-1252" {
		t.Errorf("Encoding = %q, want windows-1252", b.Encoding)
	}
}

func TestHTMLFileTitleFallsBackToFileName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "My Story.html")
	writeTestFile(t, path, []byte("<p>Some text.</p>"))

	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "My Story" {
		t.Errorf("Title = %q, want the file name", b.Title)
	}
}

func TestHTMLFileSymlinkedFolderStaysInFolder(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "secret", "x.png"), testPNG)
	writeTestFile(t, filepath.Join(dir, "book", "img", "a.png"), testPNG)
	if err := os.Symlink(filepath.Join(dir, "secret"), filepath.Join(dir, "book", "linked")); err != nil {
		t.Skipf("cannot create a symbolic link here: %v", err)
	}
	path := filepath.Join(dir, "book", "index.html")
	writeTestFile(t, path, []byte(`<p>Text</p><img src="linked/x.png" alt="escaped"><img src="img/a.png" alt="kept">`))

	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Resources) != 1 || strings.Contains(b.HTML, `alt="escaped"`) {
		t.Errorf("an image behind a linked folder outside the tree was read:\n%s", b.HTML)
	}
}

func TestHTMLFileXHTMLShortTags(t *testing.T) {
	path := filepath.Join(t.TempDir(), "page.xhtml")
	writeTestFile(t, path, []byte(`<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml"><head><title/></head>
<body><p>First paragraph.</p><p><a id="note"/>Second paragraph.</p></body></html>`))
	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "page" {
		t.Errorf("Title = %q, want the file name, since <title/> is empty", b.Title)
	}
	if !strings.Contains(b.HTML, "First paragraph.") || strings.Contains(b.HTML, `<a id="note">Second`) {
		t.Errorf("short tags swallowed the text that follows them:\n%s", b.HTML)
	}
}
