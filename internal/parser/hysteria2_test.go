package parser

import (
	"strings"
	"testing"
)

func TestParseHysteria2_RealNode(t *testing.T) {
	// Реальный URI из практики (из 3x-ui). Никаких query-параметров.
	const uri = "hysteria2://OkGilZTuWOaHp7ii6XMyRf@np.mywolfram.ru:2053"

	pn, err := ParseHysteria2(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	o := pn.Outbound
	if o.Type != "hysteria2" {
		t.Errorf("Type = %q, want hysteria2", o.Type)
	}
	if o.Tag != "proxy" {
		t.Errorf("Tag = %q, want proxy", o.Tag)
	}
	if o.Server != "np.mywolfram.ru" {
		t.Errorf("Server = %q", o.Server)
	}
	if o.ServerPort != 2053 {
		t.Errorf("ServerPort = %d, want 2053", o.ServerPort)
	}
	if o.Password != "OkGilZTuWOaHp7ii6XMyRf" {
		t.Errorf("Password = %q", o.Password)
	}
	if o.TLS == nil {
		t.Fatal("TLS is nil")
	}
	if !o.TLS.Enabled {
		t.Error("TLS.Enabled = false, want true")
	}
	// Без sni= в URI server_name берётся из host.
	if o.TLS.ServerName != "np.mywolfram.ru" {
		t.Errorf("TLS.ServerName = %q, want np.mywolfram.ru", o.TLS.ServerName)
	}
	if o.TLS.Insecure {
		t.Error("TLS.Insecure = true, want false (verify by default)")
	}
	if o.Obfs != nil {
		t.Errorf("Obfs = %+v, want nil", o.Obfs)
	}

	if pn.Display.Protocol != "Hysteria2" {
		t.Errorf("Display.Protocol = %q", pn.Display.Protocol)
	}
	if !pn.Display.TLSVerify {
		t.Error("Display.TLSVerify = false")
	}
}

func TestParseHysteria2_HY2Alias(t *testing.T) {
	const uri = "hy2://pw@example.com:443"
	pn, err := ParseHysteria2(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pn.Outbound.Server != "example.com" || pn.Outbound.ServerPort != 443 {
		t.Errorf("got %s:%d", pn.Outbound.Server, pn.Outbound.ServerPort)
	}
}

func TestParseHysteria2_AllQueryParams(t *testing.T) {
	const uri = "hysteria2://secret@srv.example.net:2053" +
		"?sni=cdn.example.net" +
		"&insecure=1" +
		"&alpn=h3,h2" +
		"&obfs=salamander" +
		"&obfs-password=obfsPW" +
		"#my%20node"

	pn, err := ParseHysteria2(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	o := pn.Outbound
	if o.TLS.ServerName != "cdn.example.net" {
		t.Errorf("ServerName = %q", o.TLS.ServerName)
	}
	if !o.TLS.Insecure {
		t.Error("Insecure should be true")
	}
	if got, want := strings.Join(o.TLS.ALPN, ","), "h3,h2"; got != want {
		t.Errorf("ALPN = %q, want %q", got, want)
	}
	if o.Obfs == nil || o.Obfs.Type != "salamander" || o.Obfs.Password != "obfsPW" {
		t.Errorf("Obfs = %+v", o.Obfs)
	}
	if pn.Label != "my node" {
		t.Errorf("Label = %q, want %q", pn.Label, "my node")
	}
	if pn.Display.TLSVerify {
		t.Error("Display.TLSVerify should be false when insecure=1")
	}
}

func TestParseHysteria2_Errors(t *testing.T) {
	cases := []struct {
		name string
		uri  string
		want string // substring expected in error message
	}{
		{"bad scheme", "vless://uuid@host:443", "unexpected scheme"},
		{"missing port", "hysteria2://pw@host", "missing port"},
		{"missing password", "hysteria2://host:443", "missing password"},
		{"missing host", "hysteria2://pw@:443", "missing host"},
		{"port out of range", "hysteria2://pw@host:70000", "invalid port"},
		{"port not a number", "hysteria2://pw@host:abc", "invalid port"},
		{"obfs no password", "hysteria2://pw@host:443?obfs=salamander", "obfs-password"},
		{"unsupported obfs", "hysteria2://pw@host:443?obfs=xor&obfs-password=x", "unsupported obfs"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseHysteria2(tc.uri)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

func TestParseHysteria2_PasswordWithColon(t *testing.T) {
	// Хотя редко, пароль может содержать ':' — net/url трактует это как user:pass,
	// поэтому парсер должен склеить обратно.
	const uri = "hysteria2://user:pass@host:443"
	pn, err := ParseHysteria2(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pn.Outbound.Password != "user:pass" {
		t.Errorf("Password = %q, want %q", pn.Outbound.Password, "user:pass")
	}
}
