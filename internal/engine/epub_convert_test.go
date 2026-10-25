package engine

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// The engine cannot see the ebook package's test helpers, so these are small
// copies of its EPUB fixtures, written with archive/zip.

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func testXHTML(head, body string) []byte {
	return []byte(`<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head>` + head + `</head><body>` + body + `</body></html>`)
}

// writeTestEPUB writes an EPUB with its package at OEBPS/content.opf. files
// is name, contents, name, contents..., in archive order.
func writeTestEPUB(t *testing.T, name, opf string, files ...any) string {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	add := func(name string, data []byte, method uint16) {
		f, err := w.CreateHeader(&zip.FileHeader{Name: name, Method: method})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	add("mimetype", []byte("application/epub+zip"), zip.Store)
	add("META-INF/container.xml", []byte(`<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`), zip.Deflate)
	add("OEBPS/content.opf", []byte(opf), zip.Deflate)
	for i := 0; i+1 < len(files); i += 2 {
		add(files[i].(string), files[i+1].([]byte), zip.Deflate)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConvertReflowableEPUB(t *testing.T) {
	opf := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>The Test Book</dc:title><dc:creator>Ann Author</dc:creator>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="css" href="styles/book.css" media-type="text/css"/>
    <item id="coverpage" href="text/cover.xhtml" media-type="application/xhtml+xml"/>
    <item id="ch1" href="text/ch1.xhtml" media-type="application/xhtml+xml"/>
    <item id="ch2" href="text/ch2.xhtml" media-type="application/xhtml+xml"/>
    <item id="cover" href="images/cover.png" media-type="image/png" properties="cover-image"/>
    <item id="pic1" href="images/pic1.png" media-type="image/png"/>
    <item id="pic2" href="images/pic2.png" media-type="image/png"/>
  </manifest>
  <spine><itemref idref="coverpage"/><itemref idref="ch1"/><itemref idref="ch2"/></spine>
</package>`
	input := writeTestEPUB(t, "book.epub", opf,
		"OEBPS/nav.xhtml", testXHTML(`<title>Contents</title>`, `<nav epub:type="toc"><ol>
  <li><a href="text/ch1.xhtml">Chapter One</a><ol><li><a href="text/ch2.xhtml#sec">Section Two</a></li></ol></li>
</ol></nav>`),
		"OEBPS/styles/book.css", []byte(`.chapter { font-size: 1.4em }`),
		"OEBPS/text/cover.xhtml", testXHTML("", `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><image xlink:href="../images/cover.png"/></svg>`),
		"OEBPS/text/ch1.xhtml", testXHTML(`<title/><link rel="stylesheet" href="../styles/book.css"/>`,
			`<p class="chapter">Chapter One</p><p>First chapter text.</p><p><img src="../images/pic1.png"/></p>`),
		"OEBPS/text/ch2.xhtml", testXHTML(`<link rel="stylesheet" href="../styles/book.css"/>`,
			`<p>Opening words.</p><p id="sec" class="chapter">Section Two</p><p>Second chapter text.</p><img src="../images/pic2.png"/>`),
		"OEBPS/images/cover.png", testPNG(t, 60, 80),
		"OEBPS/images/pic1.png", testPNG(t, 20, 20),
		"OEBPS/images/pic2.png", testPNG(t, 30, 30),
	)

	res, err := Convert(context.Background(), Options{Input: input, PDF: mustOptions(t)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Headings < 2 {
		t.Errorf("%d bookmarks, want at least 2", res.Headings)
	}
	if res.Images < 2 {
		t.Errorf("%d images drawn, want at least 2", res.Images)
	}
	if res.Dropped != 0 {
		t.Errorf("%d images dropped", res.Dropped)
	}
}

func TestConvertFixedLayoutEPUB(t *testing.T) {
	opf := `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Picture Book</dc:title>
    <meta property="rendition:layout">pre-paginated</meta>
  </metadata>
  <manifest>
    <item id="p1" href="p1.xhtml" media-type="application/xhtml+xml"/>
    <item id="p2" href="p2.xhtml" media-type="application/xhtml+xml"/>
    <item id="p3" href="p3.xhtml" media-type="application/xhtml+xml"/>
    <item id="i1" href="i1.png" media-type="image/png" properties="cover-image"/>
    <item id="i3" href="i3.png" media-type="image/png"/>
  </manifest>
  <spine><itemref idref="p1"/><itemref idref="p2"/><itemref idref="p3"/></spine>
</package>`
	input := writeTestEPUB(t, "fixed.epub", opf,
		"OEBPS/p1.xhtml", testXHTML(`<meta name="viewport" content="width=1200, height=1600"/>`, `<img src="i1.png"/>`),
		"OEBPS/p2.xhtml", testXHTML(`<meta name="viewport" content="width=1000, height=1400"/>`, `<p>Words on a page.</p>`),
		"OEBPS/p3.xhtml", testXHTML("", `<img src="i3.png"/>`),
		"OEBPS/i1.png", testPNG(t, 120, 160),
		"OEBPS/i3.png", testPNG(t, 800, 1000),
	)

	res, err := Convert(context.Background(), Options{Input: input, PDF: mustOptions(t)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Pages != 3 {
		t.Errorf("%d pages, want 3 (one per spine page, no cover or title page)", res.Pages)
	}
	if res.Dropped != 0 {
		t.Errorf("%d images dropped", res.Dropped)
	}
}
