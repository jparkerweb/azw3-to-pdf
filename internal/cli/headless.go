package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/jparkerweb/azw3-to-pdf/internal/config"
	"github.com/jparkerweb/azw3-to-pdf/internal/engine"
	"github.com/jparkerweb/azw3-to-pdf/internal/logging"
	"github.com/jparkerweb/azw3-to-pdf/internal/presets"
)

// runHeadless converts books from the shell, printing progress as plain text.
func runHeadless(cmd *cobra.Command, inputs []string) error {
	if err := logging.Setup("headless", flagLogLevel); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	cfg, err := config.Load(flagConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	preset := resolvePreset(cfg)
	pdfOpts, err := buildPDFOptions(cmd, cfg, preset)
	if err != nil {
		return err
	}
	outOpts := buildOutputOptions(cfg, len(inputs) == 1)

	if flagDryRun {
		return dryRun(inputs, outOpts, preset)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Progress is only drawn when a person is watching; piping the output
	// should not fill the log with carriage returns.
	showProgress := term.IsTerminal(int(os.Stderr.Fd()))

	jobs := flagJobs
	if jobs < 1 {
		jobs = 1
	}
	if jobs > len(inputs) {
		jobs = len(inputs)
	}

	type outcome struct {
		index  int
		input  string
		result *engine.Result
		err    error
	}

	results := make([]outcome, len(inputs))
	var wg sync.WaitGroup
	var mu sync.Mutex
	queue := make(chan int)

	worker := func() {
		defer wg.Done()
		for i := range queue {
			input := inputs[i]
			opts := engine.Options{Input: input, Output: outOpts, PDF: pdfOpts}

			var lastPrint time.Time
			var onProgress func(engine.Progress)
			if showProgress && jobs == 1 {
				onProgress = func(p engine.Progress) {
					if time.Since(lastPrint) < 120*time.Millisecond {
						return
					}
					lastPrint = time.Now()
					fmt.Fprintf(os.Stderr, "\r  %-18s %3.0f%%  page %-5d", p.Stage, p.Percent*100, p.Page)
				}
			}

			res, err := engine.Convert(ctx, opts, onProgress)
			mu.Lock()
			if onProgress != nil {
				fmt.Fprint(os.Stderr, "\r", strings.Repeat(" ", 48), "\r")
			}
			results[i] = outcome{index: i, input: input, result: res, err: err}
			reportOne(input, res, err)
			mu.Unlock()
		}
	}

	wg.Add(jobs)
	for range jobs {
		go worker()
	}
	for i := range inputs {
		select {
		case queue <- i:
		case <-ctx.Done():
		}
	}
	close(queue)
	wg.Wait()

	var converted, skipped, failed int
	var lastDir string
	for _, o := range results {
		switch {
		case o.err == nil && o.result != nil:
			converted++
			lastDir = filepath.Dir(o.result.OutputPath)
		case errors.Is(o.err, engine.ErrSkipped):
			skipped++
		case o.err != nil:
			failed++
		}
	}

	if len(inputs) > 1 {
		fmt.Printf("\n%d converted, %d skipped, %d failed\n", converted, skipped, failed)
	}
	if flagOpen && lastDir != "" {
		if err := engine.OpenFolder(lastDir); err != nil {
			fmt.Fprintf(os.Stderr, "could not open %s: %v\n", lastDir, err)
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d books failed to convert", failed, len(inputs))
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

func reportOne(input string, res *engine.Result, err error) {
	name := filepath.Base(input)
	switch {
	case errors.Is(err, engine.ErrSkipped):
		fmt.Printf("- %s: already converted, skipped\n", name)
	case err != nil:
		fmt.Fprintf(os.Stderr, "x %s: %v\n", name, err)
	default:
		fmt.Printf("+ %s\n", res.OutputPath)
		fmt.Printf("  %d pages · %d images · %s · %s in %s\n",
			res.Pages, res.Images, res.Font, humanBytes(res.OutputSize),
			res.Elapsed.Round(10*time.Millisecond))
	}
}

// dryRun reports what a conversion would produce without doing any work.
func dryRun(inputs []string, out engine.OutputOptions, preset presets.Preset) error {
	fmt.Printf("Preset: %s (%s)\n", preset.Name, preset.Summary())
	for _, in := range inputs {
		path, err := engine.ResolveOutput(in, out)
		switch {
		case errors.Is(err, engine.ErrSkipped):
			fmt.Printf("- %s -> %s (exists, would skip)\n", filepath.Base(in), path)
		case err != nil:
			fmt.Printf("x %s -> %v\n", filepath.Base(in), err)
		default:
			fmt.Printf("+ %s -> %s\n", filepath.Base(in), path)
		}
	}
	return nil
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit && exp < 3; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGT"[exp])
}
