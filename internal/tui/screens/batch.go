package screens

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jparkerweb/ebook-to-pdf/internal/ebook"
	"github.com/jparkerweb/ebook-to-pdf/internal/engine"
	"github.com/jparkerweb/ebook-to-pdf/internal/pdfout"
	"github.com/jparkerweb/ebook-to-pdf/internal/presets"
	"github.com/jparkerweb/ebook-to-pdf/internal/tui/messages"
	"github.com/jparkerweb/ebook-to-pdf/internal/tui/style"
)

// QueueItem is one book waiting to be converted.
type QueueItem struct {
	Path   string
	Title  string
	Author string
	Size   int64
	Images int
	Err    error
}

// Label is the item's display name.
func (q QueueItem) Label() string {
	if q.Title != "" {
		return q.Title
	}
	return filepath.Base(q.Path)
}

// BatchLoadedMsg carries the metadata read for a queue.
type BatchLoadedMsg struct{ Items []QueueItem }

// LoadBatch reads the metadata for every book in a queue.
func LoadBatch(paths []string) tea.Cmd {
	return func() tea.Msg {
		items := make([]QueueItem, 0, len(paths))
		for _, p := range paths {
			item := QueueItem{Path: p}
			if info, err := ebook.Open(p); err == nil {
				item.Title = info.Title
				item.Author = info.AuthorLine()
				item.Size = info.FileSize
				item.Images = len(info.Resources)
			} else {
				item.Err = err
			}
			items = append(items, item)
		}
		return BatchLoadedMsg{Items: items}
	}
}

// BatchQueueModel lists the books queued for conversion.
type BatchQueueModel struct {
	items   []QueueItem
	cursor  int
	loading bool
	frame   int
	preset  presets.Preset
	saved   string
	width   int
	height  int
}

// NewBatchQueueModel builds the queue screen.
func NewBatchQueueModel() BatchQueueModel { return BatchQueueModel{loading: true} }

// SetPreset records the layout the batch will use.
func (m *BatchQueueModel) SetPreset(p presets.Preset) { m.preset = p }

// Init starts the loading animation.
func (m BatchQueueModel) Init() tea.Cmd {
	if m.loading {
		return tick()
	}
	return nil
}

// Update handles queue editing.
func (m BatchQueueModel) Update(msg tea.Msg) (style.ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tickMsg:
		m.frame++
		if m.loading {
			return m, tick()
		}
		return m, nil

	case BatchLoadedMsg:
		m.items = msg.Items
		m.loading = false
		return m, nil

	case tea.KeyPressMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "d", "delete":
			if len(m.items) > 0 {
				m.items = append(m.items[:m.cursor], m.items[m.cursor+1:]...)
				if m.cursor >= len(m.items) && m.cursor > 0 {
					m.cursor--
				}
			}
			return m, nil
		case "p":
			return m, goTo(messages.ScreenPresets)
		case "l":
			return m, goTo(messages.ScreenLayout)
		case "enter":
			paths := m.readablePaths()
			if len(paths) == 0 {
				return m, status("Nothing in the queue can be converted")
			}
			return m, func() tea.Msg { return messages.BatchStartMsg{Paths: paths} }
		case "esc":
			return m, back()
		}
	}
	return m, nil
}

// FooterHint is the key hint line shown while this screen is current.
func (m BatchQueueModel) FooterHint() string {
	return "enter: start  |  d: remove  |  p: preset  |  l: layout  |  esc: back"
}

// Bindings lists this screen's help-overlay entries.
func (m BatchQueueModel) Bindings() [][2]string {
	return [][2]string{
		{"up/down", "Move through the queue"},
		{"d", "Remove the highlighted book"},
		{"enter", "Start the batch"},
	}
}

// PushState loads the shared preset into the screen.
func (m BatchQueueModel) PushState(state style.SharedState) style.ScreenModel {
	m.SetPreset(state.Preset)
	m.saved = state.SavedPageSize
	return m
}

func (m BatchQueueModel) readablePaths() []string {
	var out []string
	for _, item := range m.items {
		if item.Err == nil {
			out = append(out, item.Path)
		}
	}
	return out
}

// View renders the queue.
func (m BatchQueueModel) View() string {
	var b strings.Builder
	b.WriteString(style.TitleStyle().Render("Batch queue"))
	b.WriteString("\n")

	if m.loading {
		b.WriteString(style.AccentStyle().Render(spinnerFrames[m.frame%len(spinnerFrames)] + "  Reading the books…"))
		b.WriteString("\n")
		return b.String()
	}

	if len(m.items) == 0 {
		b.WriteString(style.MutedStyle().Render("The queue is empty."))
		b.WriteString("\n")
		return b.String()
	}

	var total int64
	for i, item := range m.items {
		marker := "  "
		name := style.ValueStyle().Render(style.Truncate(item.Label(), maxInt(m.width-24, 20)))
		if i == m.cursor {
			marker = style.SelectedStyle().Render("> ")
			name = style.SelectedStyle().Render(style.Truncate(item.Label(), maxInt(m.width-24, 20)))
		}
		b.WriteString(marker + name + "\n")

		detail := fmt.Sprintf("%s · %d images", formatBytes(item.Size), item.Images)
		if item.Err != nil {
			detail = "cannot be read: " + item.Err.Error()
			b.WriteString(style.ErrorStyle().Render("    " + style.Truncate(detail, maxInt(m.width-6, 20))))
		} else {
			total += item.Size
			b.WriteString(style.MutedStyle().Render("    " + detail))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(row("Books", fmt.Sprintf("%d queued, %s in total", len(m.items), formatBytes(total))))
	b.WriteString(row("Layout", m.preset.Name+" · "+m.preset.SummaryFor(m.saved)))

	b.WriteString("\n")
	b.WriteString(style.KeyHintStyle().Render("enter") + style.MutedStyle().Render(" start    "))
	b.WriteString(style.KeyHintStyle().Render("d") + style.MutedStyle().Render(" remove    "))
	b.WriteString(style.KeyHintStyle().Render("p") + style.MutedStyle().Render(" preset    "))
	b.WriteString(style.KeyHintStyle().Render("l") + style.MutedStyle().Render(" layout"))
	b.WriteString("\n")
	return b.String()
}

// BatchResult pairs a queued book with what happened to it.
type BatchResult struct {
	Path   string
	Result *engine.Result
	Err    error
}

// batchConcurrency keeps the interface converting one book at a time, so the
// progress screen has a single "now converting" book to show.
const batchConcurrency = 1

// runBatch is engine.RunBatch, replaceable in tests.
var runBatch = engine.RunBatch

// BatchProgressModel converts a queue one book at a time.
type BatchProgressModel struct {
	paths   []string
	results []BatchResult
	index   int
	current engine.Progress
	started time.Time
	frame   int
	stopped bool

	updates chan messages.BatchProgressMsg
	events  chan tea.Msg
	cancel  context.CancelFunc

	width  int
	height int
}

// NewBatchProgressModel builds the batch progress screen.
func NewBatchProgressModel() BatchProgressModel { return BatchProgressModel{} }

// Init does nothing: work starts from Start.
func (m BatchProgressModel) Init() tea.Cmd { return nil }

// Start converts every path in turn.
func (m *BatchProgressModel) Start(paths []string, out engine.OutputOptions, pdf pdfout.Options) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())

	m.paths = paths
	m.results = nil
	m.index = 0
	m.started = time.Now()
	m.stopped = false
	m.cancel = cancel
	m.updates = make(chan messages.BatchProgressMsg, 1)
	m.events = make(chan tea.Msg, len(paths)+1)

	// Each book gets its own destination, so --output is ignored here.
	out.Path = ""

	updates, events := m.updates, m.events
	go func() {
		defer close(updates)
		runBatch(ctx, paths, engine.BatchOptions{
			Output:      out,
			PDF:         pdf,
			Concurrency: batchConcurrency,
			OnProgress: func(i int, p engine.Progress) {
				select {
				case updates <- messages.BatchProgressMsg{Index: i, Progress: p}:
				default:
				}
			},
			OnResult: func(r engine.BatchResult) {
				events <- messages.BatchItemDoneMsg{Index: r.Index, Result: r.Result, Err: r.Err}
			},
		})
		events <- messages.BatchDoneMsg{}
	}()

	return tea.Batch(waitForBatchProgress(updates), waitForEvent(events), tick())
}

func waitForBatchProgress(ch chan messages.BatchProgressMsg) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return nil
		}
		return p
	}
}

func waitForEvent(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

// FooterHint is the key hint line shown while this screen is current.
func (m BatchProgressModel) FooterHint() string {
	return "esc: stop after this book  |  ctrl+c: quit"
}

// Bindings lists this screen's help-overlay entries: none beyond the global
// bindings.
func (m BatchProgressModel) Bindings() [][2]string { return nil }

// PushState does nothing: the batch progress screen carries no shared state
// of its own, it is driven entirely by Start.
func (m BatchProgressModel) PushState(style.SharedState) style.ScreenModel { return m }

// Results reports what happened to each book.
func (m BatchProgressModel) Results() []BatchResult { return m.results }

// Elapsed reports how long the batch has been running.
func (m BatchProgressModel) Elapsed() time.Duration { return time.Since(m.started) }

// Update handles batch progress.
func (m BatchProgressModel) Update(msg tea.Msg) (style.ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tickMsg:
		m.frame++
		return m, tick()

	case messages.BatchProgressMsg:
		m.index = msg.Index
		m.current = msg.Progress
		return m, waitForBatchProgress(m.updates)

	case messages.BatchItemDoneMsg:
		path := ""
		if msg.Index < len(m.paths) {
			path = m.paths[msg.Index]
		}
		m.results = append(m.results, BatchResult{Path: path, Result: msg.Result, Err: msg.Err})
		return m, waitForEvent(m.events)

	case tea.KeyPressMsg:
		if msg.String() == "esc" && !m.stopped {
			m.stopped = true
			if m.cancel != nil {
				m.cancel()
			}
			return m, status("Stopping after the current book")
		}
	}
	return m, nil
}

// View renders overall and per-book progress.
func (m BatchProgressModel) View() string {
	var b strings.Builder
	b.WriteString(style.TitleStyle().Render("Converting the batch"))
	b.WriteString("\n")

	done := len(m.results)
	overall := 0.0
	if len(m.paths) > 0 {
		overall = (float64(done) + m.current.Percent) / float64(len(m.paths))
	}

	width := m.width - 20
	if width < 20 {
		width = 20
	}
	if width > 60 {
		width = 60
	}

	b.WriteString(style.AccentStyle().Render(fmt.Sprintf("%s  book %d of %d",
		spinnerFrames[m.frame%len(spinnerFrames)], minInt(done+1, len(m.paths)), len(m.paths))))
	b.WriteString("\n\n")
	b.WriteString("  " + bar(overall, width))
	b.WriteString(style.ValueStyle().Render(fmt.Sprintf("  %3.0f%%", overall*100)))
	b.WriteString("\n\n")

	if m.index < len(m.paths) && done < len(m.paths) {
		name := filepath.Base(m.paths[m.index])
		b.WriteString(row("Now", style.Truncate(name, maxInt(m.width-18, 20))))
		b.WriteString(row("Stage", m.current.Stage.String()))
		if m.current.Page > 0 {
			b.WriteString(row("Pages", fmt.Sprintf("%d", m.current.Page)))
		}
	}
	b.WriteString(row("Elapsed", formatDuration(time.Since(m.started))))

	if len(m.results) > 0 {
		b.WriteString("\n")
		for _, r := range m.results {
			name := filepath.Base(r.Path)
			switch {
			case errors.Is(r.Err, engine.ErrSkipped):
				b.WriteString(style.WarningStyle().Render("  – " + name + " (already converted)"))
			case r.Err != nil:
				b.WriteString(style.ErrorStyle().Render("  x " + style.Truncate(name+": "+r.Err.Error(), maxInt(m.width-6, 20))))
			default:
				b.WriteString(style.SuccessStyle().Render(fmt.Sprintf("  ✓ %s (%d pages)", name, r.Result.Pages)))
			}
			b.WriteString("\n")
		}
	}

	if m.stopped {
		b.WriteString("\n")
		b.WriteString(style.WarningStyle().Render("Stopping after the current book…"))
		b.WriteString("\n")
	}
	return b.String()
}

// BatchCompleteModel summarises a finished batch.
type BatchCompleteModel struct {
	results []BatchResult
	elapsed time.Duration
	notice  string
	width   int
	height  int
}

// NewBatchCompleteModel builds the batch summary screen.
func NewBatchCompleteModel() BatchCompleteModel { return BatchCompleteModel{} }

// SetResults loads the outcome of a batch.
func (m *BatchCompleteModel) SetResults(results []BatchResult, elapsed time.Duration) {
	m.results = results
	m.elapsed = elapsed
	m.notice = ""
}

// Init does nothing.
func (m BatchCompleteModel) Init() tea.Cmd { return nil }

// Update handles the follow-up actions.
func (m BatchCompleteModel) Update(msg tea.Msg) (style.ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case tea.KeyPressMsg:
		switch msg.String() {
		case "f":
			if dir := m.outputDir(); dir != "" {
				if err := engine.OpenFolder(dir); err != nil {
					m.notice = "Could not open the folder: " + err.Error()
				} else {
					m.notice = "Opened " + dir
				}
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

// FooterHint is the key hint line shown while this screen is current.
func (m BatchCompleteModel) FooterHint() string {
	return "f: open folder  |  n: new batch  |  q: quit"
}

// Bindings lists this screen's help-overlay entries: none beyond the global
// bindings.
func (m BatchCompleteModel) Bindings() [][2]string { return nil }

// PushState does nothing: the batch summary is loaded via SetResults, not
// the shared conversion state.
func (m BatchCompleteModel) PushState(style.SharedState) style.ScreenModel { return m }

func (m BatchCompleteModel) outputDir() string {
	for _, r := range m.results {
		if r.Err == nil && r.Result != nil {
			return filepath.Dir(r.Result.OutputPath)
		}
	}
	return ""
}

// View renders the batch summary.
func (m BatchCompleteModel) View() string {
	var b strings.Builder

	var ok, skipped, failed, pages int
	var bytes int64
	for _, r := range m.results {
		switch {
		case errors.Is(r.Err, engine.ErrSkipped):
			skipped++
		case r.Err != nil:
			failed++
		default:
			ok++
			pages += r.Result.Pages
			bytes += r.Result.OutputSize
		}
	}

	headline := fmt.Sprintf("✓ %d book(s) converted", ok)
	if failed > 0 {
		headline += fmt.Sprintf(", %d failed", failed)
	}
	if skipped > 0 {
		headline += fmt.Sprintf(", %d skipped", skipped)
	}
	b.WriteString(style.SuccessStyle().Render(headline))
	b.WriteString("\n\n")

	b.WriteString(row("Pages", fmt.Sprintf("%d in total", pages)))
	b.WriteString(row("Written", formatBytes(bytes)))
	b.WriteString(row("Took", formatDuration(m.elapsed)))
	if dir := m.outputDir(); dir != "" {
		b.WriteString(row("Folder", style.Truncate(dir, maxInt(m.width-18, 20))))
	}
	b.WriteString("\n")

	for _, r := range m.results {
		name := filepath.Base(r.Path)
		switch {
		case errors.Is(r.Err, engine.ErrSkipped):
			b.WriteString(style.WarningStyle().Render("  – " + name + " (already converted)"))
		case r.Err != nil:
			b.WriteString(style.ErrorStyle().Render("  x " + style.Truncate(name+": "+r.Err.Error(), maxInt(m.width-6, 20))))
		default:
			b.WriteString(style.SuccessStyle().Render(fmt.Sprintf("  ✓ %s", filepath.Base(r.Result.OutputPath))))
			b.WriteString(style.MutedStyle().Render(fmt.Sprintf("  %d pages · %s", r.Result.Pages, formatBytes(r.Result.OutputSize))))
		}
		b.WriteString("\n")
	}

	if m.notice != "" {
		b.WriteString("\n")
		b.WriteString(style.AccentStyle().Render(m.notice))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(style.KeyHintStyle().Render("f") + style.MutedStyle().Render(" open the folder    "))
	b.WriteString(style.KeyHintStyle().Render("n") + style.MutedStyle().Render(" new batch    "))
	b.WriteString(style.KeyHintStyle().Render("q") + style.MutedStyle().Render(" quit"))
	b.WriteString("\n")
	return b.String()
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
