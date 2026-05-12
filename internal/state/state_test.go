package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoad_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if got != nil {
		t.Errorf("Load on missing file returned %+v, want nil", got)
	}

	want := &State{
		URI:       "hysteria2://pw@host:443",
		Label:     "my node",
		AppliedAt: time.Date(2026, 5, 12, 15, 30, 0, 0, time.UTC),
	}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err = Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got == nil {
		t.Fatal("Load returned nil after Save")
	}
	if got.URI != want.URI || got.Label != want.Label || !got.AppliedAt.Equal(want.AppliedAt) {
		t.Errorf("roundtrip mismatch: got %+v, want %+v", got, want)
	}
}

func TestSave_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "deeper", "state.json")
	if err := Save(path, &State{URI: "x"}); err != nil {
		t.Fatalf("Save with non-existing parents: %v", err)
	}
}

func TestLoad_CorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Error("expected decode error on corrupt state, got nil")
	}
}
