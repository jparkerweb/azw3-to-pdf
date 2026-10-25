package engine

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// writeTestFile writes data to name in a fresh temporary folder.
func writeTestFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func convertTestBook(t *testing.T, input string) *Result {
	t.Helper()
	res, err := Convert(context.Background(), Options{Input: input, PDF: mustOptions(t)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestConvertCBZ(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i, size := range [][2]int{{400, 600}, {600, 400}, {400, 600}} {
		f, err := w.Create(fmt.Sprintf("pages/%d.png", i+1))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(testPNG(t, size[0], size[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	res := convertTestBook(t, writeTestFile(t, "comic.cbz", buf.Bytes()))
	if res.Pages != 3 {
		t.Errorf("%d pages, want exactly 3 (one per image, no cover or title page)", res.Pages)
	}
	if res.Images != 3 || res.Dropped != 0 {
		t.Errorf("%d images drawn, %d dropped; want 3 and 0", res.Images, res.Dropped)
	}
}

const testFB2 = `<?xml version="1.0" encoding="utf-8"?>
<FictionBook xmlns="http://www.gribuser.ru/xml/fictionbook/2.0">
  <description><title-info>
    <book-title>A Test Saga</book-title>
    <author><first-name>Ann</first-name><last-name>Author</last-name></author>
  </title-info></description>
  <body>
    <section><title><p>Chapter One</p></title><p>The first chapter.</p></section>
    <section><title><p>Chapter Two</p></title><p>The second chapter.</p></section>
  </body>
</FictionBook>`

func TestConvertFB2(t *testing.T) {
	res := convertTestBook(t, writeTestFile(t, "saga.fb2", []byte(testFB2)))
	if res.Headings < 2 {
		t.Errorf("%d bookmarks, want at least 2", res.Headings)
	}
}

func TestConvertPlainText(t *testing.T) {
	text := "CHAPTER I\n\nIt was a dark night.\n\nCHAPTER II\n\nThe morning came.\n"
	res := convertTestBook(t, writeTestFile(t, "story.txt", []byte(text)))
	if res.Headings < 2 {
		t.Errorf("%d bookmarks, want at least 2", res.Headings)
	}
}

func TestConvertHTMLFile(t *testing.T) {
	input := writeTestFile(t, filepath.Join("site", "page.html"),
		[]byte(`<html><head><title>A Page</title></head><body><h1>A Page</h1><p>Words.</p><img src="pic.png"></body></html>`))
	if err := os.WriteFile(filepath.Join(filepath.Dir(input), "pic.png"), testPNG(t, 40, 40), 0o644); err != nil {
		t.Fatal(err)
	}
	res := convertTestBook(t, input)
	if res.Images < 1 || res.Dropped != 0 {
		t.Errorf("%d images drawn, %d dropped; want the local image kept", res.Images, res.Dropped)
	}
}
