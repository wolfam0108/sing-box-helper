package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadSettings reads /opt/etc/singbox-helper/config.yaml (or the path
// passed via --settings) and returns the resulting Settings.
//
// If the file doesn't exist, returns DefaultSettings() with no error —
// fresh installs work out of the box. If the file exists but is malformed,
// returns the parse error so the operator notices.
//
// Unspecified YAML fields fall back to the corresponding DefaultSettings()
// value, so adding a new Settings field to the code is backward-compatible
// with old YAML files.
func LoadSettings(path string) (Settings, error) {
	s := DefaultSettings()
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return s, fmt.Errorf("read settings %s: %w", path, err)
	}
	if err := yaml.Unmarshal(raw, &s); err != nil {
		return DefaultSettings(), fmt.Errorf("parse settings %s: %w", path, err)
	}
	return s, nil
}

// SaveSettings writes s to path atomically (temp file + rename), creating
// the parent directory if needed. The on-disk format matches the YAML
// example in ТЗ section 12.
func SaveSettings(path string, s Settings) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("settings: mkdir: %w", err)
	}
	body, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("settings: marshal: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return fmt.Errorf("settings: write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("settings: rename: %w", err)
	}
	return nil
}
