package style

import "testing"

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1 KB"},
		{319898, "312 KB"},
		{1 << 20, "1.0 MB"},
		{1<<20 + 1<<19, "1.5 MB"},
		{1 << 30, "1.00 GB"},
	}
	for _, tt := range tests {
		if got := FormatBytes(tt.in); got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
