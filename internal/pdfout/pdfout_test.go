package pdfout

import (
	"bytes"
	"compress/zlib"
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"regexp"
	"strings"
	"testing"

	"github.com/jparkerweb/ebook-to-pdf/internal/htmldoc"
)

func TestLookupPageSize(t *testing.T) {
	if got, err := LookupPageSize("a4"); err != nil || got.Name != "a4" {
		t.Errorf("LookupPageSize(a4) = (%v, %v)", got, err)
	}
	if got, err := LookupPageSize(""); err != nil || got.Name != "letter" {
		t.Errorf("an empty page size should default to letter, got (%v, %v)", got, err)
	}
	if _, err := LookupPageSize("elephant"); err == nil {
		t.Error("an unknown page size should be rejected")
	}

	custom, err := LookupPageSize("120x160mm")
	if err != nil {
		t.Fatalf("a millimetre measurement was rejected: %v", err)
	}
	if w := custom.Width; w < 340 || w > 341 {
		t.Errorf("120mm came out as %.2f pt, want about 340", w)
	}

	inches, err := LookupPageSize("6x9in")
	if err != nil || inches.Width != 432 || inches.Height != 648 {
		t.Errorf("6x9in came out as %v (%v)", inches, err)
	}
}

func TestOptionsNormalize(t *testing.T) {
	opts := Options{FontSize: 200, LineSpacing: 0.1, Margins: Margins{Left: 9999, Right: -5}}
	opts.Normalize()

	if opts.FontSize > 40 {
		t.Errorf("font size stayed at %v", opts.FontSize)
	}
	if opts.LineSpacing < 1 {
		t.Errorf("line spacing stayed at %v", opts.LineSpacing)
	}
	if opts.Margins.Left >= opts.PageSize.Width/2 {
		t.Errorf("left margin of %v swallows the page", opts.Margins.Left)
	}
	if opts.Margins.Right < 0 {
		t.Errorf("right margin stayed negative: %v", opts.Margins.Right)
	}
	if opts.PageSize.Width == 0 {
		t.Error("an unset page size was not replaced with a default")
	}
}

func TestSplitKeepingSpaces(t *testing.T) {
	got := splitKeepingSpaces(" one two ")
	if len(got) != 2 {
		t.Fatalf("split into %d fields, want 2: %#v", len(got), got)
	}
	if !got[0].leadingSpace {
		t.Error("the leading space was lost, so styled runs would run together")
	}
	if !got[0].trailingSpace || !got[1].trailingSpace {
		t.Errorf("trailing spaces were lost: %#v", got)
	}
	if got[0].text != "one" || got[1].text != "two" {
		t.Errorf("fields are %q and %q", got[0].text, got[1].text)
	}
}

// TestRenderProducesAPDF runs the whole layout engine over a small document.
func TestRenderProducesAPDF(t *testing.T) {
	doc := htmldoc.Parse(`
		<h1>Chapter One</h1>
		<p>` + strings.Repeat("Words that need wrapping across several lines. ", 40) + `</p>
		<hr/>
		<blockquote>A quotation.</blockquote>
		<ul><li>First</li><li>Second</li></ul>
		<p>The end.</p>`)

	opts := DefaultOptions()
	opts.Cover = false

	var buf bytes.Buffer
	res, err := Render(context.Background(), &buf, doc, Meta{Title: "Test", Author: "A. Writer"}, nil, opts, nil)
	if err != nil {
		t.Fatalf("Render returned %v", err)
	}
	if res.Pages < 2 {
		t.Errorf("laid out %d pages, want at least 2", res.Pages)
	}
	if res.Headings != 1 {
		t.Errorf("recorded %d bookmarks, want 1", res.Headings)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Error("the output does not start with a PDF header")
	}
	if !bytes.Contains(buf.Bytes(), []byte("%%EOF")) {
		t.Error("the output has no PDF trailer")
	}
}

func TestRenderRespectsCancellation(t *testing.T) {
	var blocks []htmldoc.Block
	for i := 0; i < 5000; i++ {
		blocks = append(blocks, htmldoc.Block{
			Kind:  htmldoc.KindParagraph,
			Spans: []htmldoc.Span{{Text: "Some text to lay out."}},
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var buf bytes.Buffer
	if _, err := Render(ctx, &buf, &htmldoc.Doc{Blocks: blocks}, Meta{}, nil, DefaultOptions(), nil); err == nil {
		t.Error("Render ignored a cancelled context")
	}
}

func TestRenderEmptyDocumentStillWritesAPDF(t *testing.T) {
	var buf bytes.Buffer
	opts := DefaultOptions()
	opts.TitlePage = false
	opts.Cover = false

	res, err := Render(context.Background(), &buf, &htmldoc.Doc{}, Meta{}, nil, opts, nil)
	if err != nil {
		t.Fatalf("Render returned %v", err)
	}
	if res.Pages != 1 {
		t.Errorf("an empty book produced %d pages, want 1", res.Pages)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Error("the output is not a PDF")
	}
}

// TestBlockSize pins the renderer's own part of the font-size rule: the body
// size times htmldoc.EffectiveScale, never below 5pt.
func TestBlockSize(t *testing.T) {
	r := &renderer{opts: Options{FontSize: 10}}

	cases := []struct {
		name  string
		block htmldoc.Block
		want  float64
	}{
		{"no stylesheet size", htmldoc.Block{Kind: htmldoc.KindParagraph}, 10},
		{"small text hits the floor", htmldoc.Block{Kind: htmldoc.KindParagraph, Scale: 0.5}, 5},
		{"damped large text", htmldoc.Block{Kind: htmldoc.KindParagraph, Scale: 2}, 10 * 1.62},
		{"capped", htmldoc.Block{Kind: htmldoc.KindParagraph, Scale: 3.5}, 24},
		{"heading default", htmldoc.Block{Kind: htmldoc.KindHeading, Level: 1}, 17.5},
		{"heading with a bigger stylesheet size", htmldoc.Block{Kind: htmldoc.KindHeading, Level: 4, Scale: 2}, 10 * 1.62},
	}
	for _, tc := range cases {
		if got := r.blockSize(tc.block); math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("%s: blockSize = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestRenderMixedPageSizes renders a book page, a sized break and a full-page
// image, which between them must produce three pages of three sizes, with
// furniture only on the two text pages.
func TestRenderMixedPageSizes(t *testing.T) {
	var png300x500 bytes.Buffer
	if err := png.Encode(&png300x500, image.NewGray(image.Rect(0, 0, 300, 500))); err != nil {
		t.Fatal(err)
	}
	images := func(index int) (Image, bool) {
		if index != 1 {
			return Image{}, false
		}
		return Image{Data: png300x500.Bytes(), Kind: "png"}, true
	}
	text := func(s string) htmldoc.Block {
		return htmldoc.Block{Kind: htmldoc.KindParagraph, Spans: []htmldoc.Span{{Text: s}}}
	}
	doc := &htmldoc.Doc{Blocks: []htmldoc.Block{
		text("On the book's own page size."),
		{Kind: htmldoc.KindPageBreak, PageWidth: 600, PageHeight: 800},
		text("On a fixed page."),
		{Kind: htmldoc.KindImage, Resource: 1, FullPage: true},
	}}

	render := func(pageNumbers bool) ([]pdfPage, Result) {
		t.Helper()
		opts := DefaultOptions()
		opts.Cover = false
		opts.TitlePage = false
		opts.ChapterBreaks = false // a sized break must start its page anyway
		opts.RunningHeader = false
		opts.PageNumbers = pageNumbers

		var buf bytes.Buffer
		res, err := Render(context.Background(), &buf, doc, Meta{Title: "Mixed"}, images, opts, nil)
		if err != nil {
			t.Fatalf("Render returned %v", err)
		}
		return parsePages(t, buf.Bytes()), res
	}

	pages, res := render(true)
	if len(pages) != 3 || res.Pages != 3 {
		t.Fatalf("got %d page objects and Result.Pages %d, want 3 of each", len(pages), res.Pages)
	}
	if res.Images != 1 || res.Dropped != 0 {
		t.Errorf("Images = %d, Dropped = %d, want 1 and 0", res.Images, res.Dropped)
	}

	base := DefaultOptions().PageSize
	want := []string{
		fmt.Sprintf("%0.2f %0.2f", base.Width, base.Height),
		"600.00 800.00",
		"225.00 375.00",
	}
	for i, p := range pages {
		if p.mediaBox != want[i] {
			t.Errorf("page %d MediaBox is %q, want %q", i+1, p.mediaBox, want[i])
		}
	}

	// Each drawn page number is one more text object on its page.
	bare, _ := render(false)
	for i := range 2 {
		if pages[i].textObjects != bare[i].textObjects+1 {
			t.Errorf("page %d has %d text objects with page numbers and %d without, want one more",
				i+1, pages[i].textObjects, bare[i].textObjects)
		}
	}
	if pages[2].textObjects != 0 {
		t.Errorf("the full-page image page has %d text objects, want none", pages[2].textObjects)
	}
	if !pages[2].drawsImage {
		t.Error("the full-page image page does not draw an image")
	}
}

// TestTextAfterFullPageImage checks that text following an image on a page
// of its own size goes back to the book's page size on a fresh page, rather
// than onto the image's page.
func TestTextAfterFullPageImage(t *testing.T) {
	var small bytes.Buffer
	if err := png.Encode(&small, image.NewGray(image.Rect(0, 0, 300, 500))); err != nil {
		t.Fatal(err)
	}
	images := func(int) (Image, bool) { return Image{Data: small.Bytes(), Kind: "png"}, true }
	doc := &htmldoc.Doc{Blocks: []htmldoc.Block{
		{Kind: htmldoc.KindImage, Resource: 1, FullPage: true},
		{Kind: htmldoc.KindParagraph, Spans: []htmldoc.Span{{Text: "After the picture."}}},
	}}
	opts := DefaultOptions()
	opts.Cover = false
	opts.TitlePage = false
	opts.RunningHeader = false
	opts.PageNumbers = false

	var buf bytes.Buffer
	if _, err := Render(context.Background(), &buf, doc, Meta{Title: "After"}, images, opts, nil); err != nil {
		t.Fatalf("Render returned %v", err)
	}
	pages := parsePages(t, buf.Bytes())
	if len(pages) != 2 {
		t.Fatalf("got %d pages, want the image page and a text page", len(pages))
	}
	if want := fmt.Sprintf("%0.2f %0.2f", opts.PageSize.Width, opts.PageSize.Height); pages[1].mediaBox != want {
		t.Errorf("the text page is %q, want the book's size %q", pages[1].mediaBox, want)
	}
	if pages[0].textObjects != 0 || pages[1].textObjects == 0 {
		t.Errorf("text objects per page = %d, %d; want the text on the second page only",
			pages[0].textObjects, pages[1].textObjects)
	}
}

type pdfPage struct {
	mediaBox    string
	textObjects int
	drawsImage  bool
}

var (
	pdfObject   = regexp.MustCompile(`(?s)(\d+) 0 obj\n(.*?)endobj`)
	pdfContents = regexp.MustCompile(`/Contents\s+(\d+) 0 R`)
	pdfMediaBox = regexp.MustCompile(`/MediaBox \[ 0 0 ([\d.]+ [\d.]+) \]`)
)

// parsePages reads just enough of gopdf's output to list its pages in order,
// with each page's MediaBox and what its (deflated) content stream draws.
func parsePages(t *testing.T, pdf []byte) []pdfPage {
	t.Helper()
	objects := map[string][]byte{}
	var pageBodies [][]byte
	for _, m := range pdfObject.FindAllSubmatch(pdf, -1) {
		objects[string(m[1])] = m[2]
		if bytes.Contains(m[2], []byte("/Type /Page\n")) {
			pageBodies = append(pageBodies, m[2])
		}
	}

	var pages []pdfPage
	for _, body := range pageBodies {
		var p pdfPage
		if m := pdfMediaBox.FindSubmatch(body); m != nil {
			p.mediaBox = string(m[1])
		}
		if m := pdfContents.FindSubmatch(body); m != nil {
			content := objects[string(m[1])]
			start := bytes.Index(content, []byte("stream\n"))
			if start < 0 {
				t.Fatalf("content object %s has no stream", m[1])
			}
			zr, err := zlib.NewReader(bytes.NewReader(content[start+len("stream\n"):]))
			if err != nil {
				t.Fatalf("content object %s: %v", m[1], err)
			}
			ops, err := io.ReadAll(zr)
			if err != nil {
				t.Fatalf("content object %s: %v", m[1], err)
			}
			p.textObjects = bytes.Count(ops, []byte("BT\n"))
			p.drawsImage = bytes.Contains(ops, []byte(" Do"))
		}
		pages = append(pages, p)
	}
	return pages
}

// webp1x1 is a 1 x 1 lossless WebP, small enough to keep inline.
var webp1x1 = []byte{
	0x52, 0x49, 0x46, 0x46, 0x1a, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50,
	0x56, 0x50, 0x38, 0x4c, 0x0d, 0x00, 0x00, 0x00, 0x2f, 0x00, 0x00, 0x00,
	0x10, 0x07, 0x10, 0x11, 0x11, 0x88, 0x88, 0xfe, 0x07, 0x00,
}

func TestDecodeWebP(t *testing.T) {
	img := Image{Data: webp1x1, Kind: "webp"}
	cfg, err := decodeConfig(img)
	if err != nil {
		t.Fatalf("decodeConfig: %v", err)
	}
	if cfg.Width != 1 || cfg.Height != 1 {
		t.Errorf("decodeConfig gave %d x %d, want 1 x 1", cfg.Width, cfg.Height)
	}
	m, err := decodeImage(img)
	if err != nil {
		t.Fatalf("decodeImage: %v", err)
	}
	if b := m.Bounds(); b.Dx() != 1 || b.Dy() != 1 {
		t.Errorf("decodeImage gave bounds %v, want 1 x 1", b)
	}
}
