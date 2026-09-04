// Package tui is the terminal interface: a file browser, layout chooser and
// progress display wrapped around the conversion engine.
package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/jparkerweb/azw3-to-pdf/internal/config"
	"github.com/jparkerweb/azw3-to-pdf/internal/ebook"
	"github.com/jparkerweb/azw3-to-pdf/internal/engine"
	"github.com/jparkerweb/azw3-to-pdf/internal/pdfout"
	"github.com/jparkerweb/azw3-to-pdf/internal/presets"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/messages"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/screens"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/style"
)

// AppOptions configures a new interface session.
type AppOptions struct {
	Version string
	Inputs  []string
	Book    *ebook.Book
	Preset  presets.Preset
	PDF     pdfout.Options
	Output  engine.OutputOptions
	Config  *config.Config
}

// App is the top-level Bubble Tea model.
type App struct {
	current  messages.Screen
	history  []messages.Screen
	screens  map[messages.Screen]style.ScreenModel
	width    int
	height   int
	keys     KeyMap
	showHelp bool
	viewport viewport.Model
	status   string

	version string
	cfg     *config.Config
	book    *ebook.Book
	inputs  []string
	preset  presets.Preset
	pdf     pdfout.Options
	output  engine.OutputOptions
	result  *engine.Result
}

// NewApp builds the interface.
func NewApp(opts AppOptions) App {
	version := opts.Version
	if version == "" {
		version = "dev"
	}
	if opts.Preset.Key == "" {
		opts.Preset = presets.Default()
	}
	if opts.PDF.PageSize.Width == 0 {
		opts.PDF = pdfout.DefaultOptions()
	}

	vp := viewport.New()
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3
	// Arrow keys belong to the screens; the viewport only scrolls by page.
	vp.KeyMap.Up.SetEnabled(false)
	vp.KeyMap.Down.SetEnabled(false)
	vp.KeyMap.Left.SetEnabled(false)
	vp.KeyMap.Right.SetEnabled(false)

	a := App{
		current:  messages.ScreenSplash,
		screens:  map[messages.Screen]style.ScreenModel{},
		keys:     DefaultKeyMap(),
		viewport: vp,
		version:  version,
		cfg:      opts.Config,
		book:     opts.Book,
		inputs:   opts.Inputs,
		preset:   opts.Preset,
		pdf:      opts.PDF,
		output:   opts.Output,
	}

	a.screens[messages.ScreenSplash] = screens.NewSplashModel(version)
	a.screens[messages.ScreenFilePicker] = screens.NewFilePickerModel()
	a.screens[messages.ScreenInfo] = screens.NewInfoModel()
	a.screens[messages.ScreenPresets] = screens.NewPresetsModel(opts.Preset)
	a.screens[messages.ScreenLayout] = screens.NewLayoutModel(opts.PDF)
	a.screens[messages.ScreenPreview] = screens.NewPreviewModel()
	a.screens[messages.ScreenConverting] = screens.NewConvertingModel()
	a.screens[messages.ScreenComplete] = screens.NewCompleteModel()
	a.screens[messages.ScreenBatchQueue] = screens.NewBatchQueueModel()
	a.screens[messages.ScreenBatchProgress] = screens.NewBatchProgressModel()
	a.screens[messages.ScreenBatchComplete] = screens.NewBatchCompleteModel()

	a.pushState()
	return a
}

// Init starts the splash screen, or jumps straight past it when books were
// named on the command line.
func (a App) Init() tea.Cmd {
	splash := a.screens[messages.ScreenSplash].Init()

	switch {
	case a.book != nil:
		return tea.Batch(splash, navigate(messages.ScreenInfo))
	case len(a.inputs) > 1:
		paths := a.inputs
		return tea.Batch(splash, func() tea.Msg { return messages.FilesSelectedMsg{Paths: paths} })
	default:
		return splash
	}
}

// Update handles every message in the interface.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		height := msg.Height - 2
		if height < 1 {
			height = 1
		}
		a.viewport.SetWidth(msg.Width)
		a.viewport.SetHeight(height)

		var cmds []tea.Cmd
		for id, model := range a.screens {
			updated, cmd := model.Update(msg)
			a.screens[id] = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
		return a, tea.Batch(cmds...)

	case messages.NavigateMsg:
		return a.navigateTo(msg.Screen)

	case messages.BackMsg:
		return a.goBack()

	case messages.StatusMsg:
		a.status = msg.Text
		return a, nil

	case messages.FileSelectedMsg:
		return a, screens.LoadBook(msg.Path)

	case messages.BookLoadedMsg:
		if msg.Err != nil {
			// Let the picker show the error where the user is looking.
			return a.forward(msg)
		}
		a.book = msg.Book
		a.inputs = []string{msg.Path}
		a.output.Path = ""
		a.pushState()
		return a.navigateTo(messages.ScreenInfo)

	case messages.FilesSelectedMsg:
		a.inputs = msg.Paths
		a.pushState()
		nav, cmd := a.navigateTo(messages.ScreenBatchQueue)
		app := nav.(App)
		return app, tea.Batch(cmd, screens.LoadBatch(msg.Paths))

	case messages.PresetSelectedMsg:
		a.preset = msg.Preset
		if opts, err := msg.Preset.Options(); err == nil {
			a.pdf = opts
		}
		a.pushState()
		if len(a.inputs) > 1 {
			return a.navigateTo(messages.ScreenBatchQueue)
		}
		return a.navigateTo(messages.ScreenPreview)

	case messages.LayoutChangedMsg:
		a.applyLayout(msg)
		a.pushState()
		return a.goBack()

	case messages.ConvertStartMsg:
		nav, cmd := a.navigateTo(messages.ScreenConverting)
		app := nav.(App)
		conv := app.screens[messages.ScreenConverting].(screens.ConvertingModel)
		start := conv.Start(engine.Options{
			Input:  app.inputs[0],
			Output: app.output,
			PDF:    app.pdf,
		})
		app.screens[messages.ScreenConverting] = conv
		return app, tea.Batch(cmd, start)

	case messages.ConvertDoneMsg:
		if msg.Err != nil {
			return a.forward(msg)
		}
		a.result = msg.Result
		a.pushState()
		return a.navigateTo(messages.ScreenComplete)

	case messages.ConvertCancelMsg:
		return a.navigateTo(messages.ScreenPreview)

	case messages.BatchStartMsg:
		nav, cmd := a.navigateTo(messages.ScreenBatchProgress)
		app := nav.(App)
		bp := app.screens[messages.ScreenBatchProgress].(screens.BatchProgressModel)
		start := bp.Start(msg.Paths, app.output, app.pdf)
		app.screens[messages.ScreenBatchProgress] = bp
		return app, tea.Batch(cmd, start)

	case messages.BatchDoneMsg:
		if bp, ok := a.screens[messages.ScreenBatchProgress].(screens.BatchProgressModel); ok {
			if bc, ok := a.screens[messages.ScreenBatchComplete].(screens.BatchCompleteModel); ok {
				bc.SetResults(bp.Results(), bp.Elapsed())
				a.screens[messages.ScreenBatchComplete] = bc
			}
		}
		return a.navigateTo(messages.ScreenBatchComplete)

	case messages.ThemeToggleMsg:
		name := style.ToggleTheme()
		if a.cfg != nil {
			a.cfg.UI.Theme = string(name)
			_ = a.cfg.Save()
		}
		a.status = "Theme: " + string(name)
		return a, nil

	case tea.KeyPressMsg:
		if a.showHelp {
			switch msg.String() {
			case "?", "esc", "enter", "q", "up", "down", "left", "right", "tab":
				a.showHelp = false
			}
			return a, nil
		}
		switch {
		case key.Matches(msg, a.keys.Help):
			a.showHelp = true
			return a, nil
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit
		case key.Matches(msg, a.keys.ThemeToggle):
			return a, func() tea.Msg { return messages.ThemeToggleMsg{} }
		}
		a.status = ""
	}

	a.syncViewport()

	if _, isMouse := msg.(tea.MouseMsg); isMouse {
		var vpCmd tea.Cmd
		a.viewport, vpCmd = a.viewport.Update(msg)
		model, cmd := a.forwardCmd(msg)
		return model, tea.Batch(vpCmd, cmd)
	}

	model, cmd := a.forwardCmd(msg)
	app := model.(App)
	app.syncViewport()
	if _, isKey := msg.(tea.KeyPressMsg); isKey {
		var vpCmd tea.Cmd
		app.viewport, vpCmd = app.viewport.Update(msg)
		return app, tea.Batch(cmd, vpCmd)
	}
	return app, cmd
}

// forward hands a message to the current screen and returns the model.
func (a App) forward(msg tea.Msg) (tea.Model, tea.Cmd) {
	return a.forwardCmd(msg)
}

func (a App) forwardCmd(msg tea.Msg) (tea.Model, tea.Cmd) {
	model, ok := a.screens[a.current]
	if !ok {
		return a, nil
	}
	updated, cmd := model.Update(msg)
	a.screens[a.current] = updated
	return a, cmd
}

// pushState hands the shared conversion state to every screen that displays it.
func (a *App) pushState() {
	if m, ok := a.screens[messages.ScreenInfo].(screens.InfoModel); ok {
		m.SetBook(a.book)
		a.screens[messages.ScreenInfo] = m
	}
	if m, ok := a.screens[messages.ScreenPresets].(screens.PresetsModel); ok {
		m.SetBook(a.book)
		m.SetPreset(a.preset)
		a.screens[messages.ScreenPresets] = m
	}
	if m, ok := a.screens[messages.ScreenLayout].(screens.LayoutModel); ok {
		m.SetOptions(a.pdf)
		a.screens[messages.ScreenLayout] = m
	}
	if m, ok := a.screens[messages.ScreenPreview].(screens.PreviewModel); ok {
		m.SetState(a.book, a.preset, a.pdf, a.output)
		a.screens[messages.ScreenPreview] = m
	}
	if m, ok := a.screens[messages.ScreenConverting].(screens.ConvertingModel); ok {
		m.SetBook(a.book)
		a.screens[messages.ScreenConverting] = m
	}
	if m, ok := a.screens[messages.ScreenComplete].(screens.CompleteModel); ok {
		m.SetResult(a.result)
		a.screens[messages.ScreenComplete] = m
	}
	if m, ok := a.screens[messages.ScreenBatchQueue].(screens.BatchQueueModel); ok {
		m.SetPreset(a.preset)
		a.screens[messages.ScreenBatchQueue] = m
	}
}

func (a *App) applyLayout(msg messages.LayoutChangedMsg) {
	if size, err := pdfout.LookupPageSize(msg.PageSize); err == nil {
		a.pdf.PageSize = size
	}
	a.pdf.Margins = pdfout.UniformMargins(msg.MarginMM)
	a.pdf.Font = msg.Font
	a.pdf.FontSize = msg.FontSize
	a.pdf.LineSpacing = msg.LineSpacing
	a.pdf.Justify = msg.Justify
	a.pdf.Images = msg.Images
	a.pdf.Cover = msg.Cover
	a.pdf.TitlePage = msg.TitlePage
	a.pdf.PageNumbers = msg.PageNumbers
	a.pdf.RunningHeader = msg.Header
	a.pdf.Bookmarks = msg.Bookmarks
	a.pdf.ChapterBreaks = msg.Breaks
	a.pdf.Normalize()
}

func (a *App) syncViewport() {
	if model, ok := a.screens[a.current]; ok {
		a.viewport.SetContent(model.View())
	}
}

func (a App) navigateTo(screen messages.Screen) (tea.Model, tea.Cmd) {
	if screen == a.current {
		return a, nil
	}
	a.history = append(a.history, a.current)
	a.current = screen
	a.viewport.GotoTop()
	a.status = ""

	var cmds []tea.Cmd
	if a.width > 0 && a.height > 0 {
		if model, ok := a.screens[screen]; ok {
			updated, cmd := model.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height})
			a.screens[screen] = updated
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}
	if model, ok := a.screens[screen]; ok {
		if cmd := model.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return a, tea.Batch(cmds...)
}

func (a App) goBack() (tea.Model, tea.Cmd) {
	if len(a.history) == 0 {
		return a, nil
	}
	prev := a.history[len(a.history)-1]
	a.history = a.history[:len(a.history)-1]
	a.current = prev
	a.viewport.GotoTop()
	a.status = ""
	return a, nil
}

// View renders the header, the current screen and the footer.
func (a App) View() tea.View {
	if model, ok := a.screens[a.current]; ok {
		a.viewport.SetContent(model.View())
	}
	content := a.header() + "\n" + a.viewport.View() + "\n" + a.footer()
	if a.showHelp {
		content = a.helpOverlay(content)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (a App) header() string {
	version := a.version
	if version != "" && version[0] != 'v' {
		version = "v" + version
	}
	left := style.HeaderStyle().Render(fmt.Sprintf(" azw3-to-pdf %s ", version))
	right := style.HeaderStyle().Render(fmt.Sprintf(" %s ", a.current.String()))

	gap := a.width - style.Visible(left) - style.Visible(right)
	if gap < 0 {
		gap = 0
	}
	return left + style.HeaderStyle().Render(strings.Repeat(" ", gap)) + right
}

func (a App) footer() string {
	hints := a.status
	if hints == "" {
		hints = a.footerHints()
	}
	if a.viewport.TotalLineCount() > a.viewport.VisibleLineCount() {
		hints += fmt.Sprintf("  |  PgUp/PgDn %d%%", int(a.viewport.ScrollPercent()*100))
	}
	return style.FooterStyle().Width(a.width).Render(style.Truncate(hints, maxInt(a.width-2, 10)))
}

func (a App) footerHints() string {
	switch a.current {
	case messages.ScreenSplash:
		return "any key: continue  |  ?: help  |  ctrl+c: quit"
	case messages.ScreenFilePicker:
		return "enter: open/select  |  space: add to batch  |  tab: type a path  |  esc: back"
	case messages.ScreenInfo:
		return "enter: choose layout  |  c: convert now  |  esc: back"
	case messages.ScreenPresets:
		return "up/down: browse  |  enter: use preset  |  esc: back"
	case messages.ScreenLayout:
		return "up/down: setting  |  left/right: change  |  enter: apply  |  esc: cancel"
	case messages.ScreenPreview:
		return "enter: convert  |  l: layout  |  p: preset  |  esc: back"
	case messages.ScreenConverting:
		return "esc: cancel  |  ctrl+c: quit"
	case messages.ScreenComplete:
		return "o: open PDF  |  f: folder  |  n: another book  |  q: quit"
	case messages.ScreenBatchQueue:
		return "enter: start  |  d: remove  |  p: preset  |  l: layout  |  esc: back"
	case messages.ScreenBatchProgress:
		return "esc: stop after this book  |  ctrl+c: quit"
	case messages.ScreenBatchComplete:
		return "f: open folder  |  n: new batch  |  q: quit"
	}
	return "ctrl+c: quit"
}

// helpOverlay draws the shortcut list centred over the current screen.
func (a App) helpOverlay(base string) string {
	var b strings.Builder
	b.WriteString(style.TitleStyle().Render("Keyboard shortcuts"))
	b.WriteString("\n")
	b.WriteString(style.AccentStyle().Render("Everywhere"))
	b.WriteString("\n")
	for _, bind := range [][2]string{
		{"ctrl+c", "Quit"},
		{"ctrl+t", "Switch theme"},
		{"?", "Show or hide this"},
		{"esc", "Go back"},
		{"PgUp/PgDn", "Scroll"},
	} {
		fmt.Fprintf(&b, "  %s  %s\n", style.KeyHintStyle().Render(fmt.Sprintf("%-10s", bind[0])), bind[1])
	}
	b.WriteString("\n")
	b.WriteString(style.AccentStyle().Render(a.current.String()))
	b.WriteString("\n")
	for _, bind := range a.screenBindings() {
		fmt.Fprintf(&b, "  %s  %s\n", style.KeyHintStyle().Render(fmt.Sprintf("%-10s", bind[0])), bind[1])
	}
	b.WriteString("\n")
	b.WriteString(style.MutedStyle().Render("Press ? or esc to close"))

	width := 50
	if a.width > 0 && width > a.width-4 {
		width = a.width - 4
	}
	box := style.CardStyle().Width(width).Render(b.String())

	lines := strings.Split(base, "\n")
	boxLines := strings.Split(box, "\n")
	start := (len(lines) - len(boxLines)) / 2
	if start < 1 {
		start = 1
	}
	pad := 0
	if a.width > width+4 {
		pad = (a.width - width - 4) / 2
	}
	prefix := strings.Repeat(" ", pad)
	for i, line := range boxLines {
		if row := start + i; row < len(lines) {
			lines[row] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

func (a App) screenBindings() [][2]string {
	switch a.current {
	case messages.ScreenFilePicker:
		return [][2]string{
			{"enter", "Open a folder, or pick the book"},
			{"space", "Add a book to the batch"},
			{"b", "Convert the selected batch"},
			{"x", "Clear the batch"},
			{"tab", "Type a path instead"},
			{"d", "Switch drive (Windows)"},
			{"h", "Jump to your home folder"},
		}
	case messages.ScreenInfo:
		return [][2]string{
			{"enter", "Choose a layout preset"},
			{"c", "Convert with the current settings"},
		}
	case messages.ScreenPresets:
		return [][2]string{
			{"up/down", "Move through the presets"},
			{"enter", "Use this preset"},
			{"r", "Use the recommended preset"},
		}
	case messages.ScreenLayout:
		return [][2]string{
			{"up/down", "Choose a setting"},
			{"left/right", "Change its value"},
			{"space", "Toggle a yes/no setting"},
			{"enter", "Apply"},
			{"r", "Reset to the preset"},
		}
	case messages.ScreenPreview:
		return [][2]string{
			{"enter", "Start converting"},
			{"l", "Fine-tune the layout"},
			{"p", "Pick another preset"},
			{"o", "Change where the PDF goes"},
		}
	case messages.ScreenComplete:
		return [][2]string{
			{"o", "Open the PDF"},
			{"f", "Open the containing folder"},
			{"n", "Convert another book"},
			{"q", "Quit"},
		}
	case messages.ScreenBatchQueue:
		return [][2]string{
			{"up/down", "Move through the queue"},
			{"d", "Remove the highlighted book"},
			{"enter", "Start the batch"},
		}
	}
	return nil
}

func navigate(screen messages.Screen) tea.Cmd {
	return func() tea.Msg { return messages.NavigateMsg{Screen: screen} }
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
