package config

import (
	"os"
	"path/filepath"
)

const appName = "azw3-to-pdf"

// envConfigDir lets a user (or a test) relocate the whole configuration tree.
const envConfigDir = "AZW3_TO_PDF_CONFIG"

// Dir returns the configuration directory.
func Dir() (string, error) {
	if dir := os.Getenv(envConfigDir); dir != "" {
		return dir, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appName), nil
}

// CacheDir returns the cache directory, which holds logs.
func CacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appName), nil
}

// LogDir returns the directory log files are written to.
func LogDir() (string, error) {
	cache, err := CacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "logs"), nil
}
