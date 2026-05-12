package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSettings_MissingFile_ReturnsDefaults(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "no-such.yaml")
	s, err := LoadSettings(tmp)
	if err != nil {
		t.Fatalf("missing file should be a non-error: %v", err)
	}
	def := DefaultSettings()
	if s != def {
		t.Errorf("got %+v, want defaults %+v", s, def)
	}
}

func TestLoadSettings_PartialFile_MergesWithDefaults(t *testing.T) {
	// Only some fields specified — the rest must keep their default values.
	body := []byte(
		"mixed_listen: auto\n" +
			"upstream_dns: 9.9.9.9\n" +
			"keep_backups: 10\n")
	tmp := filepath.Join(t.TempDir(), "settings.yaml")
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSettings(tmp)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if s.MixedListen != "auto" {
		t.Errorf("MixedListen = %q, want auto", s.MixedListen)
	}
	if s.UpstreamDNS != "9.9.9.9" {
		t.Errorf("UpstreamDNS = %q, want 9.9.9.9", s.UpstreamDNS)
	}
	if s.TunInterfaceName != "singtun" {
		t.Errorf("TunInterfaceName = %q, want singtun (default kept)", s.TunInterfaceName)
	}
	if !s.EnableMixed {
		t.Errorf("EnableMixed default lost: %v", s.EnableMixed)
	}
}

func TestLoadSettings_Malformed_ReturnsError(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "broken.yaml")
	if err := os.WriteFile(tmp, []byte("mixed_listen: [unclosed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSettings(tmp); err == nil {
		t.Fatalf("malformed YAML should error")
	}
}

func TestSaveLoad_RoundTrip(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "rt.yaml")
	want := DefaultSettings()
	want.MixedListen = "auto"
	want.UpstreamDNS = "9.9.9.9"
	want.MixedListenPort = 7892
	if err := SaveSettings(tmp, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadSettings(tmp)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got != want {
		t.Errorf("round-trip mismatch:\n  got  %+v\n  want %+v", got, want)
	}
}

func TestSaveSettings_CreatesParentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dir", "config.yaml")
	if err := SaveSettings(path, DefaultSettings()); err != nil {
		t.Fatalf("save with missing parent: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file not created: %v", err)
	}
}
