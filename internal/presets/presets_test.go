package presets

import "testing"

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
