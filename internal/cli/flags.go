package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jparkerweb/azw3-to-pdf/internal/config"
	"github.com/jparkerweb/azw3-to-pdf/internal/engine"
	"github.com/jparkerweb/azw3-to-pdf/internal/pdfout"
	"github.com/jparkerweb/azw3-to-pdf/internal/presets"
)

// Conversion flags.
var (
	flagInput  string
	flagOutput string
	flagDir    string
	flagSuffix string
	flagPreset string

	flagPageSize    string
	flagMargin      float64
	flagFont        string
	flagFontSize    float64
	flagLineSpacing float64

	flagJustify       bool
	flagNoJustify     bool
	flagNoImages      bool
	flagNoCover       bool
	flagNoTitlePage   bool
	flagNoPageNumbers bool
	flagRunningHeader bool
	flagNoBookmarks   bool
	flagNoBreaks      bool

	flagOverwrite    bool
	flagAutoRename   bool
	flagSkipExisting bool

	flagRecursive bool
	flagJobs      int
	flagNoTUI     bool
	flagDryRun    bool
	flagOpen      bool
	flagStdin     bool
)

func registerRunFlags(cmd *cobra.Command) {
	f := cmd.Flags()

	f.StringVarP(&flagInput, "input", "i", "", "book to convert (also accepted as a positional argument)")
	f.StringVarP(&flagOutput, "output", "o", "", "output PDF path (single input only)")
	f.StringVar(&flagDir, "output-dir", "", "directory to write PDFs into")
	f.StringVar(&flagSuffix, "suffix", "", "suffix added to the output file name")
	f.StringVarP(&flagPreset, "preset", "p", "", "layout preset ("+strings.Join(presets.Keys(), ", ")+")")

	f.StringVar(&flagPageSize, "page-size", "", "page size name or measurement, e.g. a5, letter, 120x160mm")
	f.Float64Var(&flagMargin, "margin", 0, "page margin in millimetres")
	f.StringVar(&flagFont, "font", "", "body typeface: serif, sans, mono, or a path to a .ttf file")
	f.Float64Var(&flagFontSize, "font-size", 0, "body font size in points")
	f.Float64Var(&flagLineSpacing, "line-spacing", 0, "line spacing as a multiple of the font size")

	f.BoolVar(&flagJustify, "justify", false, "justify body text")
	f.BoolVar(&flagNoJustify, "no-justify", false, "leave body text ragged right")
	f.BoolVar(&flagNoImages, "no-images", false, "leave the book's illustrations out")
	f.BoolVar(&flagNoCover, "no-cover", false, "skip the cover page")
	f.BoolVar(&flagNoTitlePage, "no-title-page", false, "skip the generated title page")
	f.BoolVar(&flagNoPageNumbers, "no-page-numbers", false, "omit page numbers")
	f.BoolVar(&flagRunningHeader, "running-header", false, "print the book title at the top of each page")
	f.BoolVar(&flagNoBookmarks, "no-bookmarks", false, "do not build the PDF outline from headings")
	f.BoolVar(&flagNoBreaks, "no-chapter-breaks", false, "ignore the book's own page breaks and run the text on")

	f.BoolVar(&flagOverwrite, "overwrite", false, "replace an existing output file")
	f.BoolVar(&flagAutoRename, "auto-rename", false, "write to the next free file name instead of failing")
	f.BoolVar(&flagSkipExisting, "skip-existing", false, "leave books whose PDF already exists alone")

	f.BoolVarP(&flagRecursive, "recursive", "r", false, "search directories for books recursively")
	f.IntVarP(&flagJobs, "jobs", "j", 1, "number of books to convert in parallel")
	f.BoolVar(&flagNoTUI, "no-tui", false, "convert from the shell instead of opening the interface")
	f.BoolVar(&flagDryRun, "dry-run", false, "report what would be written without writing it")
	f.BoolVar(&flagOpen, "open", false, "open the output folder when finished")
	f.BoolVar(&flagStdin, "stdin", false, "read book paths from standard input, one per line")
}

// validateFlags rejects combinations that contradict each other.
func validateFlags(cmd *cobra.Command) error {
	conflict := 0
	for _, name := range []string{"overwrite", "auto-rename", "skip-existing"} {
		if cmd.Flags().Changed(name) {
			conflict++
		}
	}
	if conflict > 1 {
		return fmt.Errorf("--overwrite, --auto-rename and --skip-existing are mutually exclusive")
	}
	if cmd.Flags().Changed("justify") && cmd.Flags().Changed("no-justify") {
		return fmt.Errorf("--justify and --no-justify are mutually exclusive")
	}
	if flagPreset != "" {
		if _, ok := presets.Lookup(flagPreset); !ok {
			return fmt.Errorf("unknown preset %q (available: %s)", flagPreset, strings.Join(presets.Keys(), ", "))
		}
	}
	if flagPageSize != "" {
		if _, err := pdfout.LookupPageSize(flagPageSize); err != nil {
			return err
		}
	}
	if cmd.Flags().Changed("jobs") && flagJobs < 1 {
		return fmt.Errorf("--jobs must be at least 1")
	}
	if cmd.Flags().Changed("font-size") && (flagFontSize < 5 || flagFontSize > 40) {
		return fmt.Errorf("--font-size must be between 5 and 40 points")
	}
	if cmd.Flags().Changed("margin") && (flagMargin < 0 || flagMargin > 100) {
		return fmt.Errorf("--margin must be between 0 and 100 millimetres")
	}
	return nil
}

// resolvePreset picks the preset named on the command line, in the config, or
// the built-in default.
func resolvePreset(cfg *config.Config) presets.Preset {
	if flagPreset != "" {
		if p, ok := presets.Lookup(flagPreset); ok {
			return p
		}
	}
	if cfg != nil && cfg.Preset != "" {
		if p, ok := presets.Lookup(cfg.Preset); ok {
			return p
		}
	}
	return presets.Default()
}

// buildPDFOptions layers the config file and then the command line on top of
// the chosen preset.
func buildPDFOptions(cmd *cobra.Command, cfg *config.Config, preset presets.Preset) (pdfout.Options, error) {
	opts, err := preset.Options()
	if err != nil {
		return opts, err
	}

	if cfg != nil {
		l := cfg.Layout
		if l.PageSize != "" {
			size, err := pdfout.LookupPageSize(l.PageSize)
			if err != nil {
				return opts, err
			}
			opts.PageSize = size
		}
		if l.MarginMM > 0 {
			opts.Margins = pdfout.UniformMargins(l.MarginMM)
		}
		if l.Font != "" {
			opts.Font = l.Font
		}
		if l.FontSize > 0 {
			opts.FontSize = l.FontSize
		}
		if l.LineSpacing > 0 {
			opts.LineSpacing = l.LineSpacing
		}
		applyBool(&opts.Images, l.Images)
		applyBool(&opts.Cover, l.Cover)
		applyBool(&opts.TitlePage, l.TitlePage)
		applyBool(&opts.PageNumbers, l.PageNumbers)
		applyBool(&opts.Bookmarks, l.Bookmarks)
	}

	if flagPageSize != "" {
		size, err := pdfout.LookupPageSize(flagPageSize)
		if err != nil {
			return opts, err
		}
		opts.PageSize = size
	}
	if cmd.Flags().Changed("margin") {
		opts.Margins = pdfout.UniformMargins(flagMargin)
	}
	if flagFont != "" {
		opts.Font = flagFont
	}
	if flagFontSize > 0 {
		opts.FontSize = flagFontSize
	}
	if flagLineSpacing > 0 {
		opts.LineSpacing = flagLineSpacing
	}
	if flagJustify {
		opts.Justify = true
	}
	if flagNoJustify {
		opts.Justify = false
	}
	if flagNoImages {
		opts.Images = false
	}
	if flagNoCover {
		opts.Cover = false
	}
	if flagNoTitlePage {
		opts.TitlePage = false
	}
	if flagNoPageNumbers {
		opts.PageNumbers = false
	}
	if flagRunningHeader {
		opts.RunningHeader = true
	}
	if flagNoBookmarks {
		opts.Bookmarks = false
	}
	if flagNoBreaks {
		opts.ChapterBreaks = false
	}

	opts.Normalize()
	return opts, nil
}

func applyBool(dst *bool, src *bool) {
	if src != nil {
		*dst = *src
	}
}

// buildOutputOptions resolves where the PDFs go.
func buildOutputOptions(cfg *config.Config, single bool) engine.OutputOptions {
	out := engine.OutputOptions{Conflict: engine.ConflictFail}

	if cfg != nil {
		if cfg.Output.Dir != "" {
			out.Dir = cfg.Output.Dir
		}
		if cfg.Output.Suffix != "" {
			out.Suffix = cfg.Output.Suffix
		}
		switch engine.ConflictMode(cfg.Output.Conflict) {
		case engine.ConflictOverwrite, engine.ConflictRename, engine.ConflictSkip:
			out.Conflict = engine.ConflictMode(cfg.Output.Conflict)
		}
	}

	if flagDir != "" {
		out.Dir = flagDir
	}
	if flagSuffix != "" {
		out.Suffix = flagSuffix
	}
	if single && flagOutput != "" {
		out.Path = flagOutput
	}

	switch {
	case flagOverwrite:
		out.Conflict = engine.ConflictOverwrite
	case flagAutoRename:
		out.Conflict = engine.ConflictRename
	case flagSkipExisting:
		out.Conflict = engine.ConflictSkip
	}
	return out
}
