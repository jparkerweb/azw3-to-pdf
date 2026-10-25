package screens

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jparkerweb/azw3-to-pdf/internal/engine"
	"github.com/jparkerweb/azw3-to-pdf/internal/pdfout"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/messages"
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
