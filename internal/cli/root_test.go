package cli

import (
	"os"
	"testing"
)

func TestNoColorSetsTheEnvironment(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"absent", []string{"book.epub"}, false},
		{"plain", []string{"--no-color", "book.epub"}, true},
		{"with help", []string{"--help", "--no-color"}, true},
		{"explicit true", []string{"--no-color=true"}, true},
		{"explicit false", []string{"--no-color=false"}, false},
		{"after the terminator", []string{"--", "--no-color"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Setenv restores the variable afterwards; unset it so the
			// test sees only what applyNoColor does.
			t.Setenv("NO_COLOR", "")
			_ = os.Unsetenv("NO_COLOR")

			applyNoColor(tt.args)
			_, got := os.LookupEnv("NO_COLOR")
			if got != tt.want {
				t.Errorf("NO_COLOR set = %v for %q, want %v", got, tt.args, tt.want)
			}
		})
	}
}
