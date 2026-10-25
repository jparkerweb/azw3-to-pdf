package screens

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/jparkerweb/ebook-to-pdf/internal/engine"
	"github.com/jparkerweb/ebook-to-pdf/internal/pdfout"
	"github.com/jparkerweb/ebook-to-pdf/internal/tui/messages"
)

// TestBatchStartRunsOneBookAtATime checks that the interface hands the shared
// runner a concurrency of 1 and drops any single-file output path.
func TestBatchStartRunsOneBookAtATime(t *testing.T) {
	got := make(chan engine.BatchOptions, 1)
	orig := runBatch
	runBatch = func(ctx context.Context, paths []string, opts engine.BatchOptions) []engine.BatchResult {
		got <- opts
		return nil
	}
	t.Cleanup(func() { runBatch = orig })

	m := NewBatchProgressModel()
	m.Start([]string{"a.azw3", "b.azw3"}, engine.OutputOptions{Path: "single.pdf"}, pdfout.DefaultOptions())

	select {
	case opts := <-got:
		if opts.Concurrency != 1 {
			t.Errorf("concurrency is %d, want 1", opts.Concurrency)
		}
		if opts.Output.Path != "" {
			t.Errorf("output path %q should have been cleared for a batch", opts.Output.Path)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the batch runner was never called")
	}
}

// TestBatchStartReportsThroughChannels drives Start with a fake runner that
// reports progress and results, and checks what reaches the screen's channels.
func TestBatchStartReportsThroughChannels(t *testing.T) {
	orig := runBatch
	runBatch = func(ctx context.Context, paths []string, opts engine.BatchOptions) []engine.BatchResult {
		// Nobody drains updates, so these must not block (one-slot channel,
		// non-blocking send).
		opts.OnProgress(0, engine.Progress{Page: 1})
		opts.OnProgress(0, engine.Progress{Page: 2})
		opts.OnProgress(1, engine.Progress{Page: 3})
		opts.OnResult(engine.BatchResult{Index: 0, Path: paths[0], Result: &engine.Result{Pages: 4}})
		opts.OnResult(engine.BatchResult{Index: 1, Path: paths[1], Err: errors.New("boom")})
		return nil
	}
	t.Cleanup(func() { runBatch = orig })

	m := NewBatchProgressModel()
	m.Start([]string{"a.azw3", "b.azw3"}, engine.OutputOptions{}, pdfout.DefaultOptions())

	next := func() any {
		select {
		case msg := <-m.events:
			return msg
		case <-time.After(2 * time.Second):
			t.Fatal("timed out waiting for a batch message")
			return nil
		}
	}

	first, ok := next().(messages.BatchItemDoneMsg)
	if !ok || first.Index != 0 || first.Result == nil || first.Result.Pages != 4 {
		t.Errorf("first message is %+v", first)
	}
	second, ok := next().(messages.BatchItemDoneMsg)
	if !ok || second.Index != 1 || second.Err == nil {
		t.Errorf("second message is %+v", second)
	}
	if _, ok := next().(messages.BatchDoneMsg); !ok {
		t.Error("the last message should be BatchDoneMsg")
	}
}

// TestBatchEscStopsAfterTheCurrentBook checks that esc closes the runner's
// stop signal and leaves the book in flight running, as the status line says.
func TestBatchEscStopsAfterTheCurrentBook(t *testing.T) {
	type call struct {
		ctx  context.Context
		stop <-chan struct{}
	}
	got := make(chan call, 1)
	orig := runBatch
	runBatch = func(ctx context.Context, paths []string, opts engine.BatchOptions) []engine.BatchResult {
		got <- call{ctx, opts.Stop}
		return nil
	}
	t.Cleanup(func() { runBatch = orig })

	m := NewBatchProgressModel()
	m.Start([]string{"a.azw3", "b.azw3"}, engine.OutputOptions{}, pdfout.DefaultOptions())

	var c call
	select {
	case c = <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("the batch runner was never called")
	}

	m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	select {
	case <-c.stop:
	default:
		t.Error("esc did not close the stop signal")
	}
	if c.ctx.Err() != nil {
		t.Error("esc cancelled the context, which aborts the book in flight")
	}
}

// TestTypedFolderSkipsText checks that a typed folder follows the CLI scan
// rule: ebooks are picked up, text and HTML files are not.
func TestTypedFolderSkipsText(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"book.epub", "notes.txt", "page.html"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m := NewFilePickerModel()
	m.typing, m.typed = true, dir
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("entering a folder with a book in it should select it")
	}
	msg, ok := cmd().(messages.FileSelectedMsg)
	if !ok || filepath.Base(msg.Path) != "book.epub" {
		t.Errorf("selected %+v, want only book.epub", msg)
	}

	textOnly := t.TempDir()
	if err := os.WriteFile(filepath.Join(textOnly, "notes.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m = NewFilePickerModel()
	m.typing, m.typed = true, textOnly
	next, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := next.(FilePickerModel).errMsg; got != "No ebooks in "+textOnly {
		t.Errorf("error is %q", got)
	}
}
