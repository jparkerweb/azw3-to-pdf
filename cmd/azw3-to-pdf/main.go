// Command azw3-to-pdf converts Kindle books into PDFs.
package main

import (
	"os"

	"github.com/jparkerweb/azw3-to-pdf/internal/cli"
)

// Build details, injected with -ldflags at release time.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cli.SetBuildInfo(version, commit, date)
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
