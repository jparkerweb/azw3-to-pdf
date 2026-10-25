package presets

import (
	"strings"
	"testing"

	"github.com/jparkerweb/ebook-to-pdf/internal/pdfout"
)

func TestAllPresetsProduceValidOptions(t *testing.T) {
	for _, p := range All() {
		opts, err := p.Options()
		if err != nil {
			t.Errorf("preset %q: %v", p.Key, err)
			continue
		}
		if opts.PageSize.Width <= 0 || opts.PageSize.Height <= 0 {
			t.Errorf("preset %q has an empty page size", p.Key)
		}
		if opts.FontSize < 5 || opts.FontSize > 40 {
			t.Errorf("preset %q has an unreadable font size of %v", p.Key, opts.FontSize)
		}
		if width := opts.PageSize.Width - opts.Margins.Left - opts.Margins.Right; width < 100 {
			t.Errorf("preset %q leaves only %.0f pt for text", p.Key, width)
		}
		if p.Description == "" || p.Name == "" {
			t.Errorf("preset %q is missing a name or description", p.Key)
		}
	}
}

func TestLookup(t *testing.T) {
	if _, ok := Lookup("EREADER"); !ok {
		t.Error("Lookup should ignore case")
	}
	if _, ok := Lookup("  paperback "); !ok {
		t.Error("Lookup should ignore surrounding space")
	}
	if _, ok := Lookup("nope"); ok {
		t.Error("Lookup accepted an unknown preset")
	}
	if Default().Key != "ereader" {
		t.Errorf("the default preset is %q", Default().Key)
	}
}

func TestRecommend(t *testing.T) {
	cases := []struct {
		name string
		book Book
		want string
	}{
		{"illustrated", Book{Images: 60}, "print"},
		{"very long", Book{TextBytes: 2_000_000}, "compact"},
		{"a few pictures", Book{Images: 12}, "paperback"},
		{"plain novel", Book{TextBytes: 400_000}, "ereader"},
	}
	for _, tc := range cases {
		if got := Recommend(tc.book).Key; got != tc.want {
			t.Errorf("%s: recommended %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestSavedPageSize(t *testing.T) {
	cases := []struct {
		preset, saved, want string
	}{
		{"ereader", "", "letter"},
		{"ereader", "a5", "a5"},
		{"phone", "a5", "kindle"},
		{"paperback", "a5", "trade"},
		{"a4", "a5", "a4"},
		{"large-print", "", "letter"},
	}
	for _, tc := range cases {
		p, _ := Lookup(tc.preset)
		opts, err := p.OptionsWithPageSize(tc.saved)
		if err != nil {
			t.Fatalf("%s with %q saved: %v", tc.preset, tc.saved, err)
		}
		want, _ := pdfout.LookupPageSize(tc.want)
		if opts.PageSize.Width != want.Width || opts.PageSize.Height != want.Height {
			t.Errorf("%s with %q saved: %v x %v, want %s (%v x %v)", tc.preset, tc.saved,
				opts.PageSize.Width, opts.PageSize.Height, tc.want, want.Width, want.Height)
		}
	}
	if got, _ := Default().Options(); got.PageSize.Width != 612 || got.PageSize.Height != 792 {
		t.Errorf("E-reader with nothing saved is %v x %v, want US Letter (612 x 792)", got.PageSize.Width, got.PageSize.Height)
	}
}

func TestSummaryFollowsSavedPageSize(t *testing.T) {
	ereader := Default()
	if got := ereader.SummaryFor("a5"); !strings.HasPrefix(got, "A5") {
		t.Errorf("E-reader with a5 saved is summarised as %q", got)
	}
	if got := ereader.Summary(); !strings.HasPrefix(got, "US Letter") {
		t.Errorf("E-reader with nothing saved is summarised as %q", got)
	}
	phone, _ := Lookup("phone")
	if phone.SummaryFor("a5") != phone.Summary() {
		t.Error("a saved size should not change the Phone summary")
	}
}
