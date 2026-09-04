package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/jparkerweb/azw3-to-pdf/internal/config"
)

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			short := commit
			if len(short) > 7 {
				short = short[:7]
			}
			fmt.Printf("azw3-to-pdf %s (commit: %s, built: %s)\n", version, short, date)
			fmt.Printf("  go:      %s\n", runtime.Version())
			fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)

			if dir, err := config.Dir(); err == nil {
				fmt.Printf("  config:  %s\n", dir)
			}
			if dir, err := config.LogDir(); err == nil {
				fmt.Printf("  logs:    %s\n", dir)
			}
			return nil
		},
	}
}
