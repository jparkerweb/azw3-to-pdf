// Package config loads and saves the user's persisted preferences.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the on-disk configuration file.
type Config struct {
	// Preset is the layout chosen by default.
	Preset string `yaml:"preset"`

	Output OutputConfig `yaml:"output"`
	Layout LayoutConfig `yaml:"layout"`
	UI     UIConfig     `yaml:"ui"`

	path string `yaml:"-"`
}

// OutputConfig controls where PDFs are written.
type OutputConfig struct {
	Dir      string `yaml:"dir"`
	Suffix   string `yaml:"suffix"`
	Conflict string `yaml:"conflict"` // fail, overwrite, rename, skip
}

// LayoutConfig holds overrides applied on top of the chosen preset.
type LayoutConfig struct {
	PageSize    string  `yaml:"page_size"`
	MarginMM    float64 `yaml:"margin_mm"`
	Font        string  `yaml:"font"`
	FontSize    float64 `yaml:"font_size"`
	LineSpacing float64 `yaml:"line_spacing"`
	Images      *bool   `yaml:"images"`
	Cover       *bool   `yaml:"cover"`
	TitlePage   *bool   `yaml:"title_page"`
	PageNumbers *bool   `yaml:"page_numbers"`
	Bookmarks   *bool   `yaml:"bookmarks"`
}

// UIConfig holds terminal-interface preferences.
type UIConfig struct {
	Theme string `yaml:"theme"`
}

// Default returns the configuration used when no file exists.
func Default() *Config {
	return &Config{
		Preset: "ereader",
		Output: OutputConfig{Suffix: "", Conflict: "fail"},
		UI:     UIConfig{Theme: "midnight-ink"},
	}
}

// Load reads the configuration, falling back to defaults when the file is
// absent. An explicit path overrides the standard location.
func Load(path string) (*Config, error) {
	cfg := Default()

	if path == "" {
		dir, err := Dir()
		if err != nil {
			return cfg, nil
		}
		path = filepath.Join(dir, "config.yaml")
	}
	cfg.path = path

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("reading %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return Default(), fmt.Errorf("parsing %s: %w", path, err)
	}
	cfg.path = path
	return cfg, nil
}

// Save writes the configuration back to disk.
func (c *Config) Save() error {
	path := c.path
	if path == "" {
		dir, err := Dir()
		if err != nil {
			return err
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		path = filepath.Join(dir, "config.yaml")
		c.path = path
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Path reports where the configuration lives.
func (c *Config) Path() string { return c.path }
