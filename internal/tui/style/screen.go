package style

import (
	tea "charm.land/bubbletea/v2"

	"github.com/jparkerweb/azw3-to-pdf/internal/ebook"
	"github.com/jparkerweb/azw3-to-pdf/internal/engine"
	"github.com/jparkerweb/azw3-to-pdf/internal/pdfout"
	"github.com/jparkerweb/azw3-to-pdf/internal/presets"
)

// ScreenModel is what every screen in the interface implements. It mirrors
// tea.Model except that View returns a plain string: only the top-level app
// decides about the alternate screen and mouse handling.
type ScreenModel interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (ScreenModel, tea.Cmd)
	View() string
}

// Screen is a ScreenModel that also owns what used to be app.go's three
// hand-maintained switches: its footer hint, its help-overlay bindings and
// how shared conversion state reaches it.
type Screen interface {
	ScreenModel

	// FooterHint is the key hint line shown in the footer while this screen
	// is current.
	FooterHint() string

	// Bindings lists this screen's key/description pairs for the help
	// overlay, or nil when it adds none beyond the global bindings.
	Bindings() [][2]string

	// PushState hands the shared conversion state to the screen and returns
	// the updated model, the same value-return idiom as Update.
	PushState(SharedState) ScreenModel
}

// SharedState is the conversion state App threads to every screen that
// displays it.
type SharedState struct {
	Book   *ebook.Book
	Preset presets.Preset
	PDF    pdfout.Options
	Output engine.OutputOptions
	Result *engine.Result
}
