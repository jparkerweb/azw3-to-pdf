package cli

import "testing"

func TestWrapText(t *testing.T) {
	cases := []struct {
		name  string
		s     string
		width int
		want  string
	}{
		{
			name:  "empty input",
			s:     "",
			width: 76,
			want:  "",
		},
		{
			name:  "shorter than width",
			s:     "hi there",
			width: 50,
			want:  "hi there",
		},
		{
			name:  "single word longer than width",
			s:     "supercalifragilisticexpialidocious",
			width: 10,
			want:  "supercalifragilisticexpialidocious",
		},
		{
			name:  "exact-width input fits on one line",
			s:     "ab cd",
			width: 5,
			want:  "ab cd",
		},
		{
			name:  "multi-line wrap at word boundaries",
			s:     "the quick brown fox jumps over the lazy dog",
			width: 10,
			want:  "the quick\nbrown fox\njumps over\nthe lazy\ndog",
		},
		{
			name:  "existing newlines are not preserved",
			s:     "one\ntwo three",
			width: 50,
			// strings.Fields splits on any whitespace, including the
			// original newline, so the text reflows as one paragraph.
			// Current (reflowing) behavior, pinned as-is.
			want: "one two three",
		},
		{
			name:  "multi-byte text wraps by byte count, not rune count",
			s:     "café io",
			width: 7,
			// "café" is 4 runes but 5 bytes (é is 2 bytes in UTF-8).
			// wrapText counts width with len(word), i.e. bytes, so it
			// wraps here (5+1+2=8 > 7) even though a rune-correct count
			// (4+1+2=7) would keep "café io" on one line. This is a
			// pre-existing bug (see phase-6.md Notes); pinning current
			// (byte-counting) behavior, not fixing it.
			want: "café\nio",
		},
		{
			name:  "zero width wraps every word after the first onto its own line",
			s:     "ab cd",
			width: 0,
			// line+1+len(word) > 0 is true for any word once line > 0, so
			// every word past the first starts a new line.
			want: "ab\ncd",
		},
		{
			name:  "word length equal to width still wraps once a line is started",
			s:     "ab abcde",
			width: 5,
			// "abcde" is exactly 5 bytes (== width), but the +1 for the
			// separating space after "ab" (2+1+5=8 > 5) still forces a wrap.
			want: "ab\nabcde",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := wrapText(tc.s, tc.width)
			if got != tc.want {
				t.Errorf("wrapText(%q, %d) = %q, want %q", tc.s, tc.width, got, tc.want)
			}
		})
	}
}
