// Package pdfout lays a parsed book out as a PDF.
//
// This file holds the package's exported types and the Render entry point;
// the layout work itself lives in render_text.go, render_page.go,
// render_blocks.go, render_image.go and render_cover.go.
package pdfout

import (
	"context"
	"io"

	"github.com/signintech/gopdf"

	"github.com/jparkerweb/azw3-to-pdf/internal/htmldoc"
)

// Meta is the bibliographic information printed on the title page and stored
// in the PDF's document properties.
type Meta struct {
	Title     string
	Author    string
	Publisher string
	Date      string
	Subject   string
}

// Image is one embedded picture from the source book.
type Image struct {
	Data []byte
	Kind string // "jpeg", "png", "gif" or "bmp"
}

// Images resolves a 1-based resource index to a picture. Returning ok=false
// simply drops the image from the page.
type Images func(index int) (Image, bool)

// Progress reports layout progress.
type Progress struct {
	Block  int
	Blocks int
	Page   int
}

// Result summarises a finished conversion.
type Result struct {
	Pages    int
	Font     string
	Images   int
	Dropped  int
	Headings int
}

type renderer struct {
	pdf   *gopdf.GoPdf
	opts  Options
	meta  Meta
	fonts fontSet
	imgs  Images

	left, right   float64
	top, bottom   float64
	contentWidth  float64
	contentHeight float64

	y        float64
	page     int
	pageUsed bool

	result Result
}

// Render lays out doc and writes a PDF to w.
func Render(ctx context.Context, w io.Writer, doc *htmldoc.Doc, meta Meta, images Images, opts Options, onProgress func(Progress)) (Result, error) {
	opts.Normalize()

	r := &renderer{
		pdf:  &gopdf.GoPdf{},
		opts: opts,
		meta: meta,
		imgs: images,
	}
	if r.imgs == nil {
		r.imgs = func(int) (Image, bool) { return Image{}, false }
	}

	r.pdf.Start(gopdf.Config{Unit: gopdf.UnitPT, PageSize: gopdf.Rect{W: opts.PageSize.Width, H: opts.PageSize.Height}})
	r.pdf.SetCompressLevel(6)
	r.pdf.SetInfo(gopdf.PdfInfo{
		Title:    meta.Title,
		Author:   meta.Author,
		Subject:  meta.Subject,
		Creator:  "azw3-to-pdf",
		Producer: "azw3-to-pdf",
	})

	fonts, err := loadFonts(r.pdf, opts.Font)
	if err != nil {
		return r.result, err
	}
	r.fonts = fonts
	r.result.Font = fonts.Name()

	r.left = opts.Margins.Left
	r.right = opts.PageSize.Width - opts.Margins.Right
	r.top = opts.Margins.Top
	r.bottom = opts.PageSize.Height - opts.Margins.Bottom
	r.contentWidth = r.right - r.left
	r.contentHeight = r.bottom - r.top

	if opts.Cover {
		r.coverPage()
	}
	if opts.TitlePage {
		r.titlePage()
	}

	total := len(doc.Blocks)
	for i, b := range doc.Blocks {
		select {
		case <-ctx.Done():
			return r.result, ctx.Err()
		default:
		}
		r.block(b)
		if onProgress != nil && (i%25 == 0 || i == total-1) {
			onProgress(Progress{Block: i + 1, Blocks: total, Page: r.page})
		}
	}

	if r.page == 0 {
		// A book with nothing renderable still needs a valid PDF.
		r.newPage()
	}
	r.result.Pages = r.page

	if _, err := r.pdf.WriteTo(w); err != nil {
		return r.result, err
	}
	return r.result, nil
}
