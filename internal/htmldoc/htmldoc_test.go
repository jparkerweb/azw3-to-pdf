package htmldoc

import (
	"strings"
	"testing"
)

func TestParseParagraphsAndStyles(t *testing.T) {
	doc := Parse(`<html><body>
		<p>Plain text with <b>bold</b> and <i>italic</i> words.</p>
		<p>Second paragraph.</p>
	</body></html>`)

	if len(doc.Blocks) != 2 {
		t.Fatalf("parsed %d blocks, want 2: %#v", len(doc.Blocks), doc.Blocks)
	}
	if got := doc.Blocks[0].Text(); got != "Plain text with bold and italic words." {
		t.Errorf("first paragraph is %q", got)
	}

	var bold, italic bool
	for _, s := range doc.Blocks[0].Spans {
		if s.Style.Has(StyleBold) && s.Text == "bold" {
			bold = true
		}
		if s.Style.Has(StyleItalic) && s.Text == "italic" {
			italic = true
		}
	}
	if !bold || !italic {
		t.Errorf("styles were lost: bold=%v italic=%v (%#v)", bold, italic, doc.Blocks[0].Spans)
	}
}

func TestParseHeadingsFromTagsAndClasses(t *testing.T) {
	doc := Parse(`<h2>Real heading</h2><p class="h1-top-a">Styled heading</p><p class="hanging">Body</p>`)

	if len(doc.Blocks) != 3 {
		t.Fatalf("parsed %d blocks, want 3", len(doc.Blocks))
	}
	if doc.Blocks[0].Kind != KindHeading || doc.Blocks[0].Level != 2 {
		t.Errorf("<h2> became kind %v level %d", doc.Blocks[0].Kind, doc.Blocks[0].Level)
	}
	if doc.Blocks[1].Kind != KindHeading || doc.Blocks[1].Level != 1 {
		t.Errorf("class \"h1-top-a\" became kind %v level %d", doc.Blocks[1].Kind, doc.Blocks[1].Level)
	}
	if doc.Blocks[2].Kind != KindParagraph {
		t.Errorf("class \"hanging\" was mistaken for a heading")
	}
	if got := len(doc.Headings()); got != 2 {
		t.Errorf("Headings() found %d, want 2", got)
	}
}

func TestParseDropsHeadAndScripts(t *testing.T) {
	doc := Parse(`<html><head><title>Not content</title>
		<style>p { color: red; }</style></head>
		<body><script>alert(1)</script><p>Only this.</p></body></html>`)

	if len(doc.Blocks) != 1 {
		t.Fatalf("parsed %d blocks, want 1: %#v", len(doc.Blocks), doc.Blocks)
	}
	if got := doc.Blocks[0].Text(); got != "Only this." {
		t.Errorf("block text is %q", got)
	}
}

func TestParsePageBreakBetweenParts(t *testing.T) {
	doc := Parse(`<html><body><p>Part one.</p></body></html>` +
		`<html><body><p>Part two.</p></body></html>`)

	var breaks int
	for _, b := range doc.Blocks {
		if b.Kind == KindPageBreak {
			breaks++
		}
	}
	if breaks != 1 {
		t.Errorf("found %d page breaks between two parts, want 1", breaks)
	}
}

func TestParseImageReferences(t *testing.T) {
	doc := Parse(`<p><img src="kindle:embed:000A?mime=image/jpg"/></p>` +
		`<p><img recindex="00003"/></p>` +
		`<p><img src="http://example.com/x.png"/></p>`)

	var refs []int
	for _, b := range doc.Blocks {
		if b.Kind == KindImage {
			refs = append(refs, b.Resource)
		}
	}
	// "000A" is base 32, so it is the tenth resource. The remote image has no
	// resource number and is dropped.
	want := []int{10, 3}
	if len(refs) != len(want) {
		t.Fatalf("found image references %v, want %v", refs, want)
	}
	for i := range want {
		if refs[i] != want[i] {
			t.Errorf("image %d resolved to resource %d, want %d", i, refs[i], want[i])
		}
	}
}

func TestParseCollapsesWhitespace(t *testing.T) {
	doc := Parse("<p>  spaced   out\n\ttext  </p>")
	if len(doc.Blocks) != 1 {
		t.Fatalf("parsed %d blocks, want 1", len(doc.Blocks))
	}
	if got := doc.Blocks[0].Text(); got != "spaced out text" {
		t.Errorf("whitespace collapsed to %q", got)
	}
}

func TestParseSurvivesBrokenMarkup(t *testing.T) {
	// KF8 fragments routinely contain unbalanced tags.
	doc := Parse(`<p>One</b></div><p><b>Two</p><span>Three`)
	var text []string
	for _, b := range doc.Blocks {
		text = append(text, b.Text())
	}
	joined := strings.Join(text, "|")
	for _, want := range []string{"One", "Two", "Three"} {
		if !strings.Contains(joined, want) {
			t.Errorf("%q is missing from %q", want, joined)
		}
	}
}

func TestParseCSS(t *testing.T) {
	sheet := ParseCSS(`
		/* a comment */
		p.h1 { text-align: center; font-size: 290%; }
		p.body, div.body { margin-left: 2em; text-indent: -2em; }
		span.run-in { display: block; font-weight: bold; }
		.broken > p { text-align: right; }
		@page { margin: 0; }
	`)

	h1 := sheet.Lookup("p", "h1")
	if h1.Align != AlignCenter {
		t.Errorf("p.h1 alignment is %v, want centre", h1.Align)
	}
	if h1.Scale < 2.8 || h1.Scale > 3.0 {
		t.Errorf("p.h1 scale is %v, want about 2.9", h1.Scale)
	}

	body := sheet.Lookup("div", "body")
	if body.MarginLeft != 2 || body.Indent != -2 {
		t.Errorf("div.body indents are %v/%v, want 2/-2", body.MarginLeft, body.Indent)
	}

	runIn := sheet.Lookup("span", "run-in")
	if !runIn.Block || !runIn.Bold {
		t.Errorf("span.run-in is %#v, want a bold block", runIn)
	}

	if got := sheet.Lookup("p", "broken"); got.Align != AlignDefault {
		t.Error("a descendant selector should be ignored")
	}
}

func TestParseWithCSSPromotesSpansToBlocks(t *testing.T) {
	sheet := ParseCSS(`span.line { display: block; } p.big { font-size: 200%; }`)
	doc := ParseWithCSS(`<p class="big"><span class="line">Chapter One</span><span class="line">The Beginning</span></p>`, sheet)

	if len(doc.Blocks) != 2 {
		t.Fatalf("parsed %d blocks, want 2: %#v", len(doc.Blocks), doc.Blocks)
	}
	if doc.Blocks[0].Text() != "Chapter One" || doc.Blocks[1].Text() != "The Beginning" {
		t.Errorf("blocks are %q and %q", doc.Blocks[0].Text(), doc.Blocks[1].Text())
	}
}

func TestParseWithCSSPageBreak(t *testing.T) {
	sheet := ParseCSS(`div.chapter { page-break-before: always; }`)
	doc := ParseWithCSS(`<p>End of one.</p><div class="chapter"><p>Two.</p></div>`, sheet)

	if len(doc.Blocks) < 2 || doc.Blocks[1].Kind != KindPageBreak {
		t.Errorf("expected a page break before the chapter, got %#v", doc.Blocks)
	}
}

func TestParseInlinePaddingBecomesASpace(t *testing.T) {
	// Numbered lists in Kindle books put the number in a padded span rather
	// than writing a space, so the padding has to stand in for one.
	sheet := ParseCSS(`span.num { padding-right: 0.35em; padding-left: 0.35em; }`)
	doc := ParseWithCSS(`<p><span class="num">7.</span>Remove the lid.</p>`, sheet)

	if len(doc.Blocks) != 1 {
		t.Fatalf("parsed %d blocks, want 1", len(doc.Blocks))
	}
	if got := doc.Blocks[0].Text(); got != "7. Remove the lid." {
		t.Errorf("block text is %q, want \"7. Remove the lid.\"", got)
	}
}

func TestParseDoesNotDoubleSpaces(t *testing.T) {
	sheet := ParseCSS(`span.num { padding-right: 0.5em; }`)
	doc := ParseWithCSS(`<p><span class="num">7.</span> Already spaced.</p>`, sheet)
	if got := doc.Blocks[0].Text(); got != "7. Already spaced." {
		t.Errorf("block text is %q", got)
	}
}
