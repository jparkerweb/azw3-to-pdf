package screens

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jparkerweb/azw3-to-pdf/internal/ebook"
	"github.com/jparkerweb/azw3-to-pdf/internal/engine"
	"github.com/jparkerweb/azw3-to-pdf/internal/pdfout"
	"github.com/jparkerweb/azw3-to-pdf/internal/presets"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/messages"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/style"
)

// PreviewModel is the last stop before converting: what will be produced, and
// where it will be written.
type PreviewModel struct {
	book   *ebook.Book
	preset presets.Preset
	opts   pdfout.Options
	output engine.OutputOptions

	outPath   string
	outErr    error
	editing   bool
	typedPath string

	width  int
	height int
}

// NewPreviewModel builds the confirmation screen.
func NewPreviewModel() PreviewModel { return PreviewModel{} }

// SetState refreshes everything the screen displays.
func (m *PreviewModel) SetState(book *ebook.Book, preset presets.Preset, opts pdfout.Options, out engine.OutputOptions) {
	m.book, m.preset, m.opts, m.output = book, preset, opts, out
	m.refreshOutput()
}

func (m *PreviewModel) refreshOutput() {
	m.outPath, m.outErr = "", nil
	if m.book == nil {
		return
	}
	// Resolving with ConflictRename shows the name that would really be used
	// without claiming it.
	probe := m.output
	if probe.Conflict == engine.ConflictFail {
		probe.Conflict = engine.ConflictRename
	}
	path, err := engine.ResolveOutput(m.book.Path, probe)
	m.outPath, m.outErr = path, err
}

// Init does nothing.
func (m PreviewModel) Init() tea.Cmd { return nil }

// Update handles the screen's keys.
func (m PreviewModel) Update(msg tea.Msg) (style.ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyPressMsg:
		if m.editing {
			return m.editPath(msg)
		}
		switch msg.String() {
		case "enter":
			if m.book == nil {
				return m, nil
			}
			return m, func() tea.Msg { return messages.ConvertStartMsg{} }
		case "l":
			return m, goTo(messages.ScreenLayout)
		case "p":
			return m, goTo(messages.ScreenPresets)
		case "o":
			m.editing = true
			m.typedPath = m.outPath
			return m, nil
		case "esc":
			return m, back()
		}
	}
	return m, nil
}

func (m PreviewModel) editPath(msg tea.KeyPressMsg) (style.ScreenModel, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.editing = false
		return m, nil
	case "enter":
		if path := strings.TrimSpace(m.typedPath); path != "" {
			m.output.Path = path
			m.output.Conflict = engine.ConflictOverwrite
			m.refreshOutput()
		}
		m.editing = false
		return m, status("Output set to %s", filepath.Base(m.outPath))
	case "backspace":
		if r := []rune(m.typedPath); len(r) > 0 {
			m.typedPath = string(r[:len(r)-1])
		}
		return m, nil
	case "ctrl+u":
		m.typedPath = ""
		return m, nil
	}
	if s := msg.String(); len([]rune(s)) == 1 {
		m.typedPath += s
	}
	return m, nil
}

// Output reports the destination the conversion will use.
func (m PreviewModel) Output() engine.OutputOptions { return m.output }

// View renders the summary.
func (m PreviewModel) View() string {
	if m.book == nil {
		return style.MutedStyle().Render("Choose a book first.")
	}

	var b strings.Builder
	b.WriteString(style.TitleStyle().Render("Ready to convert"))
	b.WriteString("\n")

	b.WriteString(row("Book", style.Truncate(m.book.Title, maxInt(m.width-18, 20))))
	b.WriteString(row("Author", m.book.AuthorLine()))
	b.WriteString(row("Source", formatBytes(m.book.FileSize)+" · "+m.book.Format))
	b.WriteString("\n")

	b.WriteString(row("Preset", m.preset.Name))
	b.WriteString(row("Page", m.opts.PageSize.Label))
	b.WriteString(row("Type", fmt.Sprintf("%s %.1f pt, %.2f line spacing", m.opts.Font, m.opts.FontSize, m.opts.LineSpacing)))
	b.WriteString(row("Margins", fmt.Sprintf("%.0f mm", m.opts.Margins.Top*25.4/72)))
	b.WriteString(row("Extras", m.extras()))
	b.WriteString("\n")

	if m.editing {
		b.WriteString(style.SubtitleStyle().Render("Write the PDF to"))
		b.WriteString("\n")
		width := m.width - 6
		if width < 20 {
			width = 20
		}
		b.WriteString(style.CardStyle().Width(width).Render("> " + m.typedPath + "▌"))
		b.WriteString("\n")
	} else {
		label := m.outPath
		if m.outErr != nil {
			label = m.outErr.Error()
		}
		b.WriteString(row("Save as", style.Truncate(label, maxInt(m.width-18, 20))))
	}

	b.WriteString("\n")
	b.WriteString(style.KeyHintStyle().Render("enter") + style.MutedStyle().Render(" convert    "))
	b.WriteString(style.KeyHintStyle().Render("l") + style.MutedStyle().Render(" layout    "))
	b.WriteString(style.KeyHintStyle().Render("p") + style.MutedStyle().Render(" preset    "))
	b.WriteString(style.KeyHintStyle().Render("o") + style.MutedStyle().Render(" change destination"))
	b.WriteString("\n")
	return b.String()
}

func (m PreviewModel) extras() string {
	var on []string
	for _, item := range []struct {
		enabled bool
		label   string
	}{
		{m.opts.Cover, "cover"},
		{m.opts.TitlePage, "title page"},
		{m.opts.Images, "illustrations"},
		{m.opts.Bookmarks, "bookmarks"},
		{m.opts.PageNumbers, "page numbers"},
		{m.opts.RunningHeader, "running header"},
		{m.opts.Justify, "justified"},
	} {
		if item.enabled {
			on = append(on, item.label)
		}
	}
	if len(on) == 0 {
		return "plain text only"
	}
	return strings.Join(on, ", ")
}
