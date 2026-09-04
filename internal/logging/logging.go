// Package logging configures the process-wide logger.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"

	"github.com/jparkerweb/azw3-to-pdf/internal/config"
)

// envLogLevel overrides the level passed to Setup.
const envLogLevel = "AZW3_TO_PDF_LOG_LEVEL"

// Setup initialises logging. In "tui" mode the log goes to a file, because
// anything written to the terminal would corrupt the interface; in "headless"
// mode it goes to stderr.
func Setup(mode, level string) error {
	if env := os.Getenv(envLogLevel); env != "" {
		level = env
	}

	var w io.Writer = os.Stderr
	if mode == "tui" {
		dir, err := config.LogDir()
		if err != nil {
			return fmt.Errorf("locating log directory: %w", err)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating log directory: %w", err)
		}
		f, err := os.Create(filepath.Join(dir, "azw3-to-pdf.log"))
		if err != nil {
			return fmt.Errorf("creating log file: %w", err)
		}
		w = f
	}

	slog.SetDefault(slog.New(log.NewWithOptions(w, log.Options{
		ReportTimestamp: true,
		Level:           parseLevel(level),
	})))
	return nil
}

func parseLevel(level string) log.Level {
	switch strings.ToLower(level) {
	case "debug":
		return log.DebugLevel
	case "warn", "warning":
		return log.WarnLevel
	case "error":
		return log.ErrorLevel
	default:
		return log.InfoLevel
	}
}
