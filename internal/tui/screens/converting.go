package screens

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jparkerweb/azw3-to-pdf/internal/ebook"
	"github.com/jparkerweb/azw3-to-pdf/internal/engine"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/messages"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/style"
)

// ConvertingModel shows progress while one book is converted.
type ConvertingModel struct {
	book    *ebook.Book
	started time.Time
	frame   int

	progress engine.Progress
	err      error

	updates chan engine.Progress
	done    chan messages.ConvertDoneMsg
	cancel  context.CancelFunc

	width  int
	height int
}

// NewConvertingModel builds the progress screen.
func NewConvertingModel() ConvertingModel { return ConvertingModel{} }

// SetBook records which book is being converted.
func (m *ConvertingModel) SetBook(book *ebook.Book) { m.book = book }

// Init does nothing: work starts from Start.
func (m ConvertingModel) Init() tea.Cmd { return nil }

// Start kicks off a conversion and returns the commands that watch it.
func (m *ConvertingModel) Start(opts engine.Options) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())

	m.cancel = cancel
	m.started = time.Now()
	m.err = nil
	m.progress = engine.Progress{}
	// A one-slot channel keeps the converter running at full speed: updates
	// that the interface has not caught up with are simply dropped.
	m.updates = make(chan engine.Progress, 1)
	m.done = make(chan messages.ConvertDoneMsg, 1)

	updates, done := m.updates, m.done
	go func() {
		defer close(updates)
		res, err := engine.Convert(ctx, opts, func(p engine.Progress) {
			select {
			case updates <- p:
			default:
			}
		})
		done <- messages.ConvertDoneMsg{Result: res, Err: err}
	}()

	return tea.Batch(waitForProgress(updates), waitForDone(done), tick())
}

func waitForProgress(ch chan engine.Progress) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return nil
		}
		return messages.ConvertProgressMsg{Progress: p}
	}
}

func waitForDone(ch chan messages.ConvertDoneMsg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

// Update handles progress updates and cancellation.
func (m ConvertingModel) Update(msg tea.Msg) (style.ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tickMsg:
		m.frame++
		return m, tick()

	case messages.ConvertProgressMsg:
		m.progress = msg.Progress
		return m, waitForProgress(m.updates)

	case messages.ConvertDoneMsg:
		if msg.Err != nil {
			m.err = msg.Err
		}
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "c":
			if m.err != nil {
				return m, back()
			}
			if m.cancel != nil {
				m.cancel()
			}
			return m, func() tea.Msg { return messages.ConvertCancelMsg{} }
		case "enter":
			if m.err != nil {
				return m, back()
			}
		}
	}
	return m, nil
}

// View renders the progress display.
func (m ConvertingModel) View() string {
	var b strings.Builder

	if m.err != nil {
		b.WriteString(style.TitleStyle().Render("That did not work"))
		b.WriteString("\n")
		b.WriteString(style.ErrorStyle().Render(wrap(m.err.Error(), maxInt(m.width-4, 30), 8)))
		b.WriteString("\n\n")
		b.WriteString(style.KeyHintStyle().Render("enter") + style.MutedStyle().Render(" go back and try different settings"))
		b.WriteString("\n")
		return b.String()
	}

	title := "Converting"
	if m.book != nil {
		title = style.Truncate(m.book.Title, maxInt(m.width-16, 20))
	}
	b.WriteString(style.TitleStyle().Render(title))
	b.WriteString("\n")

	spinner := spinnerFrames[m.frame%len(spinnerFrames)]
	b.WriteString(style.AccentStyle().Render(spinner + "  " + m.progress.Stage.String()))
	b.WriteString("\n\n")

	width := m.width - 20
	if width < 20 {
		width = 20
	}
	if width > 60 {
		width = 60
	}
	b.WriteString("  " + bar(m.progress.Percent, width))
	b.WriteString(style.ValueStyle().Render(fmt.Sprintf("  %3.0f%%", m.progress.Percent*100)))
	b.WriteString("\n\n")

	if m.progress.Page > 0 {
		b.WriteString(row("Pages", fmt.Sprintf("%d laid out", m.progress.Page)))
	}
	if m.progress.Blocks > 0 {
		b.WriteString(row("Content", fmt.Sprintf("%d of %d blocks", m.progress.Block, m.progress.Blocks)))
	}
	b.WriteString(row("Elapsed", formatDuration(time.Since(m.started))))

	b.WriteString("\n")
	b.WriteString(style.MutedStyle().Render("esc cancels; the PDF is only written once it is complete."))
	b.WriteString("\n")
	return b.String()
}

// Book reports the book being converted, used by the completion screen.
func (m ConvertingModel) Book() *ebook.Book { return m.book }

// SourceName is the file name of the book being converted.
func (m ConvertingModel) SourceName() string {
	if m.book == nil {
		return ""
	}
	return filepath.Base(m.book.Path)
}
