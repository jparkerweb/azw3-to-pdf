// Package ebook reads ebooks of several formats (Kindle, EPUB, comic
// archives, FictionBook, plain text and HTML) into one Book: metadata, markup
// in reading order and numbered embedded resources. Everything after this
// package is format-agnostic.
package ebook

import "strings"

// Book is a decoded ebook, whatever format it came from.
type Book struct {
	Path string

	// Bibliographic metadata, all optional.
	Title       string
	Authors     []string
	Publisher   string
	Description string
	ISBN        string
	Published   string
	Rights      string
	Language    string
	Subjects    []string
	ASIN        string

	// Format details, surfaced by the info screen and `probe`. Format names
	// the reader, for example "KF8 / AZW3 (MOBI v8)", "EPUB 3",
	// "EPUB 2 (fixed layout)", "Comic archive (CBZ)", "FictionBook 2",
	// "Plain text" or "HTML". Compression and Encoding are often empty for
	// the formats that are not Kindle's.
	Format      string
	Compression string
	Encoding    string
	FileSize    int64
	TextBytes   int

	// HTML is the book's markup in reading order. Flows holds the auxiliary
	// KF8 streams (stylesheets and SVG), which the renderer does not lay out
	// but which are useful for diagnostics.
	HTML  string
	Flows []string

	// Resources are the embedded files, indexed the way the markup addresses
	// them: 1-based from the first resource record.
	Resources map[int]*Resource
	Cover     *Resource

	// Paged marks a pre-paginated book (comics, fixed-layout EPUB): each
	// source page becomes exactly one PDF page, and no cover or title page
	// is added because the book's own first page already is its cover.
	Paged bool
}

// Resource is one embedded file, almost always an image.
type Resource struct {
	Index int
	Kind  string // "jpeg", "png", "gif", "bmp", "webp" or "" when unrecognised
	Data  []byte
}

// IsImage reports whether the resource is a decodable raster image.
func (r *Resource) IsImage() bool {
	return r != nil && r.Kind != ""
}

func sniffImage(rec []byte) string {
	if len(rec) < 8 {
		return ""
	}
	switch {
	case rec[0] == 0xff && rec[1] == 0xd8 && rec[2] == 0xff:
		return "jpeg"
	case string(rec[1:4]) == "PNG":
		return "png"
	case string(rec[0:3]) == "GIF":
		return "gif"
	case rec[0] == 'B' && rec[1] == 'M':
		return "bmp"
	case len(rec) >= 12 && string(rec[0:4]) == "RIFF" && string(rec[8:12]) == "WEBP":
		return "webp"
	}
	return ""
}

// AuthorLine joins the authors for display.
func (b *Book) AuthorLine() string {
	if len(b.Authors) == 0 {
		return "Unknown author"
	}
	return strings.Join(b.Authors, ", ")
}
