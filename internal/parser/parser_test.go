package parser

import (
	"strings"
	"testing"
)

func TestParse_DispatchHysteria2(t *testing.T) {
	pn, err := Parse("hysteria2://pw@host:443")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pn.Outbound.Type != "hysteria2" {
		t.Errorf("Type = %q, want hysteria2", pn.Outbound.Type)
	}
}

func TestParse_DispatchHY2Alias(t *testing.T) {
	pn, err := Parse("hy2://pw@host:443")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pn.Outbound.Type != "hysteria2" {
		t.Errorf("Type = %q", pn.Outbound.Type)
	}
}

func TestParse_DispatchVLESS(t *testing.T) {
	pn, err := Parse("vless://6a99b2ec-0d60-4607-acaa-bf666f29a787@host:443?type=tcp&security=tls&sni=ex.com&fp=chrome")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pn.Outbound.Type != "vless" {
		t.Errorf("Type = %q", pn.Outbound.Type)
	}
}

func TestParse_Errors(t *testing.T) {
	cases := []struct {
		name string
		uri  string
		want string
	}{
		{"empty", "", "empty URI"},
		{"whitespace only", "   ", "empty URI"},
		{"unknown scheme", "trojan://uuid@host:443", "unsupported URI scheme"},
		{"unknown scheme caps", "TROJAN://x@y:1", "unsupported URI scheme"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.uri)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}
