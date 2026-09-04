package screens

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"charm.land/bubbles/v2/filepicker"
	tea "charm.land/bubbletea/v2"

	"github.com/jparkerweb/azw3-to-pdf/internal/engine"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/messages"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/style"
)

// FilePickerModel is the built-in file browser.
type FilePickerModel struct {
	picker   filepicker.Model
	selected map[string]bool
	order    []string

	typing    bool
	typed     string
	errMsg    string
	recursive bool

	showDrives    bool
	drives        []string
	driveSelected int

	width  int
	height int
}

// NewFilePickerModel builds the file browser, starting in the working
// directory.
func NewFilePickerModel() FilePickerModel {
	fp := filepicker.New()
	fp.AllowedTypes = append([]string{}, engine.BookExtensions...)
	fp.SetHeight(15)
	fp.ShowHidden = false
	if cwd, err := os.Getwd(); err == nil {
		fp.CurrentDirectory = cwd
	}

	return FilePickerModel{
		picker:   fp,
		selected: map[string]bool{},
	}
}

// Init starts the browser reading the current directory.
func (m FilePickerModel) Init() tea.Cmd { return m.picker.Init() }

// Update handles browsing, multi-select and the typed-path mode.
func (m FilePickerModel) Update(msg tea.Msg) (style.ScreenModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		h := m.height - 12
		if h < 5 {
			h = 5
		}
		m.picker.SetHeight(h)
		return m, nil

	case messages.BookLoadedMsg:
		if msg.Err != nil {
			m.errMsg = msg.Err.Error()
		}
		return m, nil

	case tea.KeyPressMsg:
		if m.showDrives {
			return m.handleDrives(msg)
		}
		if m.typing {
			return m.handleTyping(msg)
		}

		switch msg.String() {
		case "esc":
			if len(m.selected) > 0 {
				m.selected = map[string]bool{}
				m.order = nil
				return m, status("Selection cleared")
			}
			return m, back()

		case "tab":
			m.typing = true
			m.errMsg = ""
			return m, nil

		case "h":
			if home, err := os.UserHomeDir(); err == nil {
				m.picker.CurrentDirectory = home
				return m, m.picker.Init()
			}
			return m, nil

		case "d":
			if drives := detectDrives(); len(drives) > 0 {
				m.drives = drives
				m.showDrives = true
				current := filepath.VolumeName(m.picker.CurrentDirectory) + string(filepath.Separator)
				for i, d := range drives {
					if strings.EqualFold(d, current) {
						m.driveSelected = i
					}
				}
				return m, nil
			}
			return m, nil

		case " ":
			path := m.picker.HighlightedPath()
			if path == "" || !engine.IsBookFile(path) {
				return m, nil
			}
			m.toggle(path)
			m.errMsg = ""
			return m, nil

		case "a":
			// Add every book in the folder being browsed.
			found := engine.DiscoverBooks(m.picker.CurrentDirectory, false)
			for _, f := range found {
				if !m.selected[f] {
					m.toggle(f)
				}
			}
			if len(found) == 0 {
				m.errMsg = "No books in this folder"
			}
			return m, nil

		case "x":
			m.selected = map[string]bool{}
			m.order = nil
			return m, nil

		case "b":
			if len(m.order) == 0 {
				return m, nil
			}
			return m, m.confirm()
		}
	}

	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)

	if picked, path := m.picker.DidSelectFile(msg); picked {
		if len(m.order) > 0 {
			// Enter adds to an in-progress batch rather than replacing it.
			if !m.selected[path] {
				m.toggle(path)
			}
			return m, nil
		}
		m.errMsg = ""
		return m, tea.Batch(
			func() tea.Msg { return messages.FileSelectedMsg{Path: path} },
			status("Reading %s", filepath.Base(path)),
		)
	}
	if rejected, path := m.picker.DidSelectDisabledFile(msg); rejected {
		m.errMsg = fmt.Sprintf("%s is not a Kindle book", filepath.Base(path))
		return m, nil
	}
	return m, cmd
}

// confirm hands the selection to the app: one book goes to its details, more
// than one starts a batch.
func (m FilePickerModel) confirm() tea.Cmd {
	paths := append([]string{}, m.order...)
	if len(paths) == 1 {
		return func() tea.Msg { return messages.FileSelectedMsg{Path: paths[0]} }
	}
	return func() tea.Msg { return messages.FilesSelectedMsg{Paths: paths} }
}

func (m *FilePickerModel) toggle(path string) {
	if m.selected[path] {
		delete(m.selected, path)
		for i, p := range m.order {
			if p == path {
				m.order = append(m.order[:i], m.order[i+1:]...)
				break
			}
		}
		return
	}
	m.selected[path] = true
	m.order = append(m.order, path)
}

func (m FilePickerModel) handleDrives(msg tea.KeyPressMsg) (style.ScreenModel, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.driveSelected > 0 {
			m.driveSelected--
		}
	case "down", "j":
		if m.driveSelected < len(m.drives)-1 {
			m.driveSelected++
		}
	case "enter":
		m.picker.CurrentDirectory = m.drives[m.driveSelected]
		m.showDrives = false
		return m, m.picker.Init()
	case "esc", "d":
		m.showDrives = false
	}
	return m, nil
}

func (m FilePickerModel) handleTyping(msg tea.KeyPressMsg) (style.ScreenModel, tea.Cmd) {
	switch msg.String() {
	case "esc", "tab":
		m.typing = false
		return m, nil

	case "enter":
		input := strings.TrimSpace(m.typed)
		if input == "" {
			m.errMsg = "Type a file or folder path"
			return m, nil
		}
		var paths []string
		for _, part := range strings.Split(input, ",") {
			part = strings.TrimSpace(strings.Trim(part, `"`))
			if part == "" {
				continue
			}
			info, err := os.Stat(part)
			if err != nil {
				m.errMsg = fmt.Sprintf("Cannot open %s", part)
				return m, nil
			}
			if info.IsDir() {
				found := engine.DiscoverBooks(part, m.recursive)
				if len(found) == 0 {
					m.errMsg = fmt.Sprintf("No Kindle books in %s", part)
					return m, nil
				}
				paths = append(paths, found...)
				continue
			}
			if !engine.IsBookFile(part) {
				m.errMsg = fmt.Sprintf("%s is not a Kindle book", filepath.Base(part))
				return m, nil
			}
			paths = append(paths, part)
		}
		m.errMsg = ""
		m.typing = false
		if len(paths) == 1 {
			return m, func() tea.Msg { return messages.FileSelectedMsg{Path: paths[0]} }
		}
		return m, func() tea.Msg { return messages.FilesSelectedMsg{Paths: paths} }

	case "backspace":
		if r := []rune(m.typed); len(r) > 0 {
			m.typed = string(r[:len(r)-1])
		}
		return m, nil

	case "ctrl+u":
		m.typed = ""
		return m, nil
	}

	if s := msg.String(); len([]rune(s)) == 1 {
		m.typed += s
	}
	return m, nil
}

// SetRecursive controls whether typed directories are searched recursively.
func (m *FilePickerModel) SetRecursive(v bool) { m.recursive = v }

// View renders the browser.
func (m FilePickerModel) View() string {
	var b strings.Builder
	b.WriteString(style.TitleStyle().Render("Choose a book"))
	b.WriteString("\n")

	switch {
	case m.showDrives:
		b.WriteString(style.SubtitleStyle().Render("Drives"))
		b.WriteString("\n\n")
		for i, d := range m.drives {
			if i == m.driveSelected {
				b.WriteString(style.SelectedStyle().Render("> " + d))
			} else {
				b.WriteString(style.MutedStyle().Render("  " + d))
			}
			b.WriteString("\n")
		}

	case m.typing:
		b.WriteString(style.SubtitleStyle().Render("Type a book, a folder, or several separated by commas"))
		b.WriteString("\n\n")
		width := m.width - 6
		if width < 20 {
			width = 20
		}
		b.WriteString(style.CardStyle().Width(width).Render("> " + m.typed + "▌"))
		b.WriteString("\n")

	default:
		b.WriteString(style.MutedStyle().Render(style.Truncate(m.picker.CurrentDirectory, maxInt(m.width-4, 20))))
		b.WriteString("\n")
		b.WriteString(m.picker.View())
		b.WriteString("\n")
	}

	if m.errMsg != "" {
		b.WriteString("\n")
		b.WriteString(style.ErrorStyle().Render(m.errMsg))
		b.WriteString("\n")
	}

	if n := len(m.order); n > 0 {
		b.WriteString("\n")
		b.WriteString(style.SuccessStyle().Render(fmt.Sprintf("%d book(s) queued", n)))
		b.WriteString(style.MutedStyle().Render("   b: convert them   x: clear"))
		b.WriteString("\n")
		shown := m.order
		if len(shown) > 6 {
			shown = shown[:6]
		}
		for _, p := range shown {
			b.WriteString(style.AccentStyle().Render("  · " + filepath.Base(p)))
			b.WriteString("\n")
		}
		if len(m.order) > len(shown) {
			b.WriteString(style.MutedStyle().Render(fmt.Sprintf("  · and %d more", len(m.order)-len(shown))))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// detectDrives lists the drive letters that exist on Windows.
func detectDrives() []string {
	if runtime.GOOS != "windows" {
		return nil
	}
	var drives []string
	for letter := 'A'; letter <= 'Z'; letter++ {
		root := string(letter) + `:\`
		if _, err := os.Stat(root); err == nil {
			drives = append(drives, root)
		}
	}
	sort.Strings(drives)
	return drives
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
