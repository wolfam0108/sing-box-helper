// Package backup manages snapshots of /opt/etc/sing-box/config.json.
// Each backup is a plain file next to the live config, named
// "config.json.bak-YYYYMMDD-HHMMSS". Listing, restoring, deleting and
// trimming are all driven by that on-disk naming convention — no
// database, no separate metadata file.
package backup

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// nameLayout is the timestamp portion of a backup filename, immediately
// after the "config.json.bak-" prefix.
const nameLayout = "20060102-150405"

// Manager handles backups of a single config file.
type Manager struct {
	// ConfigPath is the live file (e.g. /opt/etc/sing-box/config.json).
	// Backups are stored alongside it as ConfigPath + ".bak-<ts>".
	ConfigPath string
}

// New returns a Manager that operates on configPath.
func New(configPath string) *Manager {
	return &Manager{ConfigPath: configPath}
}

// Entry is one listed backup.
type Entry struct {
	// File is the absolute path on disk.
	File string `json:"file"`
	// Name is the basename (for display).
	Name string `json:"name"`
	// CreatedAt is parsed from the filename suffix.
	CreatedAt time.Time `json:"created_at"`
	// Size in bytes.
	Size int64 `json:"size"`
	// Summary is a short "Protocol  server:port" string, extracted from
	// the first proxy outbound in the JSON. Empty if the file is broken
	// or has no proxy outbound.
	Summary string `json:"summary,omitempty"`
}

// List returns all backups for the manager's ConfigPath, newest first.
// Missing parent directory is treated as "no backups", not an error.
func (m *Manager) List() ([]Entry, error) {
	dir := filepath.Dir(m.ConfigPath)
	base := filepath.Base(m.ConfigPath)
	prefix := base + ".bak-"

	ents, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("backup list: %w", err)
	}
	var out []Entry
	for _, e := range ents {
		if e.IsDir() || !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		ts, perr := time.Parse(nameLayout, strings.TrimPrefix(e.Name(), prefix))
		if perr != nil {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			continue
		}
		full := filepath.Join(dir, e.Name())
		out = append(out, Entry{
			File:      full,
			Name:      e.Name(),
			CreatedAt: ts,
			Size:      info.Size(),
			Summary:   summarise(full),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

// Create copies the current ConfigPath into a new backup file with the
// current timestamp. Returns the backup path on success, or an empty
// string if the live config doesn't exist (first-install case).
func (m *Manager) Create() (string, error) {
	src, err := os.Open(m.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	defer src.Close()

	bak := m.ConfigPath + ".bak-" + time.Now().Format(nameLayout)
	dst, err := os.OpenFile(bak, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", err
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		_ = os.Remove(bak)
		return "", err
	}
	return bak, nil
}

// Restore replaces the live config with the contents of the given backup
// file. Before overwriting, the current live config is itself snapshotted
// (so a wrong restore is itself undoable). file must be one of the
// entries returned by List() — anything else is rejected for safety.
func (m *Manager) Restore(file string) (newBackupOfCurrent string, err error) {
	if !m.belongsHere(file) {
		return "", fmt.Errorf("restore: %q is not a backup of %q", file, m.ConfigPath)
	}
	if _, err := os.Stat(file); err != nil {
		return "", fmt.Errorf("restore: %w", err)
	}
	bak, err := m.Create()
	if err != nil {
		return "", fmt.Errorf("restore: snapshot current: %w", err)
	}
	body, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("restore: read backup: %w", err)
	}
	if err := os.WriteFile(m.ConfigPath, body, 0o644); err != nil {
		return "", fmt.Errorf("restore: write live config: %w", err)
	}
	return bak, nil
}

// Delete removes a single backup file. Like Restore, only files that
// belong to this manager's ConfigPath are accepted.
func (m *Manager) Delete(file string) error {
	if !m.belongsHere(file) {
		return fmt.Errorf("delete: %q is not a backup of %q", file, m.ConfigPath)
	}
	if err := os.Remove(file); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}

// Trim deletes the oldest backups until at most keep remain. A keep of
// 0 or negative is a no-op (caller wants unlimited history).
func (m *Manager) Trim(keep int) (removed []string, err error) {
	if keep <= 0 {
		return nil, nil
	}
	all, err := m.List()
	if err != nil {
		return nil, err
	}
	if len(all) <= keep {
		return nil, nil
	}
	for _, e := range all[keep:] {
		if rerr := os.Remove(e.File); rerr == nil {
			removed = append(removed, e.File)
		}
	}
	return removed, nil
}

// belongsHere guards Restore/Delete against arbitrary-path inputs: the
// candidate file must sit in the same directory as the live config and
// match the "<base>.bak-<ts>" naming convention.
func (m *Manager) belongsHere(file string) bool {
	clean := filepath.Clean(file)
	if filepath.Dir(clean) != filepath.Dir(m.ConfigPath) {
		return false
	}
	base := filepath.Base(m.ConfigPath)
	name := filepath.Base(clean)
	prefix := base + ".bak-"
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	_, err := time.Parse(nameLayout, strings.TrimPrefix(name, prefix))
	return err == nil
}

// summarise reads the JSON at path and extracts a short description of
// the first proxy-style outbound for display in the backup list.
// Returns "" on any error — the UI shows just the timestamp in that case.
func summarise(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	type ob struct {
		Type       string `json:"type"`
		Tag        string `json:"tag"`
		Server     string `json:"server"`
		ServerPort int    `json:"server_port"`
	}
	var doc struct {
		Outbounds []ob `json:"outbounds"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	for _, o := range doc.Outbounds {
		if o.Tag == "proxy" || isProxyType(o.Type) {
			return fmt.Sprintf("%s  %s:%d", o.Type, o.Server, o.ServerPort)
		}
	}
	return ""
}

func isProxyType(t string) bool {
	switch t {
	case "hysteria2", "hysteria", "vless", "vmess", "trojan",
		"shadowsocks", "tuic", "anytls", "socks", "http":
		return true
	}
	return false
}
