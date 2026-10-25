package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/jparkerweb/azw3-to-pdf/internal/ebook"
	"github.com/jparkerweb/azw3-to-pdf/internal/engine"
	"github.com/jparkerweb/azw3-to-pdf/internal/pdfout"
	"github.com/jparkerweb/azw3-to-pdf/internal/presets"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/messages"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/style"
)

// sampleBook is enough of a book for the screens to have something to show.
func sampleBook() *ebook.Book {
	return &ebook.Book{
		Path:        "library/example.azw3",
		Title:       "An Example Book",
		Authors:     []string{"A. Writer"},
		Publisher:   "Example Press",
		Published:   "2019-04-01",
		Language:    "en",
		Format:      "KF8 / AZW3 (MOBI v8)",
		Compression: "HUFF/CDIC",
		FileSize:    2 << 20,
		TextBytes:   4096,
		HTML:        `<html><body><p class="h1">Chapter One</p><p>Once upon a time.</p></body></html>`,
		Resources:   map[int]*ebook.Resource{},
	}
}

// drive pushes a message through the app and returns the updated model,
// discarding whatever command it asked for.
func drive(t *testing.T, app App, msg tea.Msg) App {
	t.Helper()
	next, _ := send(t, app, msg)
	return next
}

func send(t *testing.T, app App, msg tea.Msg) (App, tea.Cmd) {
	t.Helper()
	model, cmd := app.Update(msg)
	next, ok := model.(App)
	if !ok {
		t.Fatalf("Update returned %T, want tui.App", model)
	}
	return next, cmd
}

// press sends a key and then delivers the single message the screen asked
// for, which is how the real event loop behaves.
func press(t *testing.T, app App, key string) App {
	t.Helper()
	app, cmd := send(t, app, tea.KeyPressMsg{Code: rune(key[0]), Text: key})
	if cmd == nil {
		return app
	}
	if msg := cmd(); msg != nil {
		app = drive(t, app, msg)
	}
	return app
}

func newTestApp(t *testing.T) App {
	t.Helper()
	app := NewApp(AppOptions{
		Version: "test",
		Book:    sampleBook(),
		Inputs:  []string{"library/example.azw3"},
		Output:  engine.OutputOptions{Conflict: engine.ConflictOverwrite},
	})
	return drive(t, app, tea.WindowSizeMsg{Width: 100, Height: 30})
}

// TestScreensRender walks every screen and checks that it draws something.
func TestScreensRender(t *testing.T) {
	app := newTestApp(t)

	screens := []messages.Screen{
		messages.ScreenSplash,
		messages.ScreenFilePicker,
		messages.ScreenInfo,
		messages.ScreenPresets,
		messages.ScreenLayout,
		messages.ScreenPreview,
		messages.ScreenConverting,
		messages.ScreenComplete,
		messages.ScreenBatchQueue,
		messages.ScreenBatchProgress,
		messages.ScreenBatchComplete,
	}

	for _, screen := range screens {
		app = drive(t, app, messages.NavigateMsg{Screen: screen})
		view := app.View()
		if strings.TrimSpace(view.Content) == "" {
			t.Errorf("%s rendered nothing", screen)
		}
		if !strings.Contains(view.Content, screen.String()) {
			t.Errorf("%s header missing from its own view", screen)
		}
	}
}

// TestRegisteredScreens walks every registered Screen and checks that its
// footer hint, help bindings and state push all behave without panicking.
func TestRegisteredScreens(t *testing.T) {
	app := newTestApp(t)

	state := style.SharedState{
		Book:   sampleBook(),
		Preset: presets.Default(),
		PDF:    pdfout.DefaultOptions(),
		Output: engine.OutputOptions{Conflict: engine.ConflictOverwrite},
		Result: &engine.Result{},
	}

	for _, screen := range messages.Screens() {
		model, ok := app.screens[screen]
		if !ok {
			t.Errorf("%s has no registered screen", screen)
			continue
		}
		s, ok := model.(style.Screen)
		if !ok {
			t.Errorf("%s does not implement style.Screen", screen)
			continue
		}
		if hint := s.FooterHint(); hint == "" {
			t.Errorf("%s: FooterHint is empty", screen)
		}
		s.Bindings() // some screens legitimately add none beyond the global bindings
		if updated := s.PushState(state); updated == nil {
			t.Errorf("%s: PushState returned a nil model", screen)
		}
	}
}

// TestEveryScreenIsRegistered fails if a messages.Screen constant has no
// entry in App's screen registry, catching a forgotten registration.
func TestEveryScreenIsRegistered(t *testing.T) {
	app := newTestApp(t)
	for _, screen := range messages.Screens() {
		if _, ok := app.screens[screen]; !ok {
			t.Errorf("%s is defined but not registered in NewApp", screen)
		}
	}
}

// TestNavigationFlow follows the path a reader takes through the interface.
func TestNavigationFlow(t *testing.T) {
	app := newTestApp(t)

	app = drive(t, app, messages.BookLoadedMsg{Path: "library/example.azw3", Book: sampleBook()})
	if app.current != messages.ScreenInfo {
		t.Fatalf("after loading a book the app is on %s, want Book details", app.current)
	}

	app = press(t, app, "c")
	if app.current != messages.ScreenPreview {
		t.Fatalf("pressing c goes to %s, want Ready to convert", app.current)
	}

	app = drive(t, app, messages.BackMsg{})
	if app.current != messages.ScreenInfo {
		t.Fatalf("esc from the preview goes to %s, want Book details", app.current)
	}
}

// TestHelpOverlay checks that ? opens the shortcut list and Esc closes it.
func TestHelpOverlay(t *testing.T) {
	app := newTestApp(t)
	app = drive(t, app, tea.KeyPressMsg{Code: '?', Text: "?"})
	if !app.showHelp {
		t.Fatal("? did not open the help overlay")
	}
	if !strings.Contains(app.View().Content, "Keyboard shortcuts") {
		t.Error("the help overlay is not drawn over the screen")
	}
	app = drive(t, app, tea.KeyPressMsg{Code: tea.KeyEscape})
	if app.showHelp {
		t.Error("esc did not close the help overlay")
	}
}

// TestLayoutChangeReachesOptions checks that the fine-tuning screen's result
// is applied to the options used for conversion.
func TestLayoutChangeReachesOptions(t *testing.T) {
	app := newTestApp(t)
	letter, err := pdfout.LookupPageSize("letter")
	if err != nil {
		t.Fatal(err)
	}
	app = drive(t, app, messages.LayoutChangedMsg{Overrides: pdfout.OptionsOverrides{
		PageSize:    &letter,
		Margins:     pdfout.Ptr(pdfout.UniformMargins(30)),
		Font:        pdfout.Ptr("sans"),
		FontSize:    pdfout.Ptr(14.0),
		LineSpacing: pdfout.Ptr(1.8),
		Justify:     pdfout.Ptr(false),
		Images:      pdfout.Ptr(false),
	}})

	if app.pdf.PageSize.Name != "letter" {
		t.Errorf("page size is %q, want letter", app.pdf.PageSize.Name)
	}
	if app.pdf.FontSize != 14 {
		t.Errorf("font size is %v, want 14", app.pdf.FontSize)
	}
	if app.pdf.Images {
		t.Error("illustrations should have been turned off")
	}
	if got := app.pdf.Margins.Top * 25.4 / 72; got < 29.9 || got > 30.1 {
		t.Errorf("margin is %.2f mm, want 30", got)
	}
}
