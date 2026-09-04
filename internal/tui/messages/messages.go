// Package messages defines the events passed between the interface's screens.
package messages

import (
	"github.com/jparkerweb/azw3-to-pdf/internal/ebook"
	"github.com/jparkerweb/azw3-to-pdf/internal/engine"
	"github.com/jparkerweb/azw3-to-pdf/internal/presets"
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
)

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
	return "azw3-to-pdf"
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

// LayoutChangedMsg carries fine-tuned layout settings back to the preview.
type LayoutChangedMsg struct {
	PageSize    string
	MarginMM    float64
	Font        string
	FontSize    float64
	LineSpacing float64
	Justify     bool
	Images      bool
	Cover       bool
	TitlePage   bool
	PageNumbers bool
	Header      bool
	Bookmarks   bool
	Breaks      bool
}

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
