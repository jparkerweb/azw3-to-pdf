package style

import tea "charm.land/bubbletea/v2"

// ScreenModel is what every screen in the interface implements. It mirrors
// tea.Model except that View returns a plain string: only the top-level app
// decides about the alternate screen and mouse handling.
type ScreenModel interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (ScreenModel, tea.Cmd)
	View() string
}
