package ebook

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/jparkerweb/ebook-to-pdf/internal/htmldoc"
)

// pngOf encodes a blank w x h PNG.
func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

const containerXML = `<?xml version="1.0"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`

// epubZip builds an EPUB whose package file is OEBPS/content.opf, with the
// mimetype and container entries every EPUB starts with.
func epubZip(t *testing.T, opf string, entries ...zipEntry) []byte {
	t.Helper()
	all := []zipEntry{
		{Name: "mimetype", Data: []byte("application/epub+zip"), Store: true},
		{Name: "META-INF/container.xml", Data: []byte(containerXML)},
		{Name: "OEBPS/content.opf", Data: []byte(opf)},
	}
	return buildZip(t, append(all, entries...)...)
}

func xhtml(head, body string) []byte {
	return []byte(`<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head>` + head + `</head><body>` + body + `</body></html>`)
}

// headingsOf parses a book's markup and returns its headings as "level:text".
func headingsOf(b *Book) []string {
	doc := htmldoc.ParseWithCSS(b.HTML, htmldoc.ParseCSS(b.Flows...))
	var out []string
	for _, i := range doc.Headings() {
		blk := doc.Blocks[i]
		out = append(out, strconv.Itoa(blk.Level)+":"+strings.TrimSpace(blk.Text()))
	}
	return out
}

var recindexAttr = regexp.MustCompile(`recindex="(\d+)"`)

func reflowableEPUB3(t *testing.T) []byte {
	t.Helper()
	opf := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>The Test Book</dc:title>
    <dc:creator>Ann Author</dc:creator>
    <dc:creator>Bob Writer</dc:creator>
    <dc:identifier id="uid">urn:isbn:9780000000002</dc:identifier>
    <dc:language>en</dc:language>
    <dc:publisher>Test Press</dc:publisher>
    <meta property="dcterms:modified">2026-01-01T00:00:00Z</meta>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="css" href="styles/book.css" media-type="text/css"/>
    <item id="ch2" href="text/ch2.xhtml" media-type="application/xhtml+xml"/>
    <item id="ch1" href="text/ch1.xhtml" media-type="application/xhtml+xml"/>
    <item id="coverpage" href="text/cover.xhtml" media-type="application/xhtml+xml"/>
    <item id="cover" href="images/cover.png" media-type="image/png" properties="cover-image"/>
    <item id="pic1" href="images/pic1.png" media-type="image/png"/>
    <item id="pic2" href="images/pic2.png" media-type="image/png"/>
  </manifest>
  <spine>
    <itemref idref="coverpage"/>
    <itemref idref="ch1"/>
    <itemref idref="ch2"/>
  </spine>
</package>`
	nav := xhtml(`<title>Contents</title>`, `<nav epub:type="toc"><ol>
  <li><a href="text/ch1.xhtml">Chapter One</a>
    <ol><li><a href="text/ch2.xhtml#sec">Section Two</a></li></ol>
  </li></ol></nav>`)
	cover := xhtml(`<title>Cover</title>`, `<div><svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 600 800">
  <image width="600" height="800" xlink:href="../images/cover.png"/></svg></div>`)
	ch1 := xhtml(`<title/><link rel="stylesheet" type="text/css" href="../styles/book.css"/>`,
		`<p class="chapter">Chapter One</p><p>First chapter text.</p><p><img src="../images/pic1.png" alt="One"/></p>`)
	ch2 := xhtml(`<title>Two</title><link rel="stylesheet" href="../styles/book.css"/><style>.note { text-align: center }</style>`,
		`<p>Opening words.</p><p id="sec" class="chapter">Section Two</p><p>Second chapter text.</p><img src="../images/pic2.png"/>`)
	return epubZip(t, opf,
		zipEntry{Name: "OEBPS/nav.xhtml", Data: nav},
		zipEntry{Name: "OEBPS/styles/book.css", Data: []byte(`@import url(extra.css); .chapter { font-size: 1.4em }`)},
		zipEntry{Name: "OEBPS/text/cover.xhtml", Data: cover},
		zipEntry{Name: "OEBPS/text/ch1.xhtml", Data: ch1},
		zipEntry{Name: "OEBPS/text/ch2.xhtml", Data: ch2},
		zipEntry{Name: "OEBPS/images/cover.png", Data: pngOf(t, 6, 8)},
		zipEntry{Name: "OEBPS/images/pic1.png", Data: pngOf(t, 2, 2)},
		zipEntry{Name: "OEBPS/images/pic2.png", Data: pngOf(t, 3, 3)},
	)
}

func TestEPUB3Reflowable(t *testing.T) {
	b, err := Decode(reflowableEPUB3(t))
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "The Test Book" || strings.Join(b.Authors, "|") != "Ann Author|Bob Writer" {
		t.Errorf("title %q, authors %q", b.Title, b.Authors)
	}
	if b.ISBN != "9780000000002" || b.Publisher != "Test Press" || b.Language != "en" {
		t.Errorf("ISBN %q, publisher %q, language %q", b.ISBN, b.Publisher, b.Language)
	}
	if b.Format != "EPUB 3" || b.Encoding != "UTF-8" || b.Paged {
		t.Errorf("format %q, encoding %q, paged %v", b.Format, b.Encoding, b.Paged)
	}
	if b.TextBytes != len(b.HTML) {
		t.Errorf("TextBytes = %d, want %d", b.TextBytes, len(b.HTML))
	}
	if b.Cover == nil || b.Cover.Index != 1 || b.Cover.Kind != "png" {
		t.Fatalf("cover = %+v, want resource 1 (the cover-image item)", b.Cover)
	}
	if len(b.Resources) != 3 {
		t.Errorf("%d resources, want 3", len(b.Resources))
	}

	for _, m := range recindexAttr.FindAllStringSubmatch(b.HTML, -1) {
		n, _ := strconv.Atoi(m[1])
		if !b.Resources[n].IsImage() {
			t.Errorf("recindex %d is not a numbered image", n)
		}
		if n == b.Cover.Index {
			t.Errorf("the cover page is still in the markup")
		}
	}
	if got := len(recindexAttr.FindAllString(b.HTML, -1)); got != 2 {
		t.Errorf("%d images in the markup, want 2", got)
	}
	if strings.Contains(b.HTML, "src=") || strings.Contains(b.HTML, "<svg") {
		t.Errorf("markup still has a src or an svg:\n%s", b.HTML)
	}

	one, two := strings.Index(b.HTML, "First chapter text"), strings.Index(b.HTML, "Second chapter text")
	if one < 0 || two < 0 || one > two {
		t.Errorf("chapters out of spine order (ch1 at %d, ch2 at %d)", one, two)
	}

	if len(b.Flows) != 2 || !strings.Contains(b.Flows[0], ".chapter") || !strings.Contains(b.Flows[1], ".note") {
		t.Errorf("flows = %q, want the linked sheet once then the inline style", b.Flows)
	}
	if strings.Contains(strings.Join(b.Flows, ""), "@import") {
		t.Errorf("@import left in the flows: %q", b.Flows)
	}

	want := []string{"1:Chapter One", "2:Section Two"}
	if got := headingsOf(b); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("headings = %q, want %q", got, want)
	}
}

func TestEPUB2(t *testing.T) {
	opf := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="2.0" unique-identifier="uid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
    <dc:title>Old Book</dc:title>
    <dc:creator opf:role="aut">Cy Classic</dc:creator>
    <dc:identifier id="uid" opf:scheme="ISBN">9781111111111</dc:identifier>
    <dc:description>&lt;p&gt;Hello &lt;b&gt;world&lt;/b&gt;&lt;/p&gt;</dc:description>
    <dc:date>1901-02-03</dc:date>
    <dc:subject>Fiction</dc:subject>
    <dc:subject>Classics</dc:subject>
    <meta name="cover" content="cover-img"/>
  </metadata>
  <manifest>
    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>
    <item id="cover-img" href="images/cover.png" media-type="image/png"/>
    <item id="pic" href="images/my%20pic.png" media-type="image/png"/>
    <item id="notes" href="text/notes.xhtml" media-type="application/xhtml+xml"/>
    <item id="ch1" href="text/ch1.xhtml" media-type="application/xhtml+xml"/>
    <item id="ch2" href="text/ch2.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine toc="ncx">
    <itemref idref="notes" linear="no"/>
    <itemref idref="ch1"/>
    <itemref idref="ch2"/>
  </spine>
</package>`
	ncx := `<?xml version="1.0"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1"><navMap>
  <navPoint id="n1"><navLabel><text>Chapter 1</text></navLabel><content src="text/ch1.xhtml#c1"/>
    <navPoint id="n2"><navLabel><text>Part</text></navLabel><content src="text/ch2.xhtml#s2"/></navPoint>
  </navPoint>
</navMap></ncx>`
	data := epubZip(t, opf,
		zipEntry{Name: "OEBPS/toc.ncx", Data: []byte(ncx)},
		zipEntry{Name: "OEBPS/text/notes.xhtml", Data: xhtml("", `<p>Endnote text.</p>`)},
		zipEntry{Name: "OEBPS/text/ch1.xhtml", Data: xhtml("", `<a id="c1"/><p>The First Chapter</p><p>Body one.</p><p><img src="../images/my%20pic.png"/></p>`)},
		zipEntry{Name: "OEBPS/text/ch2.xhtml", Data: xhtml("", `<p>Body two.</p><div id="s2">A Part</div><p>More.</p>`)},
		zipEntry{Name: "OEBPS/images/cover.png", Data: pngOf(t, 2, 3)},
		zipEntry{Name: "OEBPS/images/my pic.png", Data: pngOf(t, 4, 4)},
	)
	b, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "Old Book" || b.AuthorLine() != "Cy Classic" || b.ISBN != "9781111111111" {
		t.Errorf("title %q, authors %q, ISBN %q", b.Title, b.Authors, b.ISBN)
	}
	if b.Description != "Hello world" || b.Published != "1901-02-03" || strings.Join(b.Subjects, ",") != "Fiction,Classics" {
		t.Errorf("description %q, published %q, subjects %q", b.Description, b.Published, b.Subjects)
	}
	if b.Format != "EPUB 2" {
		t.Errorf("format %q, want EPUB 2", b.Format)
	}
	if b.Cover == nil || b.Cover.Index != 1 {
		t.Errorf("cover = %+v, want resource 1 (meta name=cover)", b.Cover)
	}
	if !strings.Contains(b.HTML, `recindex="2"`) {
		t.Errorf("the %%20 image was not resolved:\n%s", b.HTML)
	}
	one, two, notes := strings.Index(b.HTML, "Body one"), strings.Index(b.HTML, "Body two"), strings.Index(b.HTML, "Endnote text")
	if one < 0 || two < 0 || notes < 0 || one >= two || two >= notes {
		t.Errorf("order: ch1 at %d, ch2 at %d, linear=no at %d; want the non-linear item last", one, two, notes)
	}
	want := []string{"1:The First Chapter", "2:A Part"}
	if got := headingsOf(b); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("headings = %q, want %q", got, want)
	}
}

const minimalOPF = `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Locked</dc:title></metadata>
  <manifest><item id="ch1" href="ch1.xhtml" media-type="application/xhtml+xml"/></manifest>
  <spine><itemref idref="ch1"/></spine>
</package>`

func encryptionXML(algorithms ...string) []byte {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0"?><encryption xmlns="urn:oasis:names:tc:opendocument:xmlns:container" xmlns:enc="http://www.w3.org/2001/04/xmlenc#">`)
	for i, a := range algorithms {
		fmt.Fprintf(&sb, `<enc:EncryptedData><enc:EncryptionMethod Algorithm="%s"/>`+
			`<enc:CipherData><enc:CipherReference URI="OEBPS/fonts/f%d.otf"/></enc:CipherData></enc:EncryptedData>`, a, i)
	}
	sb.WriteString(`</encryption>`)
	return []byte(sb.String())
}

func lockedEPUB(t *testing.T, lock zipEntry) []byte {
	return epubZip(t, minimalOPF, lock, zipEntry{Name: "OEBPS/ch1.xhtml", Data: xhtml("", "<p>Secret.</p>")})
}

func TestEPUBDRM(t *testing.T) {
	cases := []struct {
		label  string
		lock   zipEntry
		scheme string
	}{
		{"Adobe", zipEntry{Name: "META-INF/rights.xml", Data: []byte("<rights/>")}, "Adobe ADEPT"},
		{"Apple", zipEntry{Name: "META-INF/sinf.xml", Data: []byte("<sinf/>")}, "Apple FairPlay"},
		{"Readium", zipEntry{Name: "META-INF/license.lcpl", Data: []byte("{}")}, "Readium LCP"},
		{"AES", zipEntry{Name: "META-INF/encryption.xml", Data: encryptionXML(
			"http://www.idpf.org/2008/embedding", "http://www.w3.org/2001/04/xmlenc#aes128-cbc")}, "unrecognised"},
	}
	for _, c := range cases {
		_, err := Decode(lockedEPUB(t, c.lock))
		if !errors.Is(err, ErrDRMLocked) {
			t.Errorf("%s: err = %v, want ErrDRMLocked", c.label, err)
			continue
		}
		if !strings.Contains(err.Error(), c.scheme) || !strings.Contains(err.Error(), "DRM-free copy") {
			t.Errorf("%s: message %q should name %s and suggest a DRM-free copy", c.label, err, c.scheme)
		}
	}

	fonts := lockedEPUB(t, zipEntry{Name: "META-INF/encryption.xml", Data: encryptionXML(
		"http://www.idpf.org/2008/embedding", "http://www.idpf.org/2008/embedding")})
	if b, err := Decode(fonts); err != nil || !strings.Contains(b.HTML, "Secret.") {
		t.Errorf("font obfuscation only: err = %v, want the book to decode", err)
	}

	path := filepath.Join(t.TempDir(), "locked.epub")
	if err := os.WriteFile(path, lockedEPUB(t, cases[0].lock), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Open(path)
	if !errors.Is(err, ErrDRMLocked) || !strings.Contains(err.Error(), path) {
		t.Errorf("Open: err = %v, want ErrDRMLocked naming %s", err, path)
	}
}

func TestEPUBFixedLayout(t *testing.T) {
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
    <item id="i1" href="i1.png" media-type="image/png"/>
    <item id="i3" href="i3.png" media-type="image/png"/>
  </manifest>
  <spine><itemref idref="p1"/><itemref idref="p2"/><itemref idref="p3"/></spine>
</package>`
	b, err := Decode(epubZip(t, opf,
		zipEntry{Name: "OEBPS/p1.xhtml", Data: xhtml(`<meta name="viewport" content="width=1200, height=1600"/>`, `<img src="i1.png"/>`)},
		zipEntry{Name: "OEBPS/p2.xhtml", Data: xhtml(`<meta name="viewport" content="width=1000px,height=1400px"/>`, `<p>Words on a page.</p>`)},
		zipEntry{Name: "OEBPS/p3.xhtml", Data: xhtml("", `<div><img src="i3.png"/></div>`)},
		zipEntry{Name: "OEBPS/i1.png", Data: pngOf(t, 10, 10)},
		zipEntry{Name: "OEBPS/i3.png", Data: pngOf(t, 800, 1000)},
	))
	if err != nil {
		t.Fatal(err)
	}
	if !b.Paged || !strings.HasSuffix(b.Format, "(fixed layout)") {
		t.Errorf("paged %v, format %q", b.Paged, b.Format)
	}

	doc := htmldoc.ParseWithCSS(b.HTML, nil)
	type page struct {
		w, h     float64
		fullPage bool
	}
	var pages []page
	for _, blk := range doc.Blocks {
		switch {
		case blk.Kind == htmldoc.KindPageBreak && blk.PageWidth > 0:
			pages = append(pages, page{w: blk.PageWidth, h: blk.PageHeight})
		case blk.Kind == htmldoc.KindImage && blk.FullPage && len(pages) > 0:
			pages[len(pages)-1].fullPage = true
		}
	}
	want := []page{{900, 1200, true}, {750, 1050, false}, {600, 750, true}}
	if len(pages) != len(want) {
		t.Fatalf("pages = %+v, want %+v", pages, want)
	}
	for i := range want {
		if pages[i] != want[i] {
			t.Errorf("page %d = %+v, want %+v", i+1, pages[i], want[i])
		}
	}
	if !strings.Contains(b.HTML, "Words on a page.") {
		t.Errorf("the text page lost its text")
	}
}

func TestEPUBFixedLayoutApple(t *testing.T) {
	opf := strings.Replace(minimalOPF, `version="3.0"`, `version="2.0"`, 1)
	b, err := Decode(epubZip(t, opf,
		zipEntry{Name: "META-INF/com.apple.ibooks.display-options.xml", Data: []byte(`<?xml version="1.0"?>
<display_options><platform name="*"><option name="fixed-layout">true</option></platform></display_options>`)},
		zipEntry{Name: "OEBPS/ch1.xhtml", Data: xhtml("", "<p>Page.</p>")},
	))
	if err != nil {
		t.Fatal(err)
	}
	if !b.Paged || b.Format != "EPUB 2 (fixed layout)" {
		t.Errorf("paged %v, format %q, want an EPUB 2 fixed-layout book", b.Paged, b.Format)
	}
}

func TestEPUBFixedLayoutReflowableItem(t *testing.T) {
	opf := `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Mixed</dc:title>
    <meta property="rendition:layout">pre-paginated</meta>
    <meta property="rendition:viewport">width=600, height=900</meta>
  </metadata>
  <manifest>
    <item id="p1" href="p1.xhtml" media-type="application/xhtml+xml"/>
    <item id="r1" href="r1.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="p1"/><itemref idref="r1" properties="rendition:layout-reflowable"/></spine>
</package>`
	b, err := Decode(epubZip(t, opf,
		zipEntry{Name: "OEBPS/p1.xhtml", Data: xhtml("", "<p>Fixed page.</p>")},
		zipEntry{Name: "OEBPS/r1.xhtml", Data: xhtml("", "<p>Flowing afterword.</p>")},
	))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(b.HTML, "<ebook:page"); n != 1 {
		t.Errorf("%d sized pages, want 1 (the reflowable item has none):\n%s", n, b.HTML)
	}
	if !strings.Contains(b.HTML, `<ebook:page width="450" height="675">`) {
		t.Errorf("the fixed page did not take the OPF viewport:\n%s", b.HTML)
	}
	after := b.HTML[strings.Index(b.HTML, "Fixed page."):]
	if !strings.Contains(after, "Flowing afterword.") || strings.Contains(after, "<ebook:page") {
		t.Errorf("the reflowable item was not emitted unsized after the fixed page:\n%s", b.HTML)
	}
}

func TestEPUBMissingPackage(t *testing.T) {
	data := buildZip(t, zipEntry{Name: "mimetype", Data: []byte("application/epub+zip"), Store: true})
	if _, err := Decode(data); err == nil || !strings.Contains(err.Error(), "no package file") {
		t.Errorf("err = %v, want the missing package file message", err)
	}
	empty := strings.Replace(minimalOPF, `<itemref idref="ch1"/>`, "", 1)
	if _, err := Decode(epubZip(t, empty)); err == nil || !strings.Contains(err.Error(), "no readable chapters") {
		t.Errorf("err = %v, want the no readable chapters message", err)
	}
}

// opfWith builds an EPUB 3 package around a manifest and spine.
func opfWith(metadata, manifest, spine, guide string) string {
	return `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>T</dc:title>` + metadata + `</metadata>
  <manifest>` + manifest + `</manifest>
  <spine>` + spine + `</spine>` + guide + `
</package>`
}

func TestEPUBHeadingTargets(t *testing.T) {
	opf := opfWith("", `
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/>`,
		`<itemref idref="c1"/>`, "")
	nav := xhtml("", `<nav epub:type="toc"><ol>
  <li><a href="c1.xhtml#wrap">Chapter</a><ol>
    <li><a href="c1.xhtml#sub">Sub</a></li>
  </ol></li></ol></nav>`)
	// A chapter wrapped in a <section> target, and an inline anchor at the
	// start of a later title.
	chapter := xhtml("", `<section id="wrap"><p class="ct">Wrapped Title</p><p>Body.</p>`+
		`<p class="sub"><a id="sub"></a>Sub Title</p><p>More.</p></section>`)
	b, err := Decode(epubZip(t, opf,
		zipEntry{Name: "OEBPS/nav.xhtml", Data: nav},
		zipEntry{Name: "OEBPS/c1.xhtml", Data: chapter}))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"1:Wrapped Title", "2:Sub Title"}
	if got := headingsOf(b); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("headings = %q, want %q", got, want)
	}
}

func TestEPUBByteOrderMark(t *testing.T) {
	opf := opfWith("", `
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="cp" href="cover.xhtml" media-type="application/xhtml+xml"/>
    <item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/>
    <item id="cover" href="cover.jpg" media-type="image/jpg" properties="cover-image"/>`,
		`<itemref idref="cp"/><itemref idref="c1"/>`, "")
	bom := func(b []byte) []byte { return append([]byte("\xef\xbb\xbf"), b...) }
	jpeg := []byte("\xff\xd8\xff\xe0fake jpeg bytes")
	b, err := Decode(epubZip(t, opf,
		zipEntry{Name: "OEBPS/nav.xhtml", Data: xhtml("", `<nav epub:type="toc"><ol><li><a href="c1.xhtml">One</a></li></ol></nav>`)},
		zipEntry{Name: "OEBPS/cover.xhtml", Data: bom(xhtml("", `<img src="cover.jpg"/>`))},
		zipEntry{Name: "OEBPS/c1.xhtml", Data: bom(xhtml("", `<p>Chapter One</p><p>Text.</p>`))},
		zipEntry{Name: "OEBPS/cover.jpg", Data: jpeg}))
	if err != nil {
		t.Fatal(err)
	}
	if b.Cover == nil || b.Cover.Kind != "jpeg" {
		t.Fatalf("cover = %+v, want the image/jpg item", b.Cover)
	}
	if strings.Contains(b.HTML, "recindex") {
		t.Errorf("the BOM-prefixed cover page was kept:\n%s", b.HTML)
	}
	if got := headingsOf(b); strings.Join(got, "|") != "1:Chapter One" {
		t.Errorf("headings = %q, want the BOM-prefixed chapter's title", got)
	}
}

func TestEPUBRewritesAndCoverPages(t *testing.T) {
	opf := opfWith("", `
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="intro" href="intro.xhtml" media-type="application/xhtml+xml"/>
    <item id="gc" href="guidecover.xhtml" media-type="application/xhtml+xml"/>
    <item id="lc" href="landcover.xhtml" media-type="application/xhtml+xml"/>
    <item id="c1" href="c1.xhtml" media-type="application/xhtml+xml"/>
    <item id="cover" href="cover.png" media-type="image/png" properties="cover-image"/>`,
		`<itemref idref="intro"/><itemref idref="gc"/><itemref idref="lc"/><itemref idref="c1"/>`,
		`<guide><reference type="cover" href="guidecover.xhtml"/></guide>`)
	nav := xhtml("", `<nav epub:type="landmarks"><ol><li><a epub:type="cover" href="landcover.xhtml">Cover</a></li></ol></nav>`)
	b, err := Decode(epubZip(t, opf,
		zipEntry{Name: "OEBPS/nav.xhtml", Data: nav},
		zipEntry{Name: "OEBPS/intro.xhtml", Data: xhtml("", `<p>Intro.</p>`)},
		zipEntry{Name: "OEBPS/guidecover.xhtml", Data: xhtml("", `<img src="cover.png"/>`)},
		zipEntry{Name: "OEBPS/landcover.xhtml", Data: xhtml("", `<div><img src="cover.png"/></div>`)},
		zipEntry{Name: "OEBPS/c1.xhtml", Data: xhtml(`<script>alert(1)</script>`,
			`<p>Text.<script>document.write("x")</script></p><img src="https://example.com/a.png"/><img src="missing.png"/>`)},
		zipEntry{Name: "OEBPS/cover.png", Data: pngOf(t, 2, 2)}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(b.HTML, "recindex") {
		t.Errorf("a guide or landmark cover page was kept:\n%s", b.HTML)
	}
	if !strings.Contains(b.HTML, "Intro.") {
		t.Errorf("the first spine item, which is not a cover page, was dropped")
	}
	for _, gone := range []string{"<script", "example.com", "missing.png", "<img"} {
		if strings.Contains(b.HTML, gone) {
			t.Errorf("markup still contains %q:\n%s", gone, b.HTML)
		}
	}
}

func TestEPUBFixedPageSizeFallbacks(t *testing.T) {
	opf := `<?xml version="1.0"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>Sizes</dc:title><meta property="rendition:layout">pre-paginated</meta>
  </metadata>
  <manifest>
    <item id="p1" href="p1.xhtml" media-type="application/xhtml+xml"/>
    <item id="p2" href="p2.xhtml" media-type="application/xhtml+xml"/>
    <item id="p3" href="p3.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="p1"/><itemref idref="p2"/><itemref idref="p3"/></spine>
</package>`
	sizes := func(b *Book) []string {
		var out []string
		for _, m := range regexp.MustCompile(`<ebook:page width="([\d.]+)" height="([\d.]+)">`).FindAllStringSubmatch(b.HTML, -1) {
			out = append(out, m[1]+"x"+m[2])
		}
		return out
	}

	// p1 borrows the first size found later, p2 is an <svg> page, p3 takes
	// the previous page's size.
	b, err := Decode(epubZip(t, opf,
		zipEntry{Name: "OEBPS/p1.xhtml", Data: xhtml("", `<p>Before.</p>`)},
		zipEntry{Name: "OEBPS/p2.xhtml", Data: xhtml("", `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 400 600"><rect/></svg>`)},
		zipEntry{Name: "OEBPS/p3.xhtml", Data: xhtml("", `<p>After.</p>`)}))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(sizes(b), " "); got != "300x450 300x450 300x450" {
		t.Errorf("sizes = %s, want every page at the svg's 300x450", got)
	}

	b, err = Decode(epubZip(t, opf,
		zipEntry{Name: "OEBPS/p1.xhtml", Data: xhtml("", `<p>One.</p>`)},
		zipEntry{Name: "OEBPS/p2.xhtml", Data: xhtml("", `<p>Two.</p>`)},
		zipEntry{Name: "OEBPS/p3.xhtml", Data: xhtml("", `<p>Three.</p>`)}))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(sizes(b), " "); got != "450x600 450x600 450x600" {
		t.Errorf("sizes = %s, want the 600 x 800 pixel default", got)
	}
}

func TestEPUBChapterReadErrors(t *testing.T) {
	opf := minimalOPF
	lowerZipLimits(t, 256<<20, int64(len(containerXML)+len(opf)+100))
	_, err := Decode(epubZip(t, opf,
		zipEntry{Name: "OEBPS/ch1.xhtml", Data: xhtml("", "<p>"+strings.Repeat("words ", 100)+"</p>")}))
	if err == nil || !strings.Contains(err.Error(), "ch1.xhtml") || strings.Contains(err.Error(), "no readable chapters") {
		t.Errorf("err = %v, want the size limit naming ch1.xhtml", err)
	}
}
