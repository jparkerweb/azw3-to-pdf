package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/jparkerweb/azw3-to-pdf/internal/config"
	"github.com/jparkerweb/azw3-to-pdf/internal/ebook"
	"github.com/jparkerweb/azw3-to-pdf/internal/engine"
	"github.com/jparkerweb/azw3-to-pdf/internal/logging"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui"
	"github.com/jparkerweb/azw3-to-pdf/internal/tui/style"
)

// runCmd is the default command: convert books, interactively or not.
func runCmd(cmd *cobra.Command, args []string) error {
	if err := validateFlags(cmd); err != nil {
		return err
	}

	inputs, err := collectInputs(args)
	if err != nil {
		return err
	}

	if flagNoTUI || flagDryRun || !isInteractive() {
		if len(inputs) == 0 {
			return fmt.Errorf("no book to convert: pass a file, or run without --no-tui to browse for one")
		}
		return runHeadless(cmd, inputs)
	}
	return runTUI(cmd, inputs)
}

// collectInputs gathers book paths from the positional arguments, -i, stdin
// and any directories among them.
func collectInputs(args []string) ([]string, error) {
	var raw []string
	if flagInput != "" {
		raw = append(raw, flagInput)
	}
	raw = append(raw, args...)

	if flagStdin {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			if line := strings.TrimSpace(scanner.Text()); line != "" {
				raw = append(raw, line)
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("reading paths from stdin: %w", err)
		}
	}

	var out []string
	seen := map[string]bool{}
	for _, p := range raw {
		p = strings.TrimSpace(strings.Trim(p, `"`))
		if p == "" {
			continue
		}
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("cannot read %s: %w", p, err)
		}
		var found []string
		if info.IsDir() {
			found = engine.DiscoverBooks(p, flagRecursive)
			if len(found) == 0 {
				return nil, fmt.Errorf("no Kindle books found in %s", p)
			}
		} else {
			if !engine.IsBookFile(p) {
				return nil, fmt.Errorf("%s is not a Kindle book (expected one of: %s)", filepath.Base(p), strings.Join(engine.BookExtensions, ", "))
			}
			found = []string{p}
		}
		for _, f := range found {
			abs, err := filepath.Abs(f)
			if err != nil {
				abs = f
			}
			if !seen[abs] {
				seen[abs] = true
				out = append(out, f)
			}
		}
	}

	if len(out) > 1 && flagOutput != "" {
		return nil, fmt.Errorf("--output names a single file; use --output-dir for %d books", len(out))
	}
	return out, nil
}

// runTUI starts the terminal interface, preloading any books named on the
// command line.
func runTUI(cmd *cobra.Command, inputs []string) error {
	if err := logging.Setup("tui", flagLogLevel); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	cfg, err := config.Load(flagConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	if cfg != nil && cfg.UI.Theme != "" {
		style.SetTheme(style.ThemeName(cfg.UI.Theme))
	}

	preset := resolvePreset(cfg)
	pdfOpts, err := buildPDFOptions(cmd, cfg, preset)
	if err != nil {
		return err
	}

	opts := tui.AppOptions{
		Version: version,
		Inputs:  inputs,
		Preset:  preset,
		PDF:     pdfOpts,
		Output:  buildOutputOptions(cfg, len(inputs) == 1),
		Config:  cfg,
	}

	// A single book named on the command line skips straight to its details.
	if len(inputs) == 1 {
		book, err := ebook.Open(inputs[0])
		if err != nil {
			return err
		}
		opts.Book = book
	}

	p := tea.NewProgram(tui.NewApp(opts))
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("terminal interface: %w", err)
	}
	return nil
}

// isInteractive reports whether it makes sense to draw a full-screen UI.
func isInteractive() bool {
	if os.Getenv("AZW3_TO_PDF_NO_TUI") != "" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stdin.Fd()))
}
