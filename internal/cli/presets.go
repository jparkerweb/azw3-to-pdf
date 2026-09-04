package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/jparkerweb/azw3-to-pdf/internal/pdfout"
	"github.com/jparkerweb/azw3-to-pdf/internal/presets"
)

func presetsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "presets",
		Short: "List the layout presets",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "KEY\tNAME\tLAYOUT")
			for _, p := range presets.All() {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", p.Key, p.Name, p.Summary())
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Println()
			for _, p := range presets.All() {
				fmt.Printf("  %-12s %s\n", p.Key, p.Description)
			}
			return nil
		},
	}
}

func pageSizesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "page-sizes",
		Short: "List the built-in page sizes",
		RunE: func(cmd *cobra.Command, args []string) error {
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tDESCRIPTION\tPOINTS")
			for _, s := range pdfout.PageSizes() {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%.0f x %.0f\n", s.Name, s.Label, s.Width, s.Height)
			}
			if err := w.Flush(); err != nil {
				return err
			}
			fmt.Println("\nA measurement also works: --page-size 120x160mm, 6x9in or 432x648pt.")
			return nil
		},
	}
}
