package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/wolfam0108/sing-box-helper/internal/config"
	"github.com/wolfam0108/sing-box-helper/internal/state"
)

func newTestServer() *Server {
	return New(config.DefaultSettings())
}

// --- /api/preview --------------------------------------------------------

func TestPreview_Hysteria2(t *testing.T) {
	s := newTestServer()
	body := strings.NewReader(`{"uri":"hysteria2://pw@host.example.com:443"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/preview", body)
	rec := httptest.NewRecorder()

	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp previewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Display.Protocol != "Hysteria2" {
		t.Errorf("display.protocol = %q", resp.Display.Protocol)
	}
	if !strings.Contains(resp.Config, "\"type\": \"hysteria2\"") {
		t.Errorf("config doesn't contain hysteria2 outbound: %s", resp.Config)
	}
	if !strings.Contains(resp.Config, "\"auto_route\": false") {
		t.Errorf("config doesn't contain auto_route:false: %s", resp.Config)
	}
}

func TestPreview_InvalidURI(t *testing.T) {
	s := newTestServer()
	body := strings.NewReader(`{"uri":"trojan://x@y:443"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/preview", body)
	rec := httptest.NewRecorder()

	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestPreview_BadJSON(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/preview",
		bytes.NewReader([]byte("not json")))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestPreview_WrongMethod(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/preview", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}

// --- /api/status ---------------------------------------------------------

func TestStatus_Basic(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp statusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.TunName != "singtun" {
		t.Errorf("tun_name = %q", resp.TunName)
	}
	// On the dev machine there's no sing-box process and no singtun
	// interface — both should report false, NOT crash.
	if resp.SingBoxRunning {
		t.Logf("note: sing-box reported running on dev box, unexpected but not an error")
	}
}

// TestStatus_ReadsFromConfigJSON verifies that /api/status reads the
// currently-running outbound from config.json (the source of truth) AND
// pairs it with state.json metadata when they agree.
func TestStatus_ReadsFromConfigJSON(t *testing.T) {
	tmp := t.TempDir()
	s := newTestServer()
	s.ConfigPath = filepath.Join(tmp, "config.json")
	s.StatePath = filepath.Join(tmp, "state.json")

	// Write a minimal valid sing-box config.json with an Hy2 outbound.
	cfg := `{
	  "outbounds": [
	    {"type":"hysteria2","tag":"proxy","server":"host.example.com","server_port":2053},
	    {"type":"direct","tag":"direct"}
	  ]
	}`
	if err := os.WriteFile(s.ConfigPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	// First: state.json missing → managed=false.
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	var resp statusResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.CurrentNode == nil {
		t.Fatal("current_node should be populated from config.json")
	}
	if resp.CurrentNode.Managed {
		t.Errorf("managed should be false without state.json")
	}
	if resp.CurrentNode.Protocol != "Hysteria2" {
		t.Errorf("protocol = %q", resp.CurrentNode.Protocol)
	}
	if resp.CurrentNode.Server != "host.example.com" || resp.CurrentNode.Port != 2053 {
		t.Errorf("server:port = %s:%d", resp.CurrentNode.Server, resp.CurrentNode.Port)
	}

	// Now: state.json that matches config.json → managed=true with metadata.
	at := time.Date(2026, 5, 12, 15, 30, 0, 0, time.UTC)
	if err := state.Save(s.StatePath, &state.State{
		URI:       "hysteria2://pw@host.example.com:2053#my-node",
		Label:     "my-node",
		AppliedAt: at,
	}); err != nil {
		t.Fatal(err)
	}

	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp.CurrentNode.Managed {
		t.Errorf("managed should be true after writing matching state.json")
	}
	if resp.CurrentNode.Label != "my-node" {
		t.Errorf("label = %q", resp.CurrentNode.Label)
	}
	if resp.CurrentNode.AppliedAt == nil || !resp.CurrentNode.AppliedAt.Equal(at) {
		t.Errorf("applied_at = %v, want %v", resp.CurrentNode.AppliedAt, at)
	}
}

// --- /api/backups --------------------------------------------------------

func TestBackups_ListEmpty(t *testing.T) {
	tmp := t.TempDir()
	s := newTestServer()
	s.ConfigPath = filepath.Join(tmp, "config.json")
	req := httptest.NewRequest(http.MethodGet, "/api/backups", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp struct {
		Backups []any `json:"backups"`
		Keep    int   `json:"keep"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Backups) != 0 {
		t.Errorf("expected empty list, got %d", len(resp.Backups))
	}
	if resp.Keep != 10 {
		t.Errorf("keep = %d, want 10 (default)", resp.Keep)
	}
}

func TestBackups_ListsExisting(t *testing.T) {
	tmp := t.TempDir()
	s := newTestServer()
	s.ConfigPath = filepath.Join(tmp, "config.json")
	// Two on-disk backups with parseable timestamps.
	cfg := `{"outbounds":[{"type":"hysteria2","tag":"proxy","server":"h","server_port":443}]}`
	_ = os.WriteFile(s.ConfigPath+".bak-20260510-100000", []byte(cfg), 0o644)
	_ = os.WriteFile(s.ConfigPath+".bak-20260512-140000", []byte(cfg), 0o644)

	req := httptest.NewRequest(http.MethodGet, "/api/backups", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	var resp struct {
		Backups []struct {
			Name    string `json:"name"`
			Summary string `json:"summary"`
		} `json:"backups"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Backups) != 2 {
		t.Fatalf("len = %d", len(resp.Backups))
	}
	// Newest first.
	if !strings.Contains(resp.Backups[0].Name, "20260512") {
		t.Errorf("newest-first failure: %+v", resp.Backups)
	}
	if !strings.Contains(resp.Backups[0].Summary, "hysteria2") {
		t.Errorf("summary parsed wrong: %q", resp.Backups[0].Summary)
	}
}

func TestBackups_DeleteRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	s := newTestServer()
	s.ConfigPath = filepath.Join(tmp, "config.json")
	bak := s.ConfigPath + ".bak-20260510-100000"
	_ = os.WriteFile(bak, []byte("{}"), 0o644)

	req := httptest.NewRequest(http.MethodDelete, "/api/backups?file="+bak, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(bak); !os.IsNotExist(err) {
		t.Errorf("file should be gone")
	}
}

func TestBackups_DeleteForeignRejected(t *testing.T) {
	tmp := t.TempDir()
	s := newTestServer()
	s.ConfigPath = filepath.Join(tmp, "config.json")
	foreign := filepath.Join(t.TempDir(), "elsewhere.json.bak-20260510-100000")
	_ = os.WriteFile(foreign, []byte("{}"), 0o644)

	req := httptest.NewRequest(http.MethodDelete, "/api/backups?file="+foreign, nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Errorf("foreign file must not be deleted: %v", err)
	}
}

func TestBackups_Restore_PreSnapshotsCurrent(t *testing.T) {
	tmp := t.TempDir()
	s := newTestServer()
	s.ConfigPath = filepath.Join(tmp, "config.json")
	s.InitScript = filepath.Join(tmp, "no-init") // restart will fail (soft)

	// Live config + one older backup.
	_ = os.WriteFile(s.ConfigPath,
		[]byte(`{"outbounds":[{"type":"vless","tag":"proxy","server":"current","server_port":443}]}`),
		0o644)
	bak := s.ConfigPath + ".bak-20260510-100000"
	_ = os.WriteFile(bak,
		[]byte(`{"outbounds":[{"type":"hysteria2","tag":"proxy","server":"restored","server_port":2053}]}`),
		0o644)

	body, _ := json.Marshal(map[string]string{"file": bak})
	req := httptest.NewRequest(http.MethodPost, "/api/backups/restore", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp restoreResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.BackupOfPrevious == "" {
		t.Errorf("expected a fresh pre-restore backup path, got empty")
	}
	// Live config should now contain "restored".
	body2, _ := os.ReadFile(s.ConfigPath)
	if !strings.Contains(string(body2), "restored") {
		t.Errorf("live config not restored, got %s", body2)
	}
	// init script doesn't exist → restart_error filled, not a 5xx.
	if resp.Restarted {
		t.Errorf("restart should have failed gracefully (no init script)")
	}
}

// --- /api/logs -----------------------------------------------------------

func TestLogs_Helper_ReturnsRingBuffer(t *testing.T) {
	s := newTestServer()
	// Push a couple of fake helper-log lines into the buffer.
	_, _ = s.Logs.Write([]byte("hello\nworld\n"))
	req := httptest.NewRequest(http.MethodGet, "/api/logs?source=helper&lines=10", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp logsResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Source != "helper" {
		t.Errorf("source = %q", resp.Source)
	}
	if len(resp.Lines) != 2 {
		t.Fatalf("len(lines) = %d, want 2", len(resp.Lines))
	}
	if !strings.Contains(resp.Lines[0], "hello") || !strings.Contains(resp.Lines[1], "world") {
		t.Errorf("lines = %+v", resp.Lines)
	}
}

func TestLogs_SingBox_GracefulNoteWhenNdmcAbsent(t *testing.T) {
	// On the dev machine ndmc isn't installed; the handler must return
	// 200 with a note rather than 500.
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/logs?source=singbox", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp logsResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Note == "" {
		t.Errorf("expected an explanatory note when ndmc absent, got %+v", resp)
	}
}

func TestLogs_UnknownSource(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/logs?source=mystery", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// --- /api/settings -------------------------------------------------------

func TestSettings_GetReturnsDefaults(t *testing.T) {
	s := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var resp settingsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Settings.TunInterfaceName != "singtun" {
		t.Errorf("default tun_interface_name = %q", resp.Settings.TunInterfaceName)
	}
	if resp.Settings.UpstreamDNS != "1.1.1.1" {
		t.Errorf("default upstream_dns = %q", resp.Settings.UpstreamDNS)
	}
	// MixedListen default is "0.0.0.0" (not auto) → effective passes through, auto=false.
	if resp.MixedListenEffective != "0.0.0.0" || resp.MixedListenAuto {
		t.Errorf("effective=%q auto=%v, want 0.0.0.0 false", resp.MixedListenEffective, resp.MixedListenAuto)
	}
}

func TestSettings_PostPersistsAndSwaps(t *testing.T) {
	tmp := t.TempDir()
	s := newTestServer()
	s.SettingsPath = filepath.Join(tmp, "config.yaml")
	// Override fs paths to make sure POST doesn't attempt to restart a
	// real sing-box.
	s.ConfigPath = filepath.Join(tmp, "sing-box-config.json")
	s.StatePath = filepath.Join(tmp, "state.json")
	s.InitScript = filepath.Join(tmp, "no-init-script")

	new := config.DefaultSettings()
	new.MixedListen = "auto"
	new.UpstreamDNS = "9.9.9.9"
	new.MixedListenPort = 7892
	body, _ := json.Marshal(new)

	req := httptest.NewRequest(http.MethodPost, "/api/settings", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}

	// 1. In-memory settings actually swapped.
	got := s.Settings()
	if got.UpstreamDNS != "9.9.9.9" || got.MixedListen != "auto" || got.MixedListenPort != 7892 {
		t.Errorf("in-memory settings not swapped: %+v", got)
	}

	// 2. YAML file written on disk and round-trips.
	persisted, err := config.LoadSettings(s.SettingsPath)
	if err != nil {
		t.Fatalf("load persisted: %v", err)
	}
	if persisted.UpstreamDNS != "9.9.9.9" || persisted.MixedListen != "auto" {
		t.Errorf("YAML not persisted correctly: %+v", persisted)
	}

	// 3. Without state.json there's no re-render or restart to do.
	var resp settingsResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.ReRendered || resp.Restarted {
		t.Errorf("nothing to re-render without state.json: %+v", resp)
	}

	// 4. mixed_listen=auto on this dev box falls back to 0.0.0.0 (no br0),
	// but autodetect flag is true.
	if !resp.MixedListenAuto {
		t.Errorf("MixedListenAuto must be true when MixedListen=auto")
	}
}

func TestSettings_PostRejectsInvalid(t *testing.T) {
	s := newTestServer()
	bad := config.DefaultSettings()
	bad.MixedListenPort = 0 // invalid
	body, _ := json.Marshal(bad)
	req := httptest.NewRequest(http.MethodPost, "/api/settings", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
	// In-memory settings unchanged.
	if s.Settings().MixedListenPort == 0 {
		t.Errorf("invalid settings leaked into in-memory state")
	}
}

// TestStatus_StaleStateMismatch covers the case where state.json points
// to a different server than what's actually in config.json (someone
// edited config.json by hand). The user must NOT see misleading "managed"
// metadata for a different node.
func TestStatus_StaleStateMismatch(t *testing.T) {
	tmp := t.TempDir()
	s := newTestServer()
	s.ConfigPath = filepath.Join(tmp, "config.json")
	s.StatePath = filepath.Join(tmp, "state.json")

	cfg := `{"outbounds":[{"type":"vless","tag":"proxy","server":"new.example.com","server_port":443,"uuid":"6a99b2ec-0d60-4607-acaa-bf666f29a787"}]}`
	if err := os.WriteFile(s.ConfigPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	// state.json points at an OLD server.
	if err := state.Save(s.StatePath, &state.State{
		URI:       "hysteria2://pw@old.example.com:2053",
		Label:     "old",
		AppliedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	var resp statusResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.CurrentNode == nil {
		t.Fatal("current_node should be set")
	}
	if resp.CurrentNode.Managed {
		t.Error("managed must be false when state and config disagree")
	}
	if resp.CurrentNode.Server != "new.example.com" {
		t.Errorf("server should reflect config.json, got %q", resp.CurrentNode.Server)
	}
	if resp.CurrentNode.Label != "" || resp.CurrentNode.URI != "" {
		t.Errorf("label/uri should be empty when state is stale, got label=%q uri=%q",
			resp.CurrentNode.Label, resp.CurrentNode.URI)
	}
}

