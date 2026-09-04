package tui

import "charm.land/bubbles/v2/key"

// KeyMap holds the bindings that work on every screen.
type KeyMap struct {
	Quit        key.Binding
	Back        key.Binding
	Enter       key.Binding
	Help        key.Binding
	ThemeToggle key.Binding
}

// DefaultKeyMap returns the standard global bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Quit: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("ctrl+c", "quit"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "confirm"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		ThemeToggle: key.NewBinding(
			key.WithKeys("ctrl+t"),
			key.WithHelp("ctrl+t", "theme"),
		),
	}
}
