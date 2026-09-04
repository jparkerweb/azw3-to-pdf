package screens

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/jparkerweb/azw3-to-pdf/internal/pdfout"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/messages"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/style"
)

// fontChoices are the typefaces offered in the interface. A path can still be
// passed on the command line.
var fontChoices = []string{"serif", "sans", "mono"}

// LayoutModel is the fine-tuning screen: every knob the renderer exposes.
type LayoutModel struct {
	original pdfout.Options
	opts     pdfout.Options

	pageSizes []pdfout.PageSize
	sizeIndex int
	fontIndex int

	cursor int
	width  int
	height int
}

// layoutField describes one editable row.
type layoutField struct {
	label  string
	value  func(m *LayoutModel) string
	adjust func(m *LayoutModel, delta int)
	toggle func(m *LayoutModel)
	hint   string
}

var layoutFields = []layoutField{
	{
		label: "Page size",
		value: func(m *LayoutModel) string { return m.pageSizes[m.sizeIndex].Label },
		adjust: func(m *LayoutModel, d int) {
			m.sizeIndex = wrapIndex(m.sizeIndex+d, len(m.pageSizes))
			m.opts.PageSize = m.pageSizes[m.sizeIndex]
		},
		hint: "The paper the book is laid out on.",
	},
	{
		label: "Margins",
		value: func(m *LayoutModel) string { return fmt.Sprintf("%.0f mm", m.marginMM()) },
		adjust: func(m *LayoutModel, d int) {
			m.setMargin(m.marginMM() + float64(d))
		},
		hint: "White space around the text on every side.",
	},
	{
		label: "Typeface",
		value: func(m *LayoutModel) string { return m.opts.Font },
		adjust: func(m *LayoutModel, d int) {
			m.fontIndex = wrapIndex(m.fontIndex+d, len(fontChoices))
			m.opts.Font = fontChoices[m.fontIndex]
		},
		hint: "Uses a matching system font, or the built-in Go faces.",
	},
	{
		label: "Text size",
		value: func(m *LayoutModel) string { return fmt.Sprintf("%.1f pt", m.opts.FontSize) },
		adjust: func(m *LayoutModel, d int) {
			m.opts.FontSize += float64(d) * 0.5
			m.opts.Normalize()
		},
		hint: "Larger text means more pages.",
	},
	{
		label: "Line spacing",
		value: func(m *LayoutModel) string { return fmt.Sprintf("%.2f", m.opts.LineSpacing) },
		adjust: func(m *LayoutModel, d int) {
			m.opts.LineSpacing += float64(d) * 0.05
			m.opts.Normalize()
		},
		hint: "Leading between lines, as a multiple of the text size.",
	},
	{
		label:  "Justify text",
		value:  func(m *LayoutModel) string { return yesNo(m.opts.Justify) },
		toggle: func(m *LayoutModel) { m.opts.Justify = !m.opts.Justify },
		hint:   "Straight right margin, as in a printed book.",
	},
	{
		label:  "Illustrations",
		value:  func(m *LayoutModel) string { return yesNo(m.opts.Images) },
		toggle: func(m *LayoutModel) { m.opts.Images = !m.opts.Images },
		hint:   "Turning these off makes a much smaller PDF.",
	},
	{
		label:  "Cover page",
		value:  func(m *LayoutModel) string { return yesNo(m.opts.Cover) },
		toggle: func(m *LayoutModel) { m.opts.Cover = !m.opts.Cover },
		hint:   "The book's own cover art, full page.",
	},
	{
		label:  "Title page",
		value:  func(m *LayoutModel) string { return yesNo(m.opts.TitlePage) },
		toggle: func(m *LayoutModel) { m.opts.TitlePage = !m.opts.TitlePage },
		hint:   "A generated page with the title and author.",
	},
	{
		label:  "Page numbers",
		value:  func(m *LayoutModel) string { return yesNo(m.opts.PageNumbers) },
		toggle: func(m *LayoutModel) { m.opts.PageNumbers = !m.opts.PageNumbers },
		hint:   "Printed at the foot of every page.",
	},
	{
		label:  "Running header",
		value:  func(m *LayoutModel) string { return yesNo(m.opts.RunningHeader) },
		toggle: func(m *LayoutModel) { m.opts.RunningHeader = !m.opts.RunningHeader },
		hint:   "The book title along the top of each page.",
	},
	{
		label:  "Bookmarks",
		value:  func(m *LayoutModel) string { return yesNo(m.opts.Bookmarks) },
		toggle: func(m *LayoutModel) { m.opts.Bookmarks = !m.opts.Bookmarks },
		hint:   "Builds the PDF outline from the book's headings.",
	},
	{
		label:  "Chapter breaks",
		value:  func(m *LayoutModel) string { return yesNo(m.opts.ChapterBreaks) },
		toggle: func(m *LayoutModel) { m.opts.ChapterBreaks = !m.opts.ChapterBreaks },
		hint:   "Start each chapter on a fresh page, as the book asks.",
	},
}

// NewLayoutModel builds the fine-tuning screen.
func NewLayoutModel(opts pdfout.Options) LayoutModel {
	m := LayoutModel{pageSizes: pdfout.PageSizes()}
	m.SetOptions(opts)
	m.original = m.opts
	return m
}

// SetOptions loads the current layout into the screen.
func (m *LayoutModel) SetOptions(opts pdfout.Options) {
	opts.Normalize()
	m.opts = opts
	m.original = opts
	m.sizeIndex = 0
	for i, s := range m.pageSizes {
		if s.Name == opts.PageSize.Name {
			m.sizeIndex = i
		}
	}
	if m.pageSizes[m.sizeIndex].Name != opts.PageSize.Name {
		// A custom measurement: keep it in the list so it can be chosen again.
		m.pageSizes = append(m.pageSizes, opts.PageSize)
		m.sizeIndex = len(m.pageSizes) - 1
	}
	m.fontIndex = 0
	for i, f := range fontChoices {
		if f == opts.Font {
			m.fontIndex = i
		}
	}
}

func (m *LayoutModel) marginMM() float64 { return m.opts.Margins.Top * 25.4 / 72 }

func (m *LayoutModel) setMargin(mm float64) {
	if mm < 0 {
		mm = 0
	}
	if mm > 60 {
		mm = 60
	}
	m.opts.Margins = pdfout.UniformMargins(mm)
	m.opts.Normalize()
}

// Init does nothing.
func (m LayoutModel) Init() tea.Cmd { return nil }

// Update handles the settings keys.
func (m LayoutModel) Update(msg tea.Msg) (style.ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyPressMsg:
		field := layoutFields[m.cursor]
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(layoutFields)-1 {
				m.cursor++
			}
		case "left", "h":
			m.change(field, -1)
		case "right", "l":
			m.change(field, +1)
		case " ":
			m.change(field, +1)
		case "r":
			m.opts = m.original
			m.SetOptions(m.original)
		case "enter":
			return m, m.apply()
		case "esc":
			return m, back()
		}
	}
	return m, nil
}

func (m *LayoutModel) change(field layoutField, delta int) {
	switch {
	case field.toggle != nil:
		field.toggle(m)
	case field.adjust != nil:
		field.adjust(m, delta)
	}
}

func (m LayoutModel) apply() tea.Cmd {
	opts := m.opts
	margin := m.marginMM()
	return func() tea.Msg {
		return messages.LayoutChangedMsg{
			PageSize:    opts.PageSize.Name,
			MarginMM:    margin,
			Font:        opts.Font,
			FontSize:    opts.FontSize,
			LineSpacing: opts.LineSpacing,
			Justify:     opts.Justify,
			Images:      opts.Images,
			Cover:       opts.Cover,
			TitlePage:   opts.TitlePage,
			PageNumbers: opts.PageNumbers,
			Header:      opts.RunningHeader,
			Bookmarks:   opts.Bookmarks,
			Breaks:      opts.ChapterBreaks,
		}
	}
}

// View renders the settings list.
func (m LayoutModel) View() string {
	var b strings.Builder
	b.WriteString(style.TitleStyle().Render("Fine-tune the layout"))
	b.WriteString("\n")

	for i, f := range layoutFields {
		marker := "  "
		label := style.LabelStyle().Width(18).Render(f.label)
		value := style.ValueStyle().Render(f.value(&m))
		if i == m.cursor {
			marker = style.SelectedStyle().Render("> ")
			value = style.SelectedStyle().Render("‹ " + f.value(&m) + " ›")
		}
		b.WriteString(marker + label + value + "\n")
	}

	b.WriteString("\n")
	b.WriteString(style.AccentStyle().Render(layoutFields[m.cursor].hint))
	b.WriteString("\n\n")

	cols := m.opts.PageSize.Width - m.opts.Margins.Left - m.opts.Margins.Right
	chars := int(cols / (m.opts.FontSize * 0.5))
	b.WriteString(style.MutedStyle().Render(fmt.Sprintf(
		"Text column: %.0f pt wide, roughly %d characters per line", cols, chars)))
	b.WriteString("\n")

	return b.String()
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func wrapIndex(i, n int) int {
	if n == 0 {
		return 0
	}
	return ((i % n) + n) % n
}
