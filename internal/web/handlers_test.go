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

