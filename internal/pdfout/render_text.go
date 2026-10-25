package pdfout

import (
	"strings"

	"github.com/jparkerweb/azw3-to-pdf/internal/htmldoc"
)

// word is one measured, styled token of a line.
type word struct {
	text  string
	style htmldoc.Style
	link  string
	width float64
	space float64 // width of the space that follows
	brk   bool    // forced line break
}

// layout flows spans into lines and draws them.
func (r *renderer) layout(spans []htmldoc.Span, size float64, in indent, align htmldoc.Align, justify bool) {
	words := r.measure(spans, size)
	if len(words) == 0 {
		return
	}

	lineHeight := size * r.opts.LineSpacing
	availLeft := r.left + in.left
	availWidth := r.contentWidth - in.left - in.right
	if availWidth < size {
		availWidth = size
	}

	bulletWidth := 0.0
	if in.bullet != "" {
		r.setFontStyle(in.style, size)
		bulletWidth, _ = r.pdf.MeasureTextWidth(in.bullet)
	}

	var line []word
	lineWidth := 0.0
	first := true

	flush := func(forced bool) {
		if len(line) == 0 {
			if forced {
				r.space(lineHeight)
				r.y += lineHeight
				r.pageUsed = true
			}
			return
		}
		r.space(lineHeight)

		x := availLeft
		width := availWidth
		if first && in.firstLine != 0 {
			x += in.firstLine
			width -= in.firstLine
		}
		if in.bullet != "" {
			if first {
				r.setFontStyle(in.style, size)
				r.pdf.SetXY(x-bulletWidth, r.y+size*0.82)
				_ = r.pdf.Text(in.bullet)
			}
		}

		// Extra space distributed between words when justifying.
		extra := 0.0
		gaps := 0
		for i := range line {
			if i < len(line)-1 {
				gaps++
			}
		}
		switch {
		case justify && !forced && gaps > 0 && align == htmldoc.AlignDefault:
			extra = (width - lineWidth) / float64(gaps)
			if extra < 0 || extra > size*0.45 {
				extra = 0
			}
		case align == htmldoc.AlignCenter:
			x += (width - lineWidth) / 2
		case align == htmldoc.AlignRight:
			x += width - lineWidth
		}

		baseline := r.y + size*0.82
		for i, wd := range line {
			style := wd.style | in.style
			eff := r.setFontStyle(style, size)
			dy := 0.0
			if style.Has(htmldoc.StyleSuper) {
				dy = -size * 0.32
			} else if style.Has(htmldoc.StyleSub) {
				dy = size * 0.16
			}
			r.pdf.SetXY(x, baseline+dy)
			_ = r.pdf.Text(wd.text)
			if wd.link != "" {
				r.pdf.AddExternalLink(wd.link, x, baseline-eff*0.8, wd.width, eff*1.1)
			}
			x += wd.width
			if i < len(line)-1 {
				x += wd.space + extra
			}
		}

		r.y += lineHeight
		r.pageUsed = true
		line = line[:0]
		lineWidth = 0
		first = false
	}

	for _, wd := range words {
		if wd.brk {
			flush(true)
			continue
		}
		width := availWidth
		if first && in.firstLine != 0 {
			width -= in.firstLine
		}
		next := lineWidth
		if len(line) > 0 {
			next += line[len(line)-1].space
		}
		next += wd.width
		if len(line) > 0 && next > width {
			flush(false)
		}
		if len(line) > 0 {
			lineWidth += line[len(line)-1].space
		}
		lineWidth += wd.width
		line = append(line, wd)
	}
	flush(true)
}

// measure splits spans into words and records each word's rendered width.
func (r *renderer) measure(spans []htmldoc.Span, size float64) []word {
	var words []word
	for _, sp := range spans {
		if sp.Text == htmldoc.LineBreak {
			words = append(words, word{brk: true})
			continue
		}
		r.setFontStyle(sp.Style, size)
		spaceWidth, _ := r.pdf.MeasureTextWidth(" ")

		fields := splitKeepingSpaces(sp.Text)
		for _, f := range fields {
			if f.text == "" {
				continue
			}
			w, err := r.pdf.MeasureTextWidth(f.text)
			if err != nil {
				continue
			}
			// A space that fell between two differently styled runs belongs to
			// the word before it; without this, "<b>Note:</b> text" would run
			// together.
			if f.leadingSpace {
				if n := len(words); n > 0 && !words[n-1].brk && words[n-1].space == 0 {
					words[n-1].space = spaceWidth
				}
			}
			words = append(words, word{
				text:  f.text,
				style: sp.Style,
				link:  sp.Link,
				width: w,
				space: spaceWidth,
			})
			if !f.trailingSpace {
				words[len(words)-1].space = 0
			}
		}
	}
	// The final word never carries a trailing space.
	if n := len(words); n > 0 {
		words[n-1].space = 0
	}
	return words
}

type field struct {
	text          string
	leadingSpace  bool
	trailingSpace bool
}

// splitKeepingSpaces breaks text into words, remembering where the spaces were
// so that adjacent styled runs do not gain or lose a gap.
func splitKeepingSpaces(s string) []field {
	var out []field
	i := 0
	pendingSpace := false
	for i < len(s) {
		if s[i] == ' ' {
			for i < len(s) && s[i] == ' ' {
				i++
			}
			if len(out) > 0 {
				out[len(out)-1].trailingSpace = true
			} else {
				pendingSpace = true
			}
			continue
		}
		start := i
		for i < len(s) && s[i] != ' ' {
			i++
		}
		out = append(out, field{text: s[start:i], leadingSpace: pendingSpace})
		pendingSpace = false
	}
	return out
}

// setFontStyle activates the font for a style, degrading gracefully when a
// variant was not available. It returns the effective point size.
func (r *renderer) setFontStyle(s htmldoc.Style, size float64) float64 {
	family := familyBody
	if s.Has(htmldoc.StyleMono) {
		family = familyMono
	}
	eff := size
	if s.Has(htmldoc.StyleSmall) || s.Has(htmldoc.StyleSuper) || s.Has(htmldoc.StyleSub) {
		eff = size * 0.75
	}

	var style string
	if s.Has(htmldoc.StyleBold) {
		style += "B"
	}
	if s.Has(htmldoc.StyleItalic) {
		style += "I"
	}
	if s.Has(htmldoc.StyleUnderline) {
		style += "U"
	}

	for _, attempt := range []string{style, strings.TrimSuffix(style, "U"), "B", ""} {
		if err := r.pdf.SetFont(family, attempt, eff); err == nil {
			return eff
		}
	}
	_ = r.pdf.SetFont(familyBody, "", eff)
	return eff
}
