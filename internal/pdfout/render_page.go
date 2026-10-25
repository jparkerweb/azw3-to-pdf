package pdfout

import (
	"fmt"
	"strings"

	"github.com/jparkerweb/azw3-to-pdf/internal/htmldoc"
)

// newPage starts a fresh page and draws its furniture.
func (r *renderer) newPage() {
	r.pdf.AddPage()
	r.page++
	r.pageUsed = false
	r.y = r.top
	r.drawFurniture()
}

// ensurePage starts the first page lazily so that leading page breaks in the
// source do not produce blank pages.
func (r *renderer) ensurePage() {
	if r.page == 0 {
		r.newPage()
	}
}

func (r *renderer) drawFurniture() {
	if r.opts.RunningHeader && r.meta.Title != "" && r.page > 1 {
		r.setFontStyle(htmldoc.StyleItalic, r.opts.FontSize*0.72)
		r.pdf.SetTextColor(130, 130, 130)
		title := truncate(r.meta.Title, 70)
		w, _ := r.pdf.MeasureTextWidth(title)
		r.pdf.SetXY(r.left+(r.contentWidth-w)/2, r.top-r.opts.FontSize*0.9)
		_ = r.pdf.Text(title)
		r.pdf.SetTextColor(0, 0, 0)
	}
	if r.opts.PageNumbers && r.page > 0 {
		r.setFontStyle(0, r.opts.FontSize*0.78)
		r.pdf.SetTextColor(130, 130, 130)
		label := fmt.Sprintf("%d", r.page)
		w, _ := r.pdf.MeasureTextWidth(label)
		r.pdf.SetXY(r.left+(r.contentWidth-w)/2, r.bottom+r.opts.FontSize*1.5)
		_ = r.pdf.Text(label)
		r.pdf.SetTextColor(0, 0, 0)
	}
}

// space reserves height, starting a new page when the block will not fit.
func (r *renderer) space(h float64) {
	r.ensurePage()
	if r.y+h > r.bottom {
		r.newPage()
	}
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return strings.TrimSpace(string(runes[:n-1])) + "…"
}
