package pdfout

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// PageSize is a named page in PostScript points (72 per inch).
type PageSize struct {
	Name          string
	Label         string
	Width, Height float64
}

// Standard page sizes. The e-reader sizes are the ones that make a converted
// book comfortable to read on a tablet rather than to print.
var pageSizes = map[string]PageSize{
	"a4":      {"a4", "A4 (210 x 297 mm)", 595.28, 841.89},
	"a5":      {"a5", "A5 (148 x 210 mm)", 419.53, 595.28},
	"a6":      {"a6", "A6 (105 x 148 mm)", 297.64, 419.53},
	"letter":  {"letter", "US Letter (8.5 x 11 in)", 612, 792},
	"legal":   {"legal", "US Legal (8.5 x 14 in)", 612, 1008},
	"digest":  {"digest", "Digest (5.5 x 8.5 in)", 396, 612},
	"trade":   {"trade", "Trade paperback (6 x 9 in)", 432, 648},
	"kindle":  {"kindle", "Kindle Paperwhite (screen)", 353, 471},
	"tablet":  {"tablet", "Tablet (4:3 screen)", 540, 720},
	"pocket":  {"pocket", "Pocket (4.25 x 6.87 in)", 306, 495},
	"square":  {"square", "Square (8 x 8 in)", 576, 576},
	"tabloid": {"tabloid", "Tabloid (11 x 17 in)", 792, 1224},
}

// LookupPageSize resolves a page size by name, or by an explicit "WIDTHxHEIGHT"
// measurement in millimetres (e.g. "120x160mm") or inches ("6x9in").
func LookupPageSize(name string) (PageSize, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return pageSizes["a5"], nil
	}
	if ps, ok := pageSizes[key]; ok {
		return ps, nil
	}
	if ps, ok := parseCustomSize(key); ok {
		return ps, nil
	}
	return PageSize{}, fmt.Errorf("unknown page size %q (try one of: %s, or 120x160mm)", name, strings.Join(PageSizeNames(), ", "))
}

func parseCustomSize(s string) (PageSize, bool) {
	unit := 72.0 / 25.4 // millimetres by default
	switch {
	case strings.HasSuffix(s, "mm"):
		s = strings.TrimSuffix(s, "mm")
	case strings.HasSuffix(s, "in"):
		s = strings.TrimSuffix(s, "in")
		unit = 72
	case strings.HasSuffix(s, "pt"):
		s = strings.TrimSuffix(s, "pt")
		unit = 1
	}
	parts := strings.SplitN(s, "x", 2)
	if len(parts) != 2 {
		return PageSize{}, false
	}
	w, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	h, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return PageSize{}, false
	}
	return PageSize{Name: s, Label: "custom", Width: w * unit, Height: h * unit}, true
}

// PageSizeNames lists the built-in page sizes in a stable order.
func PageSizeNames() []string {
	names := make([]string, 0, len(pageSizes))
	for k := range pageSizes {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// PageSizes returns every built-in page size, sorted by name.
func PageSizes() []PageSize {
	out := make([]PageSize, 0, len(pageSizes))
	for _, name := range PageSizeNames() {
		out = append(out, pageSizes[name])
	}
	return out
}

// Margins are page margins in points.
type Margins struct {
	Top, Right, Bottom, Left float64
}

// UniformMargins builds equal margins from a millimetre value.
func UniformMargins(mm float64) Margins {
	pt := mm * 72 / 25.4
	return Margins{pt, pt, pt, pt}
}

// Options controls how a book is laid out.
type Options struct {
	PageSize PageSize
	Margins  Margins

	// Font is "serif", "sans", "mono" or a path to a .ttf file.
	Font        string
	FontSize    float64
	LineSpacing float64 // multiple of the font size
	Justify     bool

	ParagraphSpacing float64 // points between paragraphs
	FirstLineIndent  float64 // points

	Images        bool
	Cover         bool
	TitlePage     bool
	PageNumbers   bool
	RunningHeader bool
	Bookmarks     bool
	ChapterBreaks bool // honour the book's own page breaks
}

// DefaultOptions returns a sensible reading layout.
func DefaultOptions() Options {
	return Options{
		PageSize:         pageSizes["a5"],
		Margins:          UniformMargins(15),
		Font:             string(FontSerif),
		FontSize:         11,
		LineSpacing:      1.38,
		Justify:          true,
		ParagraphSpacing: 5,
		FirstLineIndent:  0,
		Images:           true,
		Cover:            true,
		TitlePage:        true,
		PageNumbers:      true,
		RunningHeader:    false,
		Bookmarks:        true,
		ChapterBreaks:    true,
	}
}

// Normalize clamps values that would produce an unreadable or broken page.
func (o *Options) Normalize() {
	if o.PageSize.Width <= 0 || o.PageSize.Height <= 0 {
		o.PageSize = pageSizes["a5"]
	}
	if o.FontSize < 5 {
		o.FontSize = 5
	}
	if o.FontSize > 40 {
		o.FontSize = 40
	}
	if o.LineSpacing < 1 {
		o.LineSpacing = 1
	}
	if o.LineSpacing > 3 {
		o.LineSpacing = 3
	}
	if o.Font == "" {
		o.Font = string(FontSerif)
	}

	// Leave room for at least a few characters per line.
	maxSide := o.PageSize.Width/2 - 20
	for _, m := range []*float64{&o.Margins.Left, &o.Margins.Right} {
		if *m < 0 {
			*m = 0
		}
		if *m > maxSide {
			*m = maxSide
		}
	}
	maxVert := o.PageSize.Height/2 - 20
	for _, m := range []*float64{&o.Margins.Top, &o.Margins.Bottom} {
		if *m < 0 {
			*m = 0
		}
		if *m > maxVert {
			*m = maxVert
		}
	}
}
