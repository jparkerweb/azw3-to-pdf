package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/jparkerweb/ebook-to-pdf/internal/config"
	"github.com/jparkerweb/ebook-to-pdf/internal/engine"
	"github.com/jparkerweb/ebook-to-pdf/internal/logging"
	"github.com/jparkerweb/ebook-to-pdf/internal/presets"
	"github.com/jparkerweb/ebook-to-pdf/internal/tui/style"
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
		return dryRun(inputs, outOpts, preset, cfg.Layout.PageSize)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Progress is only drawn when a person is watching; piping the output
	// should not fill the log with carriage returns.
	showProgress := term.IsTerminal(int(os.Stderr.Fd()))

	jobs := min(max(flagJobs, 1), len(inputs))

	batch := engine.BatchOptions{
		Output:      outOpts,
		PDF:         pdfOpts,
		Concurrency: jobs,
		OnResult: func(r engine.BatchResult) {
			if showProgress && jobs == 1 {
				fmt.Fprint(os.Stderr, "\r", strings.Repeat(" ", 48), "\r")
			}
			reportOne(r.Path, r.Result, r.Err)
		},
	}
	if showProgress && jobs == 1 {
		lastPrint := make([]time.Time, len(inputs))
		batch.OnProgress = func(i int, p engine.Progress) {
			if time.Since(lastPrint[i]) < 120*time.Millisecond {
				return
			}
			lastPrint[i] = time.Now()
			fmt.Fprintf(os.Stderr, "\r  %-18s %3.0f%%  page %-5d", p.Stage, p.Percent*100, p.Page)
		}
	}
	results := engine.RunBatch(ctx, inputs, batch)

	var converted, skipped, failed int
	var lastDir string
	for _, o := range results {
		switch {
		case o.Err == nil && o.Result != nil:
			converted++
			lastDir = filepath.Dir(o.Result.OutputPath)
		case errors.Is(o.Err, engine.ErrSkipped):
			skipped++
		case o.Err != nil:
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
			res.Pages, res.Images, res.Font, style.FormatBytes(res.OutputSize),
			res.Elapsed.Round(10*time.Millisecond))
	}
}

// dryRun reports what a conversion would produce without doing any work.
func dryRun(inputs []string, out engine.OutputOptions, preset presets.Preset, savedSize string) error {
	fmt.Printf("Preset: %s (%s)\n", preset.Name, preset.SummaryFor(savedSize))
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
