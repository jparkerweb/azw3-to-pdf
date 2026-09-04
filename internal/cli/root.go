// Package cli wires up the command line, and hands off to either the terminal
// interface or the headless converter.
package cli

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

// Build information, injected via ldflags and forwarded from main.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// SetBuildInfo records the build details printed by `version`.
func SetBuildInfo(v, c, d string) {
	version, commit, date = v, c, d
}

// Global flags.
var (
	flagConfig   string
	flagLogLevel string
	flagNoColor  bool
)

// Execute runs the command line. It is the entry point used by main.
func Execute() error {
	root := &cobra.Command{
		Use:   "azw3-to-pdf [book...]",
		Short: "Turn Kindle books into PDFs.",
		Long: "azw3-to-pdf converts Kindle books (.azw3, .azw, .mobi, .prc) into PDFs.\n\n" +
			"Run it with no arguments for the interactive browser, or pass files and\n" +
			"--no-tui to convert straight from the shell. It needs no other software:\n" +
			"the parser, layout engine and PDF writer are all built in.",
		Args:              cobra.ArbitraryArgs,
		RunE:              runCmd,
		ValidArgsFunction: completeBookFiles,
	}

	root.PersistentFlags().StringVar(&flagConfig, "config", "", "path to the configuration file")
	root.PersistentFlags().StringVar(&flagLogLevel, "log-level", "info", "log level (debug, info, warn, error)")
	root.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "disable coloured output")

	registerRunFlags(root)
	registerCompletions(root)

	root.AddCommand(versionCmd())
	root.AddCommand(presetsCmd())
	root.AddCommand(pageSizesCmd())
	root.AddCommand(probeCmd())
	root.AddCommand(completionCmd())

	return fang.Execute(
		context.Background(),
		root,
		fang.WithVersion(version),
		fang.WithCommit(commit),
		fang.WithNotifySignal(os.Interrupt),
	)
}
