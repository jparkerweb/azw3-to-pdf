package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeConvert swaps in a conversion that does no real work and restores the
// real one when the test ends.
func fakeConvert(t *testing.T, fn func(ctx context.Context, opts Options, onProgress func(Progress)) (*Result, error)) {
	t.Helper()
	orig := convertFn
	convertFn = fn
	t.Cleanup(func() { convertFn = orig })
}

func names(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = string(rune('a' + i))
	}
	return out
}

func TestRunBatchRespectsConcurrency(t *testing.T) {
	const limit = 3
	var running, peak atomic.Int32
	reached := make(chan struct{})
	var once sync.Once
	fakeConvert(t, func(ctx context.Context, opts Options, _ func(Progress)) (*Result, error) {
		n := running.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		if n == limit {
			once.Do(func() { close(reached) })
		}
		// Hold every book until the pool is full, so the limit is actually
		// reached; the timeout keeps a broken pool from hanging the test.
		select {
		case <-reached:
		case <-time.After(2 * time.Second):
		}
		running.Add(-1)
		return &Result{}, nil
	})

	got := RunBatch(context.Background(), names(8), BatchOptions{Concurrency: limit})
	if len(got) != 8 {
		t.Fatalf("got %d results, want 8", len(got))
	}
	if p := peak.Load(); p != limit {
		t.Errorf("peak concurrency %d, want exactly %d", p, limit)
	}
}

func TestRunBatchConcurrencyOneIsSequential(t *testing.T) {
	var order []string
	fakeConvert(t, func(ctx context.Context, opts Options, _ func(Progress)) (*Result, error) {
		order = append(order, opts.Input) // unsynchronised on purpose: -race flags overlap
		return &Result{}, nil
	})

	for _, c := range []int{1, 0, -4} {
		order = nil
		got := RunBatch(context.Background(), names(5), BatchOptions{Concurrency: c})
		for i, r := range got {
			if r.Index != i || r.Path != order[i] {
				t.Errorf("concurrency %d: result %d is %+v, ran %q", c, i, r, order[i])
			}
		}
		if len(order) != 5 || order[0] != "a" || order[4] != "e" {
			t.Errorf("concurrency %d: ran in order %v", c, order)
		}
	}
}

func TestRunBatchResultsAreInInputOrder(t *testing.T) {
	fakeConvert(t, func(ctx context.Context, opts Options, _ func(Progress)) (*Result, error) {
		if opts.Input == "a" {
			time.Sleep(40 * time.Millisecond) // finishes last
		}
		return &Result{OutputPath: opts.Input}, nil
	})

	got := RunBatch(context.Background(), names(4), BatchOptions{Concurrency: 4})
	for i, r := range got {
		if r.Index != i || r.Result.OutputPath != names(4)[i] {
			t.Errorf("result %d is %+v", i, r)
		}
	}
}

func TestRunBatchTalliesMixedOutcomes(t *testing.T) {
	boom := errors.New("boom")
	fakeConvert(t, func(ctx context.Context, opts Options, _ func(Progress)) (*Result, error) {
		switch opts.Input {
		case "b":
			return nil, boom
		case "c":
			return nil, ErrSkipped
		}
		return &Result{}, nil
	})

	var seen []string
	got := RunBatch(context.Background(), names(4), BatchOptions{
		OnResult: func(r BatchResult) { seen = append(seen, r.Path) },
	})
	if len(got) != 4 {
		t.Fatalf("got %d results, want one per path", len(got))
	}
	var ok, skipped, failed int
	for _, r := range got {
		switch {
		case errors.Is(r.Err, ErrSkipped):
			skipped++
		case r.Err != nil:
			failed++
		default:
			ok++
		}
	}
	if ok != 2 || skipped != 1 || failed != 1 {
		t.Errorf("tally %d converted, %d skipped, %d failed; want 2, 1, 1", ok, skipped, failed)
	}
	if len(seen) != 4 {
		t.Errorf("OnResult called %d times, want 4", len(seen))
	}
}

func TestRunBatchStopsStartingAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fakeConvert(t, func(ctx context.Context, opts Options, _ func(Progress)) (*Result, error) {
		if opts.Input == "b" {
			cancel()
		}
		return &Result{}, nil
	})

	got := RunBatch(ctx, names(5), BatchOptions{Concurrency: 1})
	if len(got) != 2 {
		t.Fatalf("got %d results after cancelling during the second book, want 2", len(got))
	}
	if got[1].Path != "b" {
		t.Errorf("the book running at cancel time should keep its result, got %q", got[1].Path)
	}
}

func TestRunBatchProgressCarriesTheIndex(t *testing.T) {
	fakeConvert(t, func(ctx context.Context, opts Options, onProgress func(Progress)) (*Result, error) {
		if onProgress != nil {
			onProgress(Progress{Page: 7})
		}
		return &Result{}, nil
	})

	var mu sync.Mutex
	seen := map[int]int{}
	RunBatch(context.Background(), names(3), BatchOptions{
		Concurrency: 2,
		OnProgress: func(i int, p Progress) {
			mu.Lock()
			seen[i] = p.Page
			mu.Unlock()
		},
	})
	for i := range 3 {
		if seen[i] != 7 {
			t.Errorf("no progress seen for book %d: %v", i, seen)
		}
	}
}

func TestRunBatchWithoutProgressPassesNil(t *testing.T) {
	fakeConvert(t, func(ctx context.Context, opts Options, onProgress func(Progress)) (*Result, error) {
		if onProgress != nil {
			t.Error("a progress callback was passed although none was asked for")
		}
		return &Result{}, nil
	})
	RunBatch(context.Background(), names(2), BatchOptions{})
}

func TestRunBatchEmpty(t *testing.T) {
	if got := RunBatch(context.Background(), nil, BatchOptions{Concurrency: 3}); len(got) != 0 {
		t.Errorf("got %d results for no paths", len(got))
	}
}

// TestRunBatchOutputConflicts runs the real conversion on two books that map
// to the same output name, to show the batch keeps Convert's conflict rules.
func TestRunBatchOutputConflicts(t *testing.T) {
	in := t.TempDir()
	out := t.TempDir()
	var paths []string
	for _, sub := range []string{"x", "y"} {
		dir := filepath.Join(in, sub)
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, writeTestBook(t, dir, "same.azw3", "Same", "<p>"+sub+"</p>"))
	}

	t.Run("rename", func(t *testing.T) {
		dir := t.TempDir()
		got := RunBatch(context.Background(), paths, BatchOptions{
			Output: OutputOptions{Dir: dir, Conflict: ConflictRename}, PDF: mustOptions(t), Concurrency: 1,
		})
		if len(got) != 2 || got[0].Err != nil || got[1].Err != nil {
			t.Fatalf("both books should convert: %+v", got)
		}
		if got[0].Result.OutputPath == got[1].Result.OutputPath {
			t.Errorf("both books were written to %s", got[0].Result.OutputPath)
		}
	})

	t.Run("skip", func(t *testing.T) {
		got := RunBatch(context.Background(), paths, BatchOptions{
			Output: OutputOptions{Dir: out, Conflict: ConflictSkip}, PDF: mustOptions(t), Concurrency: 1,
		})
		if len(got) != 2 || got[0].Err != nil || !errors.Is(got[1].Err, ErrSkipped) {
			t.Errorf("want the first converted and the second skipped: %+v", got)
		}
	})
}
