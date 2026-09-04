package screens

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jparkerweb/azw3-to-pdf/internal/tui/messages"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/style"
)

// splashDwell is how long the welcome screen stays up on its own.
const splashDwell = 1500 * time.Millisecond

// logo is the wordmark shown on the welcome screen.
var logo = []string{
	`                     _____    __                     ______`,
	`  ____ ______      _|__  /   / /_____     ____  ____/ / __/`,
	` / __ '/_  / | /| / //_ <   / __/ __ \   / __ \/ __  / /_`,
	`/ /_/ / / /| |/ |/ /__/ /  / /_/ /_/ /  / /_/ / /_/ / __/`,
	`\__,_/ /___/__/|__/____/   \__/\____/  / .___/\__,_/_/`,
	`                                      /_/`,
}

type splashDoneMsg struct{}

// SplashModel is the welcome screen.
type SplashModel struct {
	width   int
	height  int
	version string
}

// NewSplashModel builds the welcome screen.
func NewSplashModel(version string) SplashModel {
	return SplashModel{version: version}
}

// Init starts the timer that moves on to the file browser.
func (m SplashModel) Init() tea.Cmd {
	return tea.Tick(splashDwell, func(time.Time) tea.Msg { return splashDoneMsg{} })
}

// Update advances past the splash on a key press or when the timer fires.
func (m SplashModel) Update(msg tea.Msg) (style.ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case splashDoneMsg, tea.KeyPressMsg:
		return m, goTo(messages.ScreenFilePicker)
	}
	return m, nil
}

// View renders the welcome screen.
func (m SplashModel) View() string {
	var b strings.Builder

	top := (m.height - len(logo) - 10) / 3
	for i := 0; i < top; i++ {
		b.WriteString("\n")
	}

	// The wordmark is centred as one block rather than line by line, so that
	// its letterforms stay aligned with each other.
	art := 0
	for _, line := range logo {
		if len(line) > art {
			art = len(line)
		}
	}
	pad := ""
	if m.width > art {
		pad = strings.Repeat(" ", (m.width-art)/2)
	}
	for _, line := range logo {
		b.WriteString(pad + style.LogoStyle().Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(style.Center(style.SubtitleStyle().Render("Kindle books, laid out as PDFs."), m.width))
	b.WriteString("\n\n")

	for _, line := range []string{
		"reads .azw3, .azw, .mobi and .prc",
		"no Calibre, no Python, no other software required",
	} {
		b.WriteString(style.Center(style.MutedStyle().Render(line), m.width))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(style.Center(style.KeyHintStyle().Render("Press any key to choose a book"), m.width))
	return b.String()
}
