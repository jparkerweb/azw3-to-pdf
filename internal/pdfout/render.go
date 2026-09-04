// Package pdfout lays a parsed book out as a PDF.
package pdfout

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"github.com/signintech/gopdf"
	"golang.org/x/image/bmp"

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

// heading sizes as a multiple of the body font size, indexed by level 1-6.
var headingScale = [7]float64{1, 1.75, 1.45, 1.28, 1.15, 1.06, 1.0}

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

func (r *renderer) block(b htmldoc.Block) {
	switch b.Kind {
	case htmldoc.KindPageBreak:
		if r.opts.ChapterBreaks && r.pageUsed {
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

// blockSize is the point size for a block, honouring the stylesheet's relative
// font size but keeping it within a readable range.
func (r *renderer) blockSize(b htmldoc.Block) float64 {
	if b.Scale <= 0 {
		return r.opts.FontSize
	}
	scale := b.Scale
	// Damp the extremes: a 290% heading at a 5.5in page width would not fit.
	if scale > 1 {
		scale = 1 + (scale-1)*0.62
	}
	size := r.opts.FontSize * scale
	if size < 5 {
		size = 5
	}
	if max := r.opts.FontSize * 2.4; size > max {
		size = max
	}
	return size
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
	level := b.Level
	if level < 1 || level > 6 {
		level = 3
	}
	size := r.opts.FontSize * headingScale[level]
	if css := r.blockSize(b); b.Scale > 0 && css > size {
		size = css
	}
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
			next = wd.width
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

func (r *renderer) image(b htmldoc.Block) {
	img, ok := r.imgs(b.Resource)
	if !ok || len(img.Data) == 0 {
		r.result.Dropped++
		return
	}
	cfg, err := decodeConfig(img)
	if err != nil {
		r.result.Dropped++
		return
	}

	// Never blow a small image up past its natural size at 96 dpi.
	natW := float64(cfg.Width) * 0.75
	natH := float64(cfg.Height) * 0.75
	scale := 1.0
	if natW > r.contentWidth {
		scale = r.contentWidth / natW
	}
	w := natW * scale
	h := natH * scale
	if h > r.contentHeight {
		s := r.contentHeight / h
		w *= s
		h *= s
	}

	r.ensurePage()
	if r.y+h > r.bottom {
		r.newPage()
	}

	x := r.left + (r.contentWidth-w)/2
	if err := r.place(img, x, r.y, w, h); err != nil {
		r.result.Dropped++
		return
	}
	r.result.Images++
	r.y += h + r.opts.FontSize*0.6
	r.pageUsed = true
}

// place draws an image, converting formats gopdf cannot embed directly.
func (r *renderer) place(img Image, x, y, w, h float64) error {
	rect := &gopdf.Rect{W: w, H: h}
	switch img.Kind {
	case "jpeg", "png":
		holder, err := gopdf.ImageHolderByBytes(img.Data)
		if err != nil {
			return err
		}
		return r.pdf.ImageByHolder(holder, x, y, rect)
	default:
		decoded, err := decodeImage(img)
		if err != nil {
			return err
		}
		return r.pdf.ImageFromWithOption(decoded, gopdf.ImageFromOption{
			Format: "png", X: x, Y: y, Rect: rect,
		})
	}
}

func decodeConfig(img Image) (image.Config, error) {
	r := bytes.NewReader(img.Data)
	switch img.Kind {
	case "jpeg":
		return jpeg.DecodeConfig(r)
	case "png":
		return png.DecodeConfig(r)
	case "gif":
		return gif.DecodeConfig(r)
	case "bmp":
		return bmp.DecodeConfig(r)
	}
	cfg, _, err := image.DecodeConfig(r)
	return cfg, err
}

func decodeImage(img Image) (image.Image, error) {
	r := bytes.NewReader(img.Data)
	switch img.Kind {
	case "gif":
		return gif.Decode(r)
	case "bmp":
		return bmp.Decode(r)
	}
	m, _, err := image.Decode(r)
	return m, err
}

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

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return strings.TrimSpace(string(runes[:n-1])) + "…"
}
