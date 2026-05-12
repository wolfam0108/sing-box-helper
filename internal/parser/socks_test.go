package parser

import (
	"strings"
	"testing"
)

func TestParseSOCKS5_Local_NoAuth(t *testing.T) {
	pn, err := ParseSOCKS("socks5://127.0.0.1:1080#mieru-local")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pn.Outbound.Type != "socks" || pn.Outbound.Tag != "proxy" {
		t.Errorf("unexpected type/tag: %+v", pn.Outbound)
	}
	if pn.Outbound.Server != "127.0.0.1" || pn.Outbound.ServerPort != 1080 {
		t.Errorf("server/port: %+v", pn.Outbound)
	}
	if pn.Outbound.Version != "5" {
		t.Errorf("version: %q", pn.Outbound.Version)
	}
	if pn.Outbound.Username != "" || pn.Outbound.Password != "" {
		t.Errorf("auth should be empty: u=%q p=%q", pn.Outbound.Username, pn.Outbound.Password)
	}
	if pn.Label != "mieru-local" {
		t.Errorf("label: %q", pn.Label)
	}
	if !containsNoteLike(pn.Display.Notes, "Локальный SOCKS-upstream") {
		t.Errorf("expected local-host hint in notes, got %v", pn.Display.Notes)
	}
}

func TestParseSOCKS5_LocalLocalhost(t *testing.T) {
	pn, err := ParseSOCKS("socks5://localhost:2080")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !containsNoteLike(pn.Display.Notes, "Локальный SOCKS-upstream") {
		t.Errorf("expected local-host hint for localhost, got %v", pn.Display.Notes)
	}
}

func TestParseSOCKS5_WithAuth(t *testing.T) {
	pn, err := ParseSOCKS("socks5://alice:s3cret@127.0.0.1:1080")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pn.Outbound.Username != "alice" || pn.Outbound.Password != "s3cret" {
		t.Errorf("auth: u=%q p=%q", pn.Outbound.Username, pn.Outbound.Password)
	}
}

func TestParseSOCKS5_UsernameWithoutPassword_Rejected(t *testing.T) {
	_, err := ParseSOCKS("socks5://aliceonly@127.0.0.1:1080")
	if err == nil {
		t.Fatalf("expected error for username-without-password URI")
	}
	if !strings.Contains(err.Error(), "username without password") {
		t.Errorf("error mentions cause: %v", err)
	}
}

func TestParseSOCKS_DefaultsToV5(t *testing.T) {
	pn, err := ParseSOCKS("socks://10.0.0.5:1080")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pn.Outbound.Version != "5" {
		t.Errorf("scheme socks:// should map to version 5, got %q", pn.Outbound.Version)
	}
}

func TestParseSOCKS5h_MapsToV5(t *testing.T) {
	pn, err := ParseSOCKS("socks5h://10.0.0.5:1080")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pn.Outbound.Version != "5" {
		t.Errorf("socks5h:// → version 5, got %q", pn.Outbound.Version)
	}
}

func TestParseSOCKS4_NoAuth(t *testing.T) {
	pn, err := ParseSOCKS("socks4://1.2.3.4:1080")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pn.Outbound.Version != "4" {
		t.Errorf("version: %q", pn.Outbound.Version)
	}
}

func TestParseSOCKS4a_NoAuth(t *testing.T) {
	pn, err := ParseSOCKS("socks4a://1.2.3.4:1080")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pn.Outbound.Version != "4a" {
		t.Errorf("version: %q", pn.Outbound.Version)
	}
}

func TestParseSOCKS4_DropsAuth_WithWarning(t *testing.T) {
	pn, err := ParseSOCKS("socks4://alice:s3cret@1.2.3.4:1080")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if pn.Outbound.Username != "" || pn.Outbound.Password != "" {
		t.Errorf("socks4 auth must be dropped, got u=%q p=%q", pn.Outbound.Username, pn.Outbound.Password)
	}
	if !containsNoteLike(pn.Display.Notes, "SOCKS4 не поддерживает auth") {
		t.Errorf("expected socks4 auth-drop warning, got %v", pn.Display.Notes)
	}
}

func TestParseSOCKS_RemoteHost_TLSWarning(t *testing.T) {
	pn, err := ParseSOCKS("socks5://1.2.3.4:1080")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !containsNoteLike(pn.Display.Notes, "Удалённый SOCKS без TLS") {
		t.Errorf("expected remote-no-tls warning, got %v", pn.Display.Notes)
	}
}

func TestParseSOCKS_MissingPort(t *testing.T) {
	_, err := ParseSOCKS("socks5://127.0.0.1")
	if err == nil {
		t.Fatalf("expected error for missing port")
	}
}

func TestParseSOCKS_BadPort(t *testing.T) {
	_, err := ParseSOCKS("socks5://127.0.0.1:99999")
	if err == nil {
		t.Fatalf("expected error for out-of-range port")
	}
}

func TestParseSOCKS_DispatchedByMainParser(t *testing.T) {
	for _, uri := range []string{
		"socks://127.0.0.1:1080",
		"socks4://1.2.3.4:1080",
		"socks4a://1.2.3.4:1080",
		"socks5://127.0.0.1:1080",
		"socks5h://127.0.0.1:1080",
	} {
		pn, err := Parse(uri)
		if err != nil {
			t.Errorf("%s: %v", uri, err)
			continue
		}
		if pn.Outbound.Type != "socks" {
			t.Errorf("%s: got type %q", uri, pn.Outbound.Type)
		}
	}
}

func containsNoteLike(notes []string, substr string) bool {
	for _, n := range notes {
		if strings.Contains(n, substr) {
			return true
		}
	}
	return false
}
