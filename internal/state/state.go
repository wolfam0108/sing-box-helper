// Package state persists what singbox-helper considers the "currently
// applied" node so the UI can answer "what's running right now?" even
// after the server (or the whole router) is restarted.
//
// The state file lives at /opt/etc/singbox-helper/state.json by default.
// It is rewritten on every successful /api/apply.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// State is a snapshot of the last successful Apply.
type State struct {
	URI       string    `json:"uri"`
	Label     string    `json:"label,omitempty"`
	AppliedAt time.Time `json:"applied_at"`
}

// Load reads state from path. Returns (nil, nil) if the file doesn't
// exist (first-ever run); other I/O or JSON errors propagate.
func Load(path string) (*State, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("decode state %s: %w", path, err)
	}
	return &s, nil
}

// Save writes state to path atomically (write to temp + rename), creating
// the parent directory as needed.
func Save(path string, s *State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
