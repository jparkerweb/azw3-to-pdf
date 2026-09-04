package screens

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jparkerweb/azw3-to-pdf/internal/ebook"
	"github.com/jparkerweb/azw3-to-pdf/internal/htmldoc"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/messages"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/style"
)

// InfoModel shows what was found inside the chosen book.
type InfoModel struct {
	book     *ebook.Book
	blocks   int
	headings []string
	width    int
	height   int
}

// NewInfoModel builds the book details screen.
func NewInfoModel() InfoModel { return InfoModel{} }

// SetBook loads a book into the screen and summarises its structure.
func (m *InfoModel) SetBook(book *ebook.Book) {
	m.book = book
	m.blocks, m.headings = 0, nil
	if book == nil {
		return
	}
	doc := htmldoc.ParseWithCSS(book.HTML, htmldoc.ParseCSS(book.Flows...))
	m.blocks = len(doc.Blocks)
	for _, i := range doc.Headings() {
		if len(m.headings) >= 12 {
			break
		}
		if text := strings.TrimSpace(doc.Blocks[i].Text()); text != "" {
			m.headings = append(m.headings, text)
		}
	}
}

// Init does nothing: the book is already loaded.
func (m InfoModel) Init() tea.Cmd { return nil }

// Update handles the screen's keys.
func (m InfoModel) Update(msg tea.Msg) (style.ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tea.KeyPressMsg:
		switch msg.String() {
		case "enter":
			return m, goTo(messages.ScreenPresets)
		case "c":
			return m, goTo(messages.ScreenPreview)
		case "esc":
			return m, back()
		}
	}
	return m, nil
}

// View renders the book details.
func (m InfoModel) View() string {
	if m.book == nil {
		return style.MutedStyle().Render("No book loaded.")
	}
	b := m.book

	var sb strings.Builder
	sb.WriteString(style.TitleStyle().Render(style.Truncate(b.Title, maxInt(m.width-4, 20))))
	sb.WriteString("\n")
	sb.WriteString(style.SubtitleStyle().Render(b.AuthorLine()))
	sb.WriteString("\n\n")

	sb.WriteString(row("File", filepath.Base(b.Path)))
	sb.WriteString(row("Size", formatBytes(b.FileSize)))
	sb.WriteString(row("Format", b.Format))
	sb.WriteString(row("Compression", b.Compression))
	if b.Publisher != "" {
		sb.WriteString(row("Publisher", b.Publisher))
	}
	if b.Published != "" {
		sb.WriteString(row("Published", b.Published))
	}
	if b.Language != "" {
		sb.WriteString(row("Language", b.Language))
	}
	if b.ISBN != "" {
		sb.WriteString(row("ISBN", b.ISBN))
	}
	sb.WriteString(row("Content", fmt.Sprintf("%s of markup, %d blocks", formatBytes(int64(b.TextBytes)), m.blocks)))

	images := fmt.Sprintf("%d embedded", len(b.Resources))
	if b.Cover != nil {
		images += ", cover included"
	}
	sb.WriteString(row("Images", images))

	if len(b.Subjects) > 0 {
		sb.WriteString(row("Subjects", style.Truncate(strings.Join(b.Subjects, ", "), maxInt(m.width-18, 20))))
	}

	if b.Description != "" {
		sb.WriteString("\n")
		sb.WriteString(style.MutedStyle().Render(wrap(b.Description, maxInt(m.width-4, 30), 6)))
		sb.WriteString("\n")
	}

	if len(m.headings) > 0 {
		sb.WriteString("\n")
		sb.WriteString(style.AccentStyle().Render("Chapters found"))
		sb.WriteString("\n")
		for _, h := range m.headings {
			sb.WriteString(style.MutedStyle().Render("  · " + style.Truncate(h, maxInt(m.width-8, 20))))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(style.KeyHintStyle().Render("enter") + style.MutedStyle().Render(" choose a layout preset    "))
	sb.WriteString(style.KeyHintStyle().Render("c") + style.MutedStyle().Render(" convert with current settings"))
	sb.WriteString("\n")
	return sb.String()
}

// wrap hard-wraps text to a width, stopping after a number of lines.
func wrap(s string, width, maxLines int) string {
	var out []string
	var line strings.Builder
	for _, word := range strings.Fields(s) {
		if line.Len() > 0 && line.Len()+1+len(word) > width {
			out = append(out, line.String())
			if len(out) >= maxLines {
				return strings.Join(out, "\n") + " …"
			}
			line.Reset()
		}
		if line.Len() > 0 {
			line.WriteByte(' ')
		}
		line.WriteString(word)
	}
	if line.Len() > 0 {
		out = append(out, line.String())
	}
	return strings.Join(out, "\n")
}
