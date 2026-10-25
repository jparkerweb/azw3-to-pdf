package pdfout

import (
	"fmt"
	"strings"

	"github.com/signintech/gopdf"

	"github.com/jparkerweb/ebook-to-pdf/internal/htmldoc"
)

// setGeometry makes w x h the size of the pages added from now on and lays
// the configured margins out inside it.
func (r *renderer) setGeometry(w, h float64) {
	r.pageW, r.pageH = w, h
	m := r.opts.Margins
	r.left = m.Left
	r.right = w - m.Right
	r.top = m.Top
	r.bottom = h - m.Bottom
	r.contentWidth = r.right - r.left
	r.contentHeight = r.bottom - r.top
}

// newPage starts a fresh page and draws its furniture.
func (r *renderer) newPage() {
	r.newBarePage()
	r.drawFurniture()
}

// newBarePage starts a fresh page of the current geometry without a header or
// page number. Every page records its own size, so one PDF can mix them.
func (r *renderer) newBarePage() {
	r.pdf.AddPageWithOption(gopdf.PageOption{PageSize: &gopdf.Rect{W: r.pageW, H: r.pageH}})
	r.page++
	r.pageUsed = false
	r.sizedPage = false
	r.furniturePending = false
	r.y = r.top
}

// ensurePage starts the first page lazily so that leading page breaks in the
// source do not produce blank pages. It is also the point every ordinary
// drawing path passes through, so a sized page gets its held-back furniture
// here.
func (r *renderer) ensurePage() {
	if r.page == 0 {
		r.newPage()
	}
	if r.furniturePending {
		r.furniturePending = false
		r.drawFurniture()
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
