package pdfout

import (
	"bytes"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"

	"github.com/signintech/gopdf"
	"golang.org/x/image/bmp"

	"github.com/jparkerweb/azw3-to-pdf/internal/htmldoc"
)

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
