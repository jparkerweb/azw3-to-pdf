// Package style holds the terminal interface's colours, text styles and the
// interface every screen implements.
package style

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme is the full colour set used by the interface.
type Theme struct {
	Primary   color.Color
	Secondary color.Color
	Accent    color.Color
	Success   color.Color
	Warning   color.Color
	Error     color.Color
	Muted     color.Color
	Surface   color.Color
	Text      color.Color
	TextDim   color.Color
	Border    color.Color
}

// ThemeName identifies a theme.
type ThemeName string

const (
	// ThemeMidnightInk is the default: deep indigo with a warm gold accent,
	// the palette of a book read by lamplight.
	ThemeMidnightInk ThemeName = "midnight-ink"
	// ThemePaperSepia is the light-terminal companion: parchment and ink.
	ThemePaperSepia ThemeName = "paper-sepia"
)

// MidnightInk returns the default dark theme.
func MidnightInk() Theme {
	return Theme{
		Primary:   lipgloss.Color("#8B7BF7"), // lamplit violet
		Secondary: lipgloss.Color("#F0A868"), // warm gold
		Accent:    lipgloss.Color("#5ED3D3"), // faded teal
		Success:   lipgloss.Color("#7BD88F"), // moss
		Warning:   lipgloss.Color("#F0C674"), // amber
		Error:     lipgloss.Color("#F07178"), // rust
		Muted:     lipgloss.Color("#6B6B85"),
		Surface:   lipgloss.Color("#232336"),
		Text:      lipgloss.Color("#E6E6F0"),
		TextDim:   lipgloss.Color("#9A9AB5"),
		Border:    lipgloss.Color("#4A4A63"),
	}
}

// PaperSepia returns the warm, light-background theme.
func PaperSepia() Theme {
	return Theme{
		Primary:   lipgloss.Color("#8C5A3C"), // bound leather
		Secondary: lipgloss.Color("#B07D48"), // gilt
		Accent:    lipgloss.Color("#4F7A6F"), // ink green
		Success:   lipgloss.Color("#4F7A4F"),
		Warning:   lipgloss.Color("#B08948"),
		Error:     lipgloss.Color("#A64B4B"),
		Muted:     lipgloss.Color("#8A7B6B"),
		Surface:   lipgloss.Color("#EFE6D8"),
		Text:      lipgloss.Color("#2E2820"),
		TextDim:   lipgloss.Color("#6B6155"),
		Border:    lipgloss.Color("#B9A88F"),
	}
}

// ActiveTheme is the theme currently in use.
var ActiveTheme = MidnightInk()

// ActiveThemeName names the theme currently in use.
var ActiveThemeName = ThemeMidnightInk

// ToggleTheme switches between the two themes and returns the new name.
func ToggleTheme() ThemeName {
	if ActiveThemeName == ThemeMidnightInk {
		SetTheme(ThemePaperSepia)
	} else {
		SetTheme(ThemeMidnightInk)
	}
	return ActiveThemeName
}

// SetTheme selects a theme by name, reporting whether the name was known.
func SetTheme(name ThemeName) bool {
	switch name {
	case ThemeMidnightInk:
		ActiveTheme, ActiveThemeName = MidnightInk(), ThemeMidnightInk
		return true
	case ThemePaperSepia:
		ActiveTheme, ActiveThemeName = PaperSepia(), ThemePaperSepia
		return true
	}
	return false
}
