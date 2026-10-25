// Package messages defines the events passed between the interface's screens.
package messages

import (
	"github.com/jparkerweb/ebook-to-pdf/internal/ebook"
	"github.com/jparkerweb/ebook-to-pdf/internal/engine"
	"github.com/jparkerweb/ebook-to-pdf/internal/pdfout"
	"github.com/jparkerweb/ebook-to-pdf/internal/presets"
)

// Screen identifies a screen in the interface.
type Screen int

const (
	ScreenSplash Screen = iota
	ScreenFilePicker
	ScreenInfo
	ScreenPresets
	ScreenLayout
	ScreenPreview
	ScreenConverting
	ScreenComplete
	ScreenBatchQueue
	ScreenBatchProgress
	ScreenBatchComplete

	// screenCount is a sentinel, not a real screen: it lets tests iterate
	// every defined Screen without hand-maintaining a list that could fall
	// out of step with this block.
	screenCount
)

// Screens lists every defined Screen, in declaration order.
func Screens() []Screen {
	out := make([]Screen, 0, int(screenCount))
	for s := Screen(0); s < screenCount; s++ {
		out = append(out, s)
	}
	return out
}

// String returns the screen's title, shown in the header.
func (s Screen) String() string {
	switch s {
	case ScreenSplash:
		return "Welcome"
	case ScreenFilePicker:
		return "Choose a book"
	case ScreenInfo:
		return "Book details"
	case ScreenPresets:
		return "Layout preset"
	case ScreenLayout:
		return "Fine-tune layout"
	case ScreenPreview:
		return "Ready to convert"
	case ScreenConverting:
		return "Converting"
	case ScreenComplete:
		return "Finished"
	case ScreenBatchQueue:
		return "Batch queue"
	case ScreenBatchProgress:
		return "Converting batch"
	case ScreenBatchComplete:
		return "Batch finished"
	}
	return "ebook-to-pdf"
}

// NavigateMsg switches to another screen.
type NavigateMsg struct{ Screen Screen }

// BackMsg returns to the previous screen.
type BackMsg struct{}

// FileSelectedMsg reports that one book was chosen.
type FileSelectedMsg struct{ Path string }

// FilesSelectedMsg reports that several books were chosen.
type FilesSelectedMsg struct{ Paths []string }

// BookLoadedMsg carries a parsed book, or the error from trying.
type BookLoadedMsg struct {
	Path string
	Book *ebook.Book
	Err  error
}

// PresetSelectedMsg reports the chosen layout preset.
type PresetSelectedMsg struct{ Preset presets.Preset }

// LayoutChangedMsg carries fine-tuned layout settings back to the preview. It
// holds the overrides rather than one field per knob, so a new layout option
// does not need a new message field.
type LayoutChangedMsg struct{ Overrides pdfout.OptionsOverrides }

// SaveDefaultPageSizeMsg asks for a page size to be saved as the default for
// later runs.
type SaveDefaultPageSizeMsg struct{ Size pdfout.PageSize }

// ConvertStartMsg asks the converting screen to begin.
type ConvertStartMsg struct{}

// ConvertProgressMsg carries a progress update.
type ConvertProgressMsg struct{ Progress engine.Progress }

// ConvertDoneMsg reports a finished conversion.
type ConvertDoneMsg struct {
	Result *engine.Result
	Err    error
}

// ConvertCancelMsg reports that the user stopped a conversion.
type ConvertCancelMsg struct{}

// BatchStartMsg asks the batch screen to begin.
type BatchStartMsg struct{ Paths []string }

// BatchItemDoneMsg reports one finished book in a batch.
type BatchItemDoneMsg struct {
	Index  int
	Result *engine.Result
	Err    error
}

// BatchProgressMsg carries progress for the book currently converting.
type BatchProgressMsg struct {
	Index    int
	Progress engine.Progress
}

// BatchDoneMsg reports that the whole batch has finished.
type BatchDoneMsg struct{}

// ThemeToggleMsg switches the colour theme.
type ThemeToggleMsg struct{}

// StatusMsg shows a transient message in the footer.
type StatusMsg struct{ Text string }
