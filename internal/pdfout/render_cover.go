package pdfout

import (
	"strings"

	"github.com/jparkerweb/azw3-to-pdf/internal/htmldoc"
)

// coverPage draws the book's cover art as a full-bleed first page.
func (r *renderer) coverPage() {
	img, ok := r.imgs(coverIndex)
	if !ok || len(img.Data) == 0 {
		return
	}
	cfg, err := decodeConfig(img)
	if err != nil || cfg.Width == 0 || cfg.Height == 0 {
		return
	}

	r.newPage()
	pw, ph := r.opts.PageSize.Width, r.opts.PageSize.Height
	scale := pw / float64(cfg.Width)
	if s := ph / float64(cfg.Height); s < scale {
		scale = s
	}
	w := float64(cfg.Width) * scale
	h := float64(cfg.Height) * scale
	if err := r.place(img, (pw-w)/2, (ph-h)/2, w, h); err != nil {
		return
	}
	r.result.Images++
	r.pageUsed = true
}

// coverIndex is the pseudo resource index the caller maps to the cover image.
const coverIndex = -1

func (r *renderer) titlePage() {
	if r.meta.Title == "" && r.meta.Author == "" {
		return
	}
	r.newPage()
	r.y = r.top + r.contentHeight*0.28

	center := func(text string, size float64, style htmldoc.Style, gap float64) {
		if text == "" {
			return
		}
		spans := []htmldoc.Span{{Text: text, Style: style}}
		r.layout(spans, size, indent{}, htmldoc.AlignCenter, false)
		r.y += gap
	}

	center(r.meta.Title, r.opts.FontSize*2.0, htmldoc.StyleBold, r.opts.FontSize*1.6)
	center(r.meta.Author, r.opts.FontSize*1.15, htmldoc.StyleItalic, r.opts.FontSize*0.8)

	footer := strings.TrimSpace(strings.Join(nonEmpty(r.meta.Publisher, r.meta.Date), "  ·  "))
	if footer != "" {
		r.y = r.bottom - r.opts.FontSize*3
		center(footer, r.opts.FontSize*0.85, htmldoc.StyleSmall, 0)
	}
	r.pageUsed = true
}

func nonEmpty(values ...string) []string {
	var out []string
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, strings.TrimSpace(v))
		}
	}
	return out
}
