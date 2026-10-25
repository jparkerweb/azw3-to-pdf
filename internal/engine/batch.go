package engine

import (
	"context"
	"sync"

	"github.com/jparkerweb/ebook-to-pdf/internal/pdfout"
)

// BatchOptions configures RunBatch.
type BatchOptions struct {
	Output OutputOptions
	PDF    pdfout.Options

	// Concurrency is how many books convert at once; anything below 1 means 1.
	Concurrency int

	// Stop, once closed, keeps any new book from starting while the books
	// already in flight run to completion. Cancelling ctx aborts those too.
	// Nil means the batch only stops with ctx.
	Stop <-chan struct{}

	// OnProgress reports one book's progress. With Concurrency above 1 it is
	// called from several goroutines, each for a different index, so keep any
	// per-book state indexed by it. Nil means no progress is wanted.
	OnProgress func(index int, p Progress)

	// OnResult is called once per finished book, one call at a time and in
	// completion order. Nil is fine.
	OnResult func(BatchResult)
}

// BatchResult is what happened to one book of a batch.
type BatchResult struct {
	Index  int
	Path   string
	Result *Result
	Err    error
}

// convertFn is Convert, replaceable in tests.
var convertFn = Convert

// RunBatch converts paths with up to opts.Concurrency books in flight and
// returns the books it attempted, in input order. A failure never stops the
// others. Once ctx is cancelled or opts.Stop is closed no new book starts, so
// a stopped batch returns fewer results than paths.
func RunBatch(ctx context.Context, paths []string, opts BatchOptions) []BatchResult {
	workers := min(max(opts.Concurrency, 1), max(len(paths), 1))

	results := make([]BatchResult, len(paths))
	attempted := make([]bool, len(paths))
	queue := make(chan int)

	var wg sync.WaitGroup
	var mu sync.Mutex

	stopped := func() bool {
		if ctx.Err() != nil {
			return true
		}
		select {
		case <-opts.Stop:
			return true
		default:
			return false
		}
	}

	worker := func() {
		defer wg.Done()
		for i := range queue {
			// A book can be handed over just as the batch is stopped.
			if stopped() {
				continue
			}
			var onProgress func(Progress)
			if opts.OnProgress != nil {
				onProgress = func(p Progress) { opts.OnProgress(i, p) }
			}

			res, err := convertFn(ctx, Options{Input: paths[i], Output: opts.Output, PDF: opts.PDF}, onProgress)
			r := BatchResult{Index: i, Path: paths[i], Result: res, Err: err}
			results[i], attempted[i] = r, true

			if opts.OnResult != nil {
				mu.Lock()
				opts.OnResult(r)
				mu.Unlock()
			}
		}
	}

	wg.Add(workers)
	for range workers {
		go worker()
	}
dispatch:
	for i := range paths {
		if stopped() {
			break
		}
		select {
		case queue <- i:
		case <-ctx.Done():
			break dispatch
		case <-opts.Stop:
			break dispatch
		}
	}
	close(queue)
	wg.Wait()

	out := make([]BatchResult, 0, len(paths))
	for i, ok := range attempted {
		if ok {
			out = append(out, results[i])
		}
	}
	return out
}
