package pdfout

import (
	"strings"

	"github.com/jparkerweb/ebook-to-pdf/internal/htmldoc"
)

func (r *renderer) block(b htmldoc.Block) {
	switch b.Kind {
	case htmldoc.KindPageBreak:
		if b.PageWidth > 0 {
			r.sizedBreak(b.PageWidth, b.PageHeight)
		} else if r.opts.ChapterBreaks && r.pageUsed && r.y < r.bottom {
			// A full page (a picture given a page of its own) leaves the
			// cursor at the bottom, so whatever follows starts a new page
			// anyway; breaking here too would leave a blank page between
			// every page of a comic.
			r.newPage()
		}
	case htmldoc.KindRule:
		r.rule()
	case htmldoc.KindImage:
		if r.opts.Images {
			r.image(b)
		}
	case htmldoc.KindHeading:
		r.heading(b)
	case htmldoc.KindQuote:
		size := r.blockSize(b)
		r.paragraph(b, size, indent{left: size * 1.5, right: size * 1.5, style: htmldoc.StyleItalic})
	case htmldoc.KindListItem:
		depth := b.Level
		if depth < 1 {
			depth = 1
		}
		size := r.blockSize(b)
		r.paragraph(b, size, indent{left: float64(depth) * size * 1.4, bullet: "• "})
	default:
		size := r.blockSize(b)
		in := indent{
			left:      clampIndent(b.MarginLeft*size, r.contentWidth),
			firstLine: b.Indent * size,
		}
		if in.firstLine == 0 {
			in.firstLine = r.opts.FirstLineIndent
		}
		r.paragraph(b, size, in)
	}
}

// sizedBreak starts a w x h page whatever ChapterBreaks says, since a fixed
// layout book places its content by page. An unused page of that size is
// reused so that consecutive breaks do not leave blank pages behind.
func (r *renderer) sizedBreak(w, h float64) {
	if r.page > 0 && !r.pageUsed && r.pageW == w && r.pageH == h {
		r.sizedPage = true
		return
	}
	r.setGeometry(w, h)
	r.newBarePage()
	r.sizedPage = true
	// The page may turn out to hold only a full-page image, which must not
	// carry furniture, so drawing it waits for the first ordinary content.
	r.furniturePending = true
}

// blockSize is the point size for a block: the body size scaled by the
// stylesheet and block kind (htmldoc.EffectiveScale), never below a readable
// minimum.
func (r *renderer) blockSize(b htmldoc.Block) float64 {
	return max(r.opts.FontSize*htmldoc.EffectiveScale(b.Scale, b.Kind, b.Level), 5)
}

// clampIndent keeps a stylesheet indent from swallowing the text column.
func clampIndent(v, width float64) float64 {
	if v < 0 {
		return 0
	}
	if limit := width * 0.4; v > limit {
		return limit
	}
	return v
}

type indent struct {
	left      float64
	right     float64
	firstLine float64
	bullet    string
	style     htmldoc.Style
}

func (r *renderer) heading(b htmldoc.Block) {
	size := r.blockSize(b)
	above := size * 0.9
	below := size * 0.35

	r.ensurePage()
	if r.pageUsed {
		r.y += above
	}

	// Keep a heading with at least two lines of the text that follows it.
	needed := size*r.opts.LineSpacing + below + r.opts.FontSize*r.opts.LineSpacing*2
	if r.y+needed > r.bottom {
		r.newPage()
	}

	if r.opts.Bookmarks {
		if text := strings.TrimSpace(b.Text()); text != "" {
			r.pdf.SetXY(r.left, r.y)
			r.pdf.AddOutline(truncate(text, 120))
			r.result.Headings++
		}
	}

	align := b.Align
	if align == htmldoc.AlignDefault {
		align = htmldoc.AlignLeft
	}
	r.layout(b.Spans, size, indent{style: htmldoc.StyleBold}, align, false)
	r.y += below
}

func (r *renderer) paragraph(b htmldoc.Block, size float64, in indent) {
	align := b.Align
	if align == htmldoc.AlignDefault {
		if r.opts.Justify {
			align = htmldoc.AlignDefault // justified
		} else {
			align = htmldoc.AlignLeft
		}
	}
	r.layout(b.Spans, size, in, align, r.opts.Justify)
	r.y += r.opts.ParagraphSpacing
}

func (r *renderer) rule() {
	r.space(r.opts.FontSize * 1.4)
	y := r.y + r.opts.FontSize*0.6
	r.pdf.SetLineWidth(0.5)
	r.pdf.SetStrokeColor(170, 170, 170)
	r.pdf.Line(r.left+r.contentWidth*0.25, y, r.right-r.contentWidth*0.25, y)
	r.pdf.SetStrokeColor(0, 0, 0)
	r.y += r.opts.FontSize * 1.4
	r.pageUsed = true
}
