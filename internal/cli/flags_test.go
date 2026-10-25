package cli

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/jparkerweb/ebook-to-pdf/internal/config"
	"github.com/jparkerweb/ebook-to-pdf/internal/pdfout"
	"github.com/jparkerweb/ebook-to-pdf/internal/presets"
)

// resetRunFlags clears the package-level flag variables, which persist between
// commands in one process.
func resetRunFlags() {
	flagPageSize, flagMargin, flagFont, flagFontSize, flagLineSpacing = "", 0, "", 0, 0
	flagJustify, flagNoJustify, flagNoImages, flagNoCover, flagNoTitlePage = false, false, false, false, false
	flagNoPageNumbers, flagRunningHeader, flagNoBookmarks, flagNoBreaks = false, false, false, false
}

func parsed(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	resetRunFlags()
	cmd := &cobra.Command{Use: "x"}
	registerRunFlags(cmd)
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatal(err)
	}
	return cmd
}

func TestBuildPDFOptionsPrecedence(t *testing.T) {
	off := false
	preset := presets.Default()

	t.Run("preset only", func(t *testing.T) {
		opts, err := buildPDFOptions(parsed(t), nil, preset)
		if err != nil {
			t.Fatal(err)
		}
		want, _ := preset.Options()
		if opts != want {
			t.Errorf("got %+v, want the preset's options %+v", opts, want)
		}
	})

	t.Run("config beats preset, false included", func(t *testing.T) {
		cfg := &config.Config{Layout: config.LayoutConfig{FontSize: 14, Images: &off}}
		opts, err := buildPDFOptions(parsed(t), cfg, preset)
		if err != nil {
			t.Fatal(err)
		}
		if opts.FontSize != 14 || opts.Images {
			t.Errorf("config not applied: font size %v, images %v", opts.FontSize, opts.Images)
		}
	})

	t.Run("flags beat config", func(t *testing.T) {
		cfg := &config.Config{Layout: config.LayoutConfig{FontSize: 14}}
		opts, err := buildPDFOptions(parsed(t, "--font-size", "9"), cfg, preset)
		if err != nil {
			t.Fatal(err)
		}
		if opts.FontSize != 9 {
			t.Errorf("font size is %v, want 9", opts.FontSize)
		}
	})

	t.Run("zero in config means unset", func(t *testing.T) {
		cfg := &config.Config{Layout: config.LayoutConfig{FontSize: 0, MarginMM: 0}}
		opts, err := buildPDFOptions(parsed(t), cfg, preset)
		if err != nil {
			t.Fatal(err)
		}
		want, _ := preset.Options()
		if opts != want {
			t.Errorf("a zero config value changed the preset's options")
		}
	})

	t.Run("explicit margin 0 counts as set", func(t *testing.T) {
		opts, err := buildPDFOptions(parsed(t, "--margin", "0"), nil, preset)
		if err != nil {
			t.Fatal(err)
		}
		if opts.Margins.Top != 0 {
			t.Errorf("top margin is %v, want 0", opts.Margins.Top)
		}
	})

	t.Run("no-justify turns justification off", func(t *testing.T) {
		opts, err := buildPDFOptions(parsed(t, "--no-justify"), nil, preset)
		if err != nil {
			t.Fatal(err)
		}
		if opts.Justify {
			t.Error("justify should be off")
		}
	})

	t.Run("bad page size in config is an error", func(t *testing.T) {
		cfg := &config.Config{Layout: config.LayoutConfig{PageSize: "elephant"}}
		_, err := buildPDFOptions(parsed(t), cfg, preset)
		if err == nil {
			t.Fatal("expected an error for an unknown page size")
		}
		if !strings.Contains(err.Error(), "elephant") {
			t.Errorf("the error should name the bad value: %v", err)
		}
	})
}

func TestBuildPDFOptionsSavedPageSize(t *testing.T) {
	ereader := presets.Default()
	phone, _ := presets.Lookup("phone")
	size := func(name string) pdfout.PageSize {
		ps, err := pdfout.LookupPageSize(name)
		if err != nil {
			t.Fatal(err)
		}
		return ps
	}
	a5 := &config.Config{Layout: config.LayoutConfig{PageSize: "a5"}}

	cases := []struct {
		name   string
		args   []string
		cfg    *config.Config
		preset presets.Preset
		want   string
	}{
		{"no config gives Letter", nil, nil, ereader, "letter"},
		{"saved a5 applies to E-reader", nil, a5, ereader, "a5"},
		{"saved a5 leaves Phone alone", nil, a5, phone, "kindle"},
		{"--page-size beats the size preset", []string{"--page-size", "a6"}, nil, phone, "a6"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := buildPDFOptions(parsed(t, tc.args...), tc.cfg, tc.preset)
			if err != nil {
				t.Fatal(err)
			}
			if want := size(tc.want); opts.PageSize != want {
				t.Errorf("page size is %+v, want %s", opts.PageSize, tc.want)
			}
		})
	}
}
