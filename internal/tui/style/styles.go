package style

import (
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"golang.org/x/term"
)

// HeaderStyle is the bar across the top of the screen.
func HeaderStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ActiveTheme.Text).
		Background(ActiveTheme.Primary).
		Bold(true).
		Padding(0, 1)
}

// FooterStyle is the key-hint bar across the bottom.
func FooterStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(ActiveTheme.TextDim).
		Background(ActiveTheme.Surface).
		Padding(0, 1)
}

// LogoStyle draws the wordmark on the welcome screen.
func LogoStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme.Primary).Bold(true)
}

// TitleStyle is a screen heading.
func TitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme.Primary).Bold(true).MarginBottom(1)
}

// SubtitleStyle is a secondary heading.
func SubtitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme.Accent).Italic(true)
}

// CardStyle is a bordered container.
func CardStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ActiveTheme.Border).
		Foreground(ActiveTheme.Text).
		Padding(1, 2)
}

// CardActiveStyle is a highlighted container.
func CardActiveStyle() lipgloss.Style {
	return CardStyle().BorderForeground(ActiveTheme.Primary).Bold(true)
}

// ButtonStyle is an unfocused button.
func ButtonStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme.Text).Background(ActiveTheme.Surface).Padding(0, 2)
}

// ButtonActiveStyle is a focused button.
func ButtonActiveStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme.Text).Background(ActiveTheme.Primary).Bold(true).Padding(0, 2)
}

// ErrorStyle marks an error message.
func ErrorStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme.Error).Bold(true)
}

// MutedStyle is de-emphasised text.
func MutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme.Muted)
}

// KeyHintStyle marks keyboard hints.
func KeyHintStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme.Accent)
}

// AccentStyle is accent-coloured text.
func AccentStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme.Accent)
}

// SecondaryStyle is the secondary accent colour.
func SecondaryStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme.Secondary)
}

// SuccessStyle marks a good outcome.
func SuccessStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme.Success).Bold(true)
}

// WarningStyle marks something the user should notice.
func WarningStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme.Warning)
}

// LabelStyle is the left column of a detail list.
func LabelStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme.Muted).Width(14)
}

// ValueStyle is the right column of a detail list.
func ValueStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme.Text).Bold(true)
}

// BadgeStyle is a small inline tag.
func BadgeStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme.Text).Background(ActiveTheme.Primary).Padding(0, 1).Bold(true)
}

// SelectedStyle marks the highlighted row of a list.
func SelectedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(ActiveTheme.Secondary).Bold(true)
}

// TerminalWidth reports the terminal width, defaulting to 80.
func TerminalWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// TerminalHeight reports the terminal height, defaulting to 24.
func TerminalHeight() int {
	_, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || h <= 0 {
		return 24
	}
	return h
}

// Visible returns the printable width of a string, ignoring escape sequences.
func Visible(s string) int { return lipgloss.Width(s) }

// Center pads a rendered string so that it sits in the middle of width.
func Center(s string, width int) string {
	pad := (width - Visible(s)) / 2
	if pad <= 0 {
		return s
	}
	return strings.Repeat(" ", pad) + s
}

// Truncate shortens a string to width, ending with an ellipsis.
func Truncate(s string, width int) string {
	runes := []rune(s)
	if width <= 1 || len(runes) <= width {
		return s
	}
	return string(runes[:width-1]) + "…"
}
