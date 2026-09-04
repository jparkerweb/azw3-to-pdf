package screens

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jparkerweb/azw3-to-pdf/internal/engine"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/messages"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/style"
)

// CompleteModel reports a finished conversion and offers what to do next.
type CompleteModel struct {
	result *engine.Result
	notice string
	width  int
	height int
}

// NewCompleteModel builds the completion screen.
func NewCompleteModel() CompleteModel { return CompleteModel{} }

// SetResult loads the outcome of a conversion.
func (m *CompleteModel) SetResult(res *engine.Result) {
	m.result = res
	m.notice = ""
}

// Init does nothing.
func (m CompleteModel) Init() tea.Cmd { return nil }

// Update handles the follow-up actions.
func (m CompleteModel) Update(msg tea.Msg) (style.ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyPressMsg:
		if m.result == nil {
			return m, nil
		}
		switch msg.String() {
		case "o":
			if err := engine.OpenFile(m.result.OutputPath); err != nil {
				m.notice = "Could not open the PDF: " + err.Error()
			} else {
				m.notice = "Opened " + filepath.Base(m.result.OutputPath)
			}
			return m, nil
		case "f":
			dir := filepath.Dir(m.result.OutputPath)
			if err := engine.OpenFolder(dir); err != nil {
				m.notice = "Could not open the folder: " + err.Error()
			} else {
				m.notice = "Opened " + dir
			}
			return m, nil
		case "n":
			return m, goTo(messages.ScreenFilePicker)
		case "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

// View renders the summary of what was written.
func (m CompleteModel) View() string {
	if m.result == nil {
		return style.MutedStyle().Render("Nothing has been converted yet.")
	}
	r := m.result

	var b strings.Builder
	b.WriteString(style.SuccessStyle().Render("✓ Converted"))
	b.WriteString("\n\n")

	if r.Book != nil {
		b.WriteString(style.TitleStyle().Render(style.Truncate(r.Book.Title, maxInt(m.width-4, 20))))
		b.WriteString("\n")
	}

	b.WriteString(row("Saved to", style.Truncate(r.OutputPath, maxInt(m.width-18, 20))))
	b.WriteString(row("Pages", fmt.Sprintf("%d", r.Pages)))
	b.WriteString(row("Illustrations", fmt.Sprintf("%d placed", r.Images)))
	if r.Dropped > 0 {
		b.WriteString(row("Skipped", fmt.Sprintf("%d images could not be read", r.Dropped)))
	}
	b.WriteString(row("Bookmarks", fmt.Sprintf("%d", r.Headings)))
	b.WriteString(row("Typeface", r.Font))
	b.WriteString(row("Size", fmt.Sprintf("%s from %s (%.0f%%)",
		formatBytes(r.OutputSize), formatBytes(r.InputSize), r.SizeRatio()*100)))
	b.WriteString(row("Took", formatDuration(r.Elapsed)))

	if m.notice != "" {
		b.WriteString("\n")
		b.WriteString(style.AccentStyle().Render(m.notice))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(style.KeyHintStyle().Render("o") + style.MutedStyle().Render(" open the PDF    "))
	b.WriteString(style.KeyHintStyle().Render("f") + style.MutedStyle().Render(" open the folder    "))
	b.WriteString(style.KeyHintStyle().Render("n") + style.MutedStyle().Render(" another book    "))
	b.WriteString(style.KeyHintStyle().Render("q") + style.MutedStyle().Render(" quit"))
	b.WriteString("\n")
	return b.String()
}
