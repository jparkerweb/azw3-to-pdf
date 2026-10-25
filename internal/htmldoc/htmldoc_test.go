package htmldoc

import (
	"math"
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

// oldEffectiveSize is the font-size rule as it stood before EffectiveScale
// existed (clampScale in css.go, blockSize and heading in pdfout/render.go),
// kept here as an oracle so the refactor is pinned to the old output.
func oldEffectiveSize(bodySize, cssScale float64, kind Kind, level int) float64 {
	clamp := func(f float64) float64 {
		if f < 0.5 {
			return 0.5
		}
		if f > 3.5 {
			return 3.5
		}
		return f
	}
	blockSize := func() float64 {
		if cssScale <= 0 {
			return bodySize
		}
		scale := clamp(cssScale)
		if scale > 1 {
			scale = 1 + (scale-1)*0.62
		}
		size := bodySize * scale
		if size < 5 {
			size = 5
		}
		if max := bodySize * 2.4; size > max {
			size = max
		}
		return size
	}
	if kind != KindHeading {
		return blockSize()
	}
	table := [7]float64{1, 1.75, 1.45, 1.28, 1.15, 1.06, 1.0}
	if level < 1 || level > 6 {
		level = 3
	}
	size := bodySize * table[level]
	if css := blockSize(); cssScale > 0 && css > size {
		size = css
	}
	return size
}

func TestEffectiveScaleMatchesTheOldRule(t *testing.T) {
	scales := []float64{0, 0.1, 0.5, 0.75, 0.92, 1, 1.0001, 1.25, 1.5, 2, 2.4, 2.9, 3, 3.5, 5, 40}
	kinds := []Kind{KindParagraph, KindHeading, KindListItem, KindQuote}
	// Body sizes are 5..40 because Options.Normalize clamps FontSize to that.
	for _, body := range []float64{5, 9.5, 11, 16, 40} {
		for _, css := range scales {
			// The parser never hands over a clamped value, so feed the raw one.
			for _, kind := range kinds {
				for level := -1; level <= 8; level++ {
					got := body * EffectiveScale(css, kind, level)
					if got < 5 {
						got = 5
					}
					want := oldEffectiveSize(body, css, kind, level)
					if got != want {
						t.Fatalf("body %v, css %v, kind %v, level %d: got %v, want %v", body, css, kind, level, got, want)
					}
				}
			}
		}
	}
}

func TestEffectiveScalePinnedValues(t *testing.T) {
	cases := []struct {
		name  string
		css   float64
		kind  Kind
		level int
		want  float64
	}{
		{"unset body text", 0, KindParagraph, 0, 1},
		{"below the lower clamp", 0.1, KindParagraph, 0, 0.5},
		{"at the lower clamp", 0.5, KindParagraph, 0, 0.5},
		{"at body size", 1, KindParagraph, 0, 1},
		{"damped above body size", 2, KindParagraph, 0, 1 + 1*0.62},
		{"capped at 2.4", 3.5, KindParagraph, 0, 2.4},
		{"above the upper clamp", 50, KindParagraph, 0, 2.4},
		{"infinite", math.Inf(1), KindParagraph, 0, 2.4},
		{"heading 1 default", 0, KindHeading, 1, 1.75},
		{"heading 2 default", 0, KindHeading, 2, 1.45},
		{"heading 3 default", 0, KindHeading, 3, 1.28},
		{"heading 4 default", 0, KindHeading, 4, 1.15},
		{"heading 5 default", 0, KindHeading, 5, 1.06},
		{"heading 6 default", 0, KindHeading, 6, 1.0},
		{"bad heading level falls back to 3", 0, KindHeading, 9, 1.28},
		{"heading never below its default", 0.6, KindHeading, 1, 1.75},
		{"heading larger than default wins", 2.9, KindHeading, 6, 1 + 1.9*0.62},
		{"level ignored for paragraphs", 0, KindParagraph, 1, 1},
	}
	for _, tc := range cases {
		if got := EffectiveScale(tc.css, tc.kind, tc.level); got != tc.want {
			t.Errorf("%s: EffectiveScale(%v, %v, %d) = %v, want %v", tc.name, tc.css, tc.kind, tc.level, got, tc.want)
		}
	}
}

func TestParseSizeKeepsTheRawScale(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"500%", 5},
		{"0.1em", 0.1},
		{"0%", 0.5},    // would otherwise read as "unset"
		{"-2em", 0.5},  // same
		{"nanem", 0.5}, // ParseFloat accepts "nan"; NaN must not read as unset
		{"150%", 1.5},
		{"24px", 1.5},
	}
	for _, tc := range cases {
		got, ok := parseSize(tc.in)
		if !ok || got != tc.want {
			t.Errorf("parseSize(%q) = (%v, %v), want %v", tc.in, got, ok, tc.want)
		}
	}
}
