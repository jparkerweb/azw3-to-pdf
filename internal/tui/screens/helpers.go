// Package screens holds the individual screens of the terminal interface.
package screens

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jparkerweb/azw3-to-pdf/internal/ebook"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/messages"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/style"
)

// formatBytes renders a byte count for humans.
func formatBytes(b int64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.2f GB", float64(b)/gb)
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/mb)
	case b >= kb:
		return fmt.Sprintf("%.0f KB", float64(b)/kb)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// formatDuration renders an elapsed time compactly.
func formatDuration(d time.Duration) string {
	switch {
	case d <= 0:
		return "--"
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%.1fs", d.Seconds())
	default:
		return fmt.Sprintf("%dm %02ds", int(d.Minutes()), int(d.Seconds())%60)
	}
}

// row renders a label and value as one line of a detail list.
func row(label, value string) string {
	return style.LabelStyle().Render(label) + style.ValueStyle().Render(value) + "\n"
}

// bar draws a simple progress bar of the given width.
func bar(percent float64, width int) string {
	if width < 4 {
		width = 4
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}
	filled := int(percent * float64(width))
	return style.AccentStyle().Render(strings.Repeat("█", filled)) +
		style.MutedStyle().Render(strings.Repeat("░", width-filled))
}

// back returns a command that goes to the previous screen.
func back() tea.Cmd {
	return func() tea.Msg { return messages.BackMsg{} }
}

// goTo returns a command that switches screens.
func goTo(s messages.Screen) tea.Cmd {
	return func() tea.Msg { return messages.NavigateMsg{Screen: s} }
}

// status returns a command that shows a message in the footer.
func status(format string, args ...any) tea.Cmd {
	text := fmt.Sprintf(format, args...)
	return func() tea.Msg { return messages.StatusMsg{Text: text} }
}

// LoadBook reads a book in the background and reports the outcome.
func LoadBook(path string) tea.Cmd {
	return func() tea.Msg {
		book, err := ebook.Open(path)
		return messages.BookLoadedMsg{Path: path, Book: book, Err: err}
	}
}

// spinnerFrames is the animation used while work is in progress.
var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type tickMsg time.Time

// tick schedules the next animation frame.
func tick() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(t time.Time) tea.Msg { return tickMsg(t) })
}
