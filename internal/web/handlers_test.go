package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wolfam0108/sing-box-helper/internal/config"
	"github.com/wolfam0108/sing-box-helper/internal/parser"
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

func TestStatus_WithLastApplied(t *testing.T) {
	s := newTestServer()
	// Inject a fake lastApplied via the unexported setter.
	pn, err := parser.Parse("hysteria2://pw@host.example.com:2053")
	if err != nil {
		t.Fatal(err)
	}
	s.setLastApplied(pn)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	var resp statusResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.CurrentNode == nil {
		t.Fatal("current_node should be populated")
	}
	if resp.CurrentNode.Server != "host.example.com" || resp.CurrentNode.Port != 2053 {
		t.Errorf("current_node = %+v", resp.CurrentNode)
	}
}
