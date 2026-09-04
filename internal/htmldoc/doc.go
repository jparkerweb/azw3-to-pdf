// Package htmldoc turns the markup found inside a Kindle book into a flat
// sequence of layout blocks. It is deliberately not a browser: it keeps the
// structure a book actually needs (paragraphs, headings, lists, images, page
// breaks) and discards the rest.
package htmldoc

// Style is a set of inline text attributes.
type Style uint8

const (
	StyleBold Style = 1 << iota
	StyleItalic
	StyleMono
	StyleUnderline
	StyleSmall
	StyleSuper
	StyleSub
)

// Has reports whether every attribute in s is set.
func (s Style) Has(other Style) bool { return s&other == other }

// Kind identifies what a block is.
type Kind int

const (
	KindParagraph Kind = iota
	KindHeading
	KindListItem
	KindQuote
	KindRule
	KindImage
	KindPageBreak
)

// Align is a block's horizontal alignment.
type Align int

const (
	AlignDefault Align = iota
	AlignLeft
	AlignCenter
	AlignRight
)

// Span is a run of text sharing one style. A span whose Text is "\n" is an
// explicit line break rather than content.
type Span struct {
	Text  string
	Style Style
	Link  string
}

// LineBreak is the sentinel span text used for <br>.
const LineBreak = "\n"

// Block is one laid-out unit: a paragraph, a heading, an image, and so on.
type Block struct {
	Kind  Kind
	Level int // heading level (1-6) or list nesting depth
	Align Align
	Spans []Span

	// Resource is the 1-based index of the embedded image for KindImage.
	Resource int
	Alt      string

	// Scale multiplies the body font size for this block (0 means "unset").
	// MarginLeft and Indent are in em, and Indent may be negative to produce
	// a hanging indent.
	Scale      float64
	MarginLeft float64
	Indent     float64
}

// Text returns the block's plain text, which the outline and diagnostics use.
func (b Block) Text() string {
	out := make([]byte, 0, 64)
	for _, s := range b.Spans {
		if s.Text == LineBreak {
			out = append(out, ' ')
			continue
		}
		out = append(out, s.Text...)
	}
	return string(out)
}

// IsEmpty reports whether a block would render nothing at all.
func (b Block) IsEmpty() bool {
	switch b.Kind {
	case KindRule, KindPageBreak, KindImage:
		return false
	}
	for _, s := range b.Spans {
		for _, r := range s.Text {
			switch r {
			case ' ', '\n', '\t', nbsp:
			default:
				return false
			}
		}
	}
	return true
}

// Doc is a whole book flattened into blocks.
type Doc struct {
	Blocks []Block
}

// Headings returns the block indices that carry headings, in order.
func (d *Doc) Headings() []int {
	var out []int
	for i, b := range d.Blocks {
		if b.Kind == KindHeading && !b.IsEmpty() {
			out = append(out, i)
		}
	}
	return out
}
