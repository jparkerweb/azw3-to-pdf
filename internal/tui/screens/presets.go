package screens

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jparkerweb/azw3-to-pdf/internal/ebook"
	"github.com/jparkerweb/azw3-to-pdf/internal/presets"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/messages"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/style"
)

// PresetsModel lets the reader pick a layout.
type PresetsModel struct {
	list        []presets.Preset
	cursor      int
	recommended string
	width       int
	height      int
}

// NewPresetsModel builds the preset chooser with a preset pre-selected.
func NewPresetsModel(current presets.Preset) PresetsModel {
	m := PresetsModel{list: presets.All()}
	m.SetPreset(current)
	return m
}

// SetPreset moves the cursor to a preset.
func (m *PresetsModel) SetPreset(p presets.Preset) {
	for i, item := range m.list {
		if item.Key == p.Key {
			m.cursor = i
			return
		}
	}
}

// SetBook records which book the recommendation is for.
func (m *PresetsModel) SetBook(book *ebook.Book) {
	if book == nil {
		m.recommended = ""
		return
	}
	m.recommended = presets.Recommend(presets.Book{
		Images:    len(book.Resources),
		TextBytes: book.TextBytes,
		HasCover:  book.Cover != nil,
	}).Key
}

// Init does nothing.
func (m PresetsModel) Init() tea.Cmd { return nil }

// Update handles navigation and selection.
func (m PresetsModel) Update(msg tea.Msg) (style.ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.list)-1 {
				m.cursor++
			}
		case "home", "g":
			m.cursor = 0
		case "end", "G":
			m.cursor = len(m.list) - 1
		case "r":
			if m.recommended != "" {
				m.SetPreset(presets.Preset{Key: m.recommended})
			}
		case "enter":
			preset := m.list[m.cursor]
			return m, func() tea.Msg { return messages.PresetSelectedMsg{Preset: preset} }
		case "esc":
			return m, back()
		}
	}
	return m, nil
}

// View renders the preset list.
func (m PresetsModel) View() string {
	var b strings.Builder
	b.WriteString(style.TitleStyle().Render("How should the pages look?"))
	b.WriteString("\n")

	for i, p := range m.list {
		marker := "  "
		name := style.ValueStyle().Render(p.Name)
		if i == m.cursor {
			marker = style.SelectedStyle().Render("> ")
			name = style.SelectedStyle().Render(p.Name)
		}
		badge := ""
		if p.Key == m.recommended {
			badge = " " + style.BadgeStyle().Render("suggested")
		}
		fmt.Fprintf(&b, "%s%-16s%s\n", marker, name, badge)
		b.WriteString(style.MutedStyle().Render("    " + style.Truncate(p.Summary(), maxInt(m.width-6, 20))))
		b.WriteString("\n")
		if i == m.cursor {
			b.WriteString(style.AccentStyle().Render("    " + style.Truncate(p.Description, maxInt(m.width-6, 20))))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(style.KeyHintStyle().Render("enter") + style.MutedStyle().Render(" use this layout    "))
	b.WriteString(style.KeyHintStyle().Render("r") + style.MutedStyle().Render(" jump to the suggestion"))
	b.WriteString("\n")
	return b.String()
}
