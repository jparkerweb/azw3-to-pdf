package pdfout

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/jparkerweb/azw3-to-pdf/internal/htmldoc"
)

func TestLookupPageSize(t *testing.T) {
	if got, err := LookupPageSize("a4"); err != nil || got.Name != "a4" {
		t.Errorf("LookupPageSize(a4) = (%v, %v)", got, err)
	}
	if got, err := LookupPageSize(""); err != nil || got.Name != "a5" {
		t.Errorf("an empty page size should default to a5, got (%v, %v)", got, err)
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
