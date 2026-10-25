package ebook

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif" // registered for image.DecodeConfig
	_ "image/jpeg"
	_ "image/png"
	"strconv"
	"strings"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/webp"
	"golang.org/x/net/html"
)

// pxToPt converts CSS pixels, which fixed-layout books are sized in, to PDF
// points.
const pxToPt = 0.75

// defaultFixedPage is the size, in pixels, of a fixed page that gives no
// size anywhere and has no neighbour to borrow one from.
var defaultFixedPage = pageSize{600, 800}

type pageSize struct{ w, h float64 }

func (s pageSize) ok() bool { return s.w > 0 && s.h > 0 }

// prePaginated reports the book-level layout: EPUB 3's rendition:layout, or
// for EPUB 2 Apple's display-options file.
func (e *epubReader) prePaginated() bool {
	for _, m := range e.pkg.Metadata.Metas {
		if m.Property == "rendition:layout" {
			return strings.TrimSpace(m.Value) == "pre-paginated"
		}
	}
	const apple = "META-INF/com.apple.ibooks.display-options.xml"
	if !e.zip.has(apple) {
		return false
	}
	raw, err := e.zip.read(apple)
	if err != nil {
		return false
	}
	var opts struct {
		Options []struct {
			Name  string `xml:"name,attr"`
			Value string `xml:",chardata"`
		} `xml:"platform>option"`
	}
	if decodeXML(raw, &opts) != nil {
		return false
	}
	for _, o := range opts.Options {
		if o.Name == "fixed-layout" && strings.TrimSpace(o.Value) == "true" {
			return true
		}
	}
	return false
}

// pageIsFixed applies an itemref's own layout override to the book setting.
func (e *epubReader) pageIsFixed(ref opfItemref) bool {
	switch {
	case hasProperty(ref.Properties, "rendition:layout-pre-paginated"):
		return true
	case hasProperty(ref.Properties, "rendition:layout-reflowable"):
		return false
	}
	return e.paged
}

// pageSizes works out every fixed page's size in points, indexed like docs.
// A page that declares nothing borrows the previous page's size, or failing
// that the first size found later in the spine.
func (e *epubReader) pageSizes(docs []spineDoc) []pageSize {
	sizes := make([]pageSize, len(docs))
	for i, d := range docs {
		if d.fixed {
			sizes[i] = e.ownPageSize(d)
		}
	}
	var prev pageSize
	for i, d := range docs {
		if !d.fixed {
			continue
		}
		if !sizes[i].ok() {
			sizes[i] = prev
		}
		if !sizes[i].ok() {
			for j := i + 1; j < len(docs); j++ {
				if docs[j].fixed && sizes[j].ok() {
					sizes[i] = sizes[j]
					break
				}
			}
		}
		if !sizes[i].ok() {
			sizes[i] = defaultFixedPage
		}
		prev = sizes[i]
	}
	for i := range sizes {
		sizes[i].w *= pxToPt
		sizes[i].h *= pxToPt
	}
	return sizes
}

// ownPageSize is the size, in pixels, a fixed page declares or implies by
// itself, or a zero size.
func (e *epubReader) ownPageSize(d spineDoc) pageSize {
	var viewport pageSize
	walk(d.root, func(n *html.Node) bool {
		if n.Type == html.ElementNode && n.Data == "meta" && strings.EqualFold(attr(n, "name"), "viewport") {
			if s := parseViewport(attr(n, "content")); s.ok() {
				viewport = s
				return false
			}
		}
		return !viewport.ok()
	})
	if viewport.ok() {
		return viewport
	}
	if svg := onlyChild(body(d.root)); svg != nil && svg.Data == "svg" {
		if s := svgSize(svg); s.ok() {
			return s
		}
	}
	for _, m := range e.pkg.Metadata.Metas {
		if m.Property == "rendition:viewport" {
			if s := parseViewport(m.Value); s.ok() {
				return s
			}
		}
	}
	if refs := e.pageImages(d); len(refs) == 1 {
		if r := e.resources[refs[0]]; r != nil {
			return imagePixels(r)
		}
	}
	return pageSize{}
}

// parseViewport reads "width=1200, height=1600", tolerating missing spaces,
// semicolons and a px suffix.
func parseViewport(content string) pageSize {
	var s pageSize
	for _, part := range strings.FieldsFunc(content, func(r rune) bool { return r == ',' || r == ';' }) {
		key, val, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		n := pixels(val)
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "width":
			s.w = n
		case "height":
			s.h = n
		}
	}
	return s
}

// pixels reads a length such as "1200" or "1200px"; anything else is 0.
func pixels(v string) float64 {
	v = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(v)), "px")
	f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
	if err != nil || f <= 0 || f > 1e6 {
		return 0
	}
	return f
}

// svgSize is an <svg>'s width and height, else its viewBox.
func svgSize(svg *html.Node) pageSize {
	if s := (pageSize{pixels(attr(svg, "width")), pixels(attr(svg, "height"))}); s.ok() {
		return s
	}
	// html.Parse keeps SVG attribute names in their SVG case.
	box := attr(svg, "viewBox")
	if box == "" {
		box = attr(svg, "viewbox")
	}
	f := strings.FieldsFunc(box, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' || r == '\n' })
	if len(f) == 4 {
		return pageSize{pixels(f[2]), pixels(f[3])}
	}
	return pageSize{}
}

// onlyChild returns n's single element child when n holds nothing else but
// whitespace.
func onlyChild(n *html.Node) *html.Node {
	var only *html.Node
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch c.Type {
		case html.ElementNode:
			if only != nil {
				return nil
			}
			only = c
		case html.TextNode:
			if strings.TrimSpace(c.Data) != "" {
				return nil
			}
		}
	}
	return only
}

// imagePixels reads an image's size from its header.
func imagePixels(r *Resource) pageSize {
	var cfg image.Config
	var err error
	if r.Kind == "webp" {
		cfg, err = webp.DecodeConfig(bytes.NewReader(r.Data))
	} else {
		cfg, _, err = image.DecodeConfig(bytes.NewReader(r.Data))
	}
	if err != nil {
		return pageSize{}
	}
	return pageSize{float64(cfg.Width), float64(cfg.Height)}
}

// pageImages lists the resource numbers of the images a page body shows,
// <img> and SVG <image> alike, before any rewriting.
func (e *epubReader) pageImages(d spineDoc) []int {
	var refs []int
	walk(body(d.root), func(n *html.Node) bool {
		if ref := imageRef(n); ref != "" {
			refs = append(refs, e.images[e.zip.resolve(d.path, ref)])
		}
		return true
	})
	return refs
}

// fixedPage emits one pre-paginated spine item as one PDF page of the given
// size in points. A page that is a single picture becomes that picture
// fitted to the page; anything else has its content reflowed onto a page of
// that size, since absolute positioning is not reproduced.
func (e *epubReader) fixedPage(d spineDoc, size pageSize) string {
	marker := fmt.Sprintf(`<ebook:page width="%s" height="%s"></ebook:page>`, points(size.w), points(size.h))
	b := body(d.root)
	if refs := e.pageImages(d); len(refs) == 1 && refs[0] > 0 && !hasText(b) {
		return fmt.Sprintf(`<html><body>%s<img recindex="%d" data-ebook-fullpage="1"></body></html>`, marker, refs[0])
	}
	e.rewriteImages(d)
	var buf bytes.Buffer
	buf.WriteString("<html><body>")
	buf.WriteString(marker)
	for c := b.FirstChild; c != nil; c = c.NextSibling {
		_ = html.Render(&buf, c)
	}
	buf.WriteString("</body></html>")
	return buf.String()
}

func points(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
