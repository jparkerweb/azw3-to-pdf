// Package presets holds the named page layouts offered by the CLI and TUI.
package presets

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jparkerweb/ebook-to-pdf/internal/pdfout"
)

// Preset is a named starting point for a PDF layout.
type Preset struct {
	Key         string
	Name        string
	Description string

	PageSize    string
	MarginMM    float64
	Font        string
	FontSize    float64
	LineSpacing float64
	Justify     bool

	// Extras a preset may turn off.
	TitlePage     bool
	PageNumbers   bool
	RunningHeader bool

	// SizePreset marks a preset whose page size is part of its identity, so it
	// wins over the page size the user saved as their default.
	SizePreset bool
}

// DefaultPageSize is the page size used when the user has not saved one.
const DefaultPageSize = "letter"

// registry is the built-in preset list, in the order they are shown.
var registry = []Preset{
	{
		Key:         "ereader",
		Name:        "E-reader",
		Description: "Tight margins and justified serif type. The best all-round reading layout.",
		PageSize:    "a5", MarginMM: 14, Font: "serif", FontSize: 11, LineSpacing: 1.38,
		Justify: true, TitlePage: true, PageNumbers: true,
	},
	{
		Key:         "paperback",
		Name:        "Paperback",
		Description: "Trade paperback proportions with book margins. Looks like a printed novel.",
		PageSize:    "trade", MarginMM: 18, Font: "serif", FontSize: 11.5, LineSpacing: 1.42,
		Justify: true, TitlePage: true, PageNumbers: true, RunningHeader: true,
		SizePreset: true,
	},
	{
		Key:         "print",
		Name:        "Print",
		Description: "Generous margins and a running header, for printing and annotating.",
		PageSize:    "letter", MarginMM: 22, Font: "serif", FontSize: 11.5, LineSpacing: 1.45,
		Justify: true, TitlePage: true, PageNumbers: true, RunningHeader: true,
	},
	{
		Key:         "a4",
		Name:        "A4",
		Description: "A4 with standard margins, for everywhere that is not the US.",
		PageSize:    "a4", MarginMM: 20, Font: "serif", FontSize: 11.5, LineSpacing: 1.45,
		Justify: true, TitlePage: true, PageNumbers: true, RunningHeader: true,
		SizePreset: true,
	},
	{
		Key:         "compact",
		Name:        "Compact",
		Description: "Small type and thin margins. Roughly a third fewer pages.",
		PageSize:    "a5", MarginMM: 9, Font: "serif", FontSize: 9.5, LineSpacing: 1.22,
		Justify: true, TitlePage: false, PageNumbers: true,
	},
	{
		Key:         "large-print",
		Name:        "Large print",
		Description: "Big type and open leading for comfortable reading.",
		PageSize:    "a4", MarginMM: 18, Font: "sans", FontSize: 16, LineSpacing: 1.6,
		Justify: false, TitlePage: true, PageNumbers: true,
	},
	{
		Key:         "phone",
		Name:        "Phone",
		Description: "A narrow page that fills a phone screen without pinch-zooming.",
		PageSize:    "kindle", MarginMM: 7, Font: "sans", FontSize: 10, LineSpacing: 1.35,
		Justify: false, TitlePage: false, PageNumbers: false,
		SizePreset: true,
	},
	{
		Key:         "manuscript",
		Name:        "Manuscript",
		Description: "Double-spaced pages for editing and mark-up.",
		PageSize:    "letter", MarginMM: 25, Font: "sans", FontSize: 12, LineSpacing: 2.0,
		Justify: false, TitlePage: true, PageNumbers: true,
	},
}

// All returns every preset in display order.
func All() []Preset {
	out := make([]Preset, len(registry))
	copy(out, registry)
	return out
}

// Default is the preset used when nothing else is chosen.
func Default() Preset {
	p, _ := Lookup("ereader")
	return p
}

// Lookup finds a preset by key, case-insensitively.
func Lookup(key string) (Preset, bool) {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, p := range registry {
		if p.Key == key {
			return p, true
		}
	}
	return Preset{}, false
}

// Keys lists the preset keys in display order.
func Keys() []string {
	keys := make([]string, 0, len(registry))
	for _, p := range registry {
		keys = append(keys, p.Key)
	}
	return keys
}

// PageSizeFor returns the page size this preset renders with, given the page
// size the user saved as their default (empty when none is saved).
func (p Preset) PageSizeFor(saved string) string {
	switch {
	case p.SizePreset:
		return p.PageSize
	case saved != "":
		return saved
	default:
		return DefaultPageSize
	}
}

// Options converts a preset into renderer options, with no saved page size.
func (p Preset) Options() (pdfout.Options, error) {
	return p.OptionsWithPageSize("")
}

// OptionsWithPageSize converts a preset into renderer options, using the saved
// default page size unless the preset is a size preset.
func (p Preset) OptionsWithPageSize(saved string) (pdfout.Options, error) {
	size, err := pdfout.LookupPageSize(p.PageSizeFor(saved))
	if err != nil {
		return pdfout.Options{}, err
	}
	opts := pdfout.DefaultOptions().ApplyOverrides(pdfout.OptionsOverrides{
		PageSize:      &size,
		Margins:       pdfout.Ptr(pdfout.UniformMargins(p.MarginMM)),
		Font:          &p.Font,
		FontSize:      &p.FontSize,
		LineSpacing:   &p.LineSpacing,
		Justify:       &p.Justify,
		TitlePage:     &p.TitlePage,
		PageNumbers:   &p.PageNumbers,
		RunningHeader: &p.RunningHeader,
	})
	opts.Normalize()
	return opts, nil
}

// Summary is a one-line description of the layout a preset produces when no
// page size is saved.
func (p Preset) Summary() string {
	return p.SummaryFor("")
}

// SummaryFor is Summary with the page size the user saved as their default.
func (p Preset) SummaryFor(saved string) string {
	name := p.PageSizeFor(saved)
	size, err := pdfout.LookupPageSize(name)
	label := name
	if err == nil {
		label = size.Label
	}
	justify := "ragged right"
	if p.Justify {
		justify = "justified"
	}
	return fmt.Sprintf("%s · %s %.1fpt · %.0fmm margins · %s", label, p.Font, p.FontSize, p.MarginMM, justify)
}

// Book describes the characteristics a recommendation is based on.
type Book struct {
	Images     int
	TextBytes  int
	HasCover   bool
	AvgLineLen int
}

// Recommend suggests a preset for a book. Heavily illustrated books want a
// larger page so pictures are not shrunk to postage stamps; very long books
// benefit from the compact layout.
func Recommend(b Book) Preset {
	switch {
	case b.Images >= 40:
		p, _ := Lookup("print")
		return p
	case b.TextBytes > 1_500_000:
		p, _ := Lookup("compact")
		return p
	case b.Images >= 8:
		p, _ := Lookup("paperback")
		return p
	default:
		return Default()
	}
}

// Sorted returns presets ordered by key, which the shell completions use.
func Sorted() []Preset {
	out := All()
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}
