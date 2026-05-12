package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newManager(t *testing.T) *Manager {
	t.Helper()
	tmp := t.TempDir()
	return New(filepath.Join(tmp, "config.json"))
}

func writeLive(t *testing.T, m *Manager, body string) {
	t.Helper()
	if err := os.WriteFile(m.ConfigPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// helper to create a backup file with a specific timestamp so List can
// distinguish "older" vs "newer" without 1-sec sleeps.
func touch(t *testing.T, m *Manager, ts time.Time, body string) string {
	t.Helper()
	path := m.ConfigPath + ".bak-" + ts.Format(nameLayout)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestList_Empty(t *testing.T) {
	m := newManager(t)
	out, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Errorf("len = %d", len(out))
	}
}

func TestCreate_NoLiveConfig_NoOp(t *testing.T) {
	m := newManager(t)
	bak, err := m.Create()
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if bak != "" {
		t.Errorf("expected empty path, got %q", bak)
	}
}

func TestCreate_RoundTrip(t *testing.T) {
	m := newManager(t)
	writeLive(t, m, `{"outbounds":[{"type":"hysteria2","tag":"proxy","server":"h","server_port":443}]}`)
	bak, err := m.Create()
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(bak, ".bak-") {
		t.Errorf("bak path = %q", bak)
	}
	body, _ := os.ReadFile(bak)
	if !strings.Contains(string(body), "hysteria2") {
		t.Errorf("backup contents wrong")
	}
}

func TestList_OrderNewestFirst_AndSummary(t *testing.T) {
	m := newManager(t)
	older := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 5, 12, 14, 0, 0, 0, time.UTC)
	touch(t, m, older,
		`{"outbounds":[{"type":"vless","tag":"proxy","server":"old.example","server_port":443}]}`)
	touch(t, m, newer,
		`{"outbounds":[{"type":"hysteria2","tag":"proxy","server":"new.example","server_port":2053}]}`)
	out, err := m.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("len = %d", len(out))
	}
	if !out[0].CreatedAt.Equal(newer) {
		t.Errorf("first should be newer, got %v", out[0].CreatedAt)
	}
	if !strings.Contains(out[0].Summary, "hysteria2") || !strings.Contains(out[0].Summary, "new.example") {
		t.Errorf("summary[0] = %q", out[0].Summary)
	}
	if !strings.Contains(out[1].Summary, "vless") {
		t.Errorf("summary[1] = %q", out[1].Summary)
	}
}

func TestRestore_RoundTrip(t *testing.T) {
	m := newManager(t)
	// 1. write live "current"
	writeLive(t, m, `{"outbounds":[{"type":"vless","tag":"proxy","server":"current","server_port":443}]}`)
	// 2. drop an older backup we'll restore
	older := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)
	oldFile := touch(t, m, older,
		`{"outbounds":[{"type":"hysteria2","tag":"proxy","server":"restored","server_port":2053}]}`)
	// 3. restore the older one
	preRestoreBackup, err := m.Restore(oldFile)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	// 3a. the previously-live config got snapshotted into a new backup
	body, _ := os.ReadFile(preRestoreBackup)
	if !strings.Contains(string(body), "current") {
		t.Errorf("pre-restore backup didn't capture the previously-live config: %s", body)
	}
	// 3b. the live file is now the older content
	body, _ = os.ReadFile(m.ConfigPath)
	if !strings.Contains(string(body), "restored") {
		t.Errorf("live config not restored to older content: %s", body)
	}
}

func TestRestore_RejectForeignPath(t *testing.T) {
	m := newManager(t)
	foreign := filepath.Join(t.TempDir(), "elsewhere", "config.json.bak-20260510-100000")
	_ = os.MkdirAll(filepath.Dir(foreign), 0o755)
	_ = os.WriteFile(foreign, []byte("{}"), 0o644)
	if _, err := m.Restore(foreign); err == nil {
		t.Fatalf("expected refusal for backup outside the managed directory")
	}
}

func TestDelete_BasicAndForeign(t *testing.T) {
	m := newManager(t)
	ts := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)
	file := touch(t, m, ts, "{}")
	if err := m.Delete(file); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("file still exists after delete")
	}

	// foreign file
	other := filepath.Join(t.TempDir(), "alien.json")
	_ = os.WriteFile(other, []byte("{}"), 0o644)
	if err := m.Delete(other); err == nil {
		t.Errorf("expected refusal for foreign delete")
	}
}

func TestTrim_KeepsNewestN(t *testing.T) {
	m := newManager(t)
	// create 5 timestamped backups
	for i := 0; i < 5; i++ {
		touch(t, m, time.Date(2026, 5, 1+i, 0, 0, 0, 0, time.UTC), "{}")
	}
	removed, err := m.Trim(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 2 {
		t.Errorf("removed = %d, want 2", len(removed))
	}
	remaining, _ := m.List()
	if len(remaining) != 3 {
		t.Errorf("remaining = %d, want 3", len(remaining))
	}
	// the 3 newest (May 3,4,5) should survive
	for _, e := range remaining {
		if e.CreatedAt.Day() < 3 {
			t.Errorf("kept too-old entry: %v", e.CreatedAt)
		}
	}
}

func TestTrim_NoOpWhenKeepZero(t *testing.T) {
	m := newManager(t)
	for i := 0; i < 5; i++ {
		touch(t, m, time.Date(2026, 5, 1+i, 0, 0, 0, 0, time.UTC), "{}")
	}
	removed, _ := m.Trim(0)
	if removed != nil {
		t.Errorf("keep=0 should be a no-op, got %v", removed)
	}
}
