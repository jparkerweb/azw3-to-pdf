package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "absent.yaml"))
	if err != nil {
		t.Fatalf("Load returned %v for a missing file", err)
	}
	if cfg.Preset != "ereader" {
		t.Errorf("default preset is %q", cfg.Preset)
	}
}

func TestSaveAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Preset = "paperback"
	cfg.UI.Theme = "paper-sepia"
	cfg.Layout.FontSize = 13
	images := false
	cfg.Layout.Images = &images
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save returned %v", err)
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned %v", err)
	}
	if reloaded.Preset != "paperback" || reloaded.UI.Theme != "paper-sepia" {
		t.Errorf("reloaded config is %+v", reloaded)
	}
	if reloaded.Layout.FontSize != 13 {
		t.Errorf("font size came back as %v", reloaded.Layout.FontSize)
	}
	if reloaded.Layout.Images == nil || *reloaded.Layout.Images {
		t.Error("an explicit false for images was not preserved")
	}
}

func TestLoadRejectsBrokenYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("preset: [unclosed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Error("Load accepted a broken configuration file")
	}
}

func TestDirRespectsEnvironment(t *testing.T) {
	t.Setenv(envConfigDir, filepath.Join("custom", "place"))
	got, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join("custom", "place") {
		t.Errorf("Dir() = %q", got)
	}
}
