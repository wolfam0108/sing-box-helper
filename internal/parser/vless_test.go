package parser

import (
	"strings"
	"testing"
)

func TestParseVLESS_TCPTls(t *testing.T) {
	const uri = "vless://6a99b2ec-0d60-4607-acaa-bf666f29a787@host.example.com:443" +
		"?type=tcp&encryption=none&security=tls&sni=cdn.example.com&fp=chrome" +
		"#tcp-tls-node"

	pn, err := ParseVLESS(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	o := pn.Outbound
	if o.Type != "vless" || o.Tag != "proxy" {
		t.Errorf("Type/Tag = %q/%q", o.Type, o.Tag)
	}
	if o.UUID != "6a99b2ec-0d60-4607-acaa-bf666f29a787" {
		t.Errorf("UUID = %q", o.UUID)
	}
	if o.Server != "host.example.com" || o.ServerPort != 443 {
		t.Errorf("server = %s:%d", o.Server, o.ServerPort)
	}
	if o.TLS == nil || !o.TLS.Enabled {
		t.Fatal("TLS missing or disabled")
	}
	if o.TLS.ServerName != "cdn.example.com" {
		t.Errorf("SNI = %q", o.TLS.ServerName)
	}
	if o.TLS.UTLS == nil || o.TLS.UTLS.Fingerprint != "chrome" {
		t.Errorf("UTLS = %+v", o.TLS.UTLS)
	}
	if o.Transport != nil {
		t.Errorf("Transport for tcp must be nil, got %+v", o.Transport)
	}
	if pn.Label != "tcp-tls-node" {
		t.Errorf("Label = %q", pn.Label)
	}
}

func TestParseVLESS_WSTls(t *testing.T) {
	const uri = "vless://6a99b2ec-0d60-4607-acaa-bf666f29a787@host.example.com:443" +
		"?type=ws&security=tls&sni=cdn.example.com&host=cdn.example.com&path=%2Fws&fp=firefox" +
		"#ws-tls-node"

	pn, err := ParseVLESS(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pn.Outbound.Transport == nil {
		t.Fatal("Transport is nil")
	}
	tr := pn.Outbound.Transport
	if tr.Type != "ws" {
		t.Errorf("Transport.Type = %q", tr.Type)
	}
	if tr.Host != "cdn.example.com" {
		t.Errorf("Transport.Host = %q", tr.Host)
	}
	if tr.Path != "/ws" {
		t.Errorf("Transport.Path = %q", tr.Path)
	}
	if pn.Outbound.TLS.UTLS.Fingerprint != "firefox" {
		t.Errorf("fingerprint = %q", pn.Outbound.TLS.UTLS.Fingerprint)
	}
}

func TestParseVLESS_GRPCTls(t *testing.T) {
	const uri = "vless://6a99b2ec-0d60-4607-acaa-bf666f29a787@host.example.com:443" +
		"?type=grpc&security=tls&sni=cdn.example.com&serviceName=mygrpc&alpn=h2&fp=chrome"

	pn, err := ParseVLESS(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tr := pn.Outbound.Transport
	if tr == nil || tr.Type != "grpc" {
		t.Fatalf("Transport = %+v", tr)
	}
	if tr.ServiceName != "mygrpc" {
		t.Errorf("ServiceName = %q", tr.ServiceName)
	}
	if got := strings.Join(pn.Outbound.TLS.ALPN, ","); got != "h2" {
		t.Errorf("ALPN = %q", got)
	}
}

func TestParseVLESS_PlainTCPNoTLS(t *testing.T) {
	const uri = "vless://6a99b2ec-0d60-4607-acaa-bf666f29a787@1.2.3.4:80?type=tcp&security=none"

	pn, err := ParseVLESS(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pn.Outbound.TLS != nil {
		t.Errorf("TLS should be nil for security=none, got %+v", pn.Outbound.TLS)
	}
	if pn.Display.TLSVerify {
		t.Error("Display.TLSVerify must be false when no TLS")
	}
}

func TestParseVLESS_TCPTLSVision(t *testing.T) {
	// Vision flow without REALITY is uncommon but technically valid for sing-box.
	const uri = "vless://6a99b2ec-0d60-4607-acaa-bf666f29a787@host:443" +
		"?type=tcp&security=tls&sni=example.com&flow=xtls-rprx-vision&fp=chrome"

	pn, err := ParseVLESS(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pn.Outbound.Flow != "xtls-rprx-vision" {
		t.Errorf("Flow = %q", pn.Outbound.Flow)
	}
}

func TestParseVLESS_RealityTCPVision(t *testing.T) {
	// Real URI from docs/ТЗ — TCP + REALITY + Vision (the most common combo
	// produced by 3x-ui for "vless+reality+vision" nodes).
	const uri = "vless://a0e5fd22-4aba-4861-a730-2a5b187424cd@ae.mywolfram.ru:58871" +
		"?type=tcp&encryption=none&security=reality&pbk=tM94SWNzuc___dzCxakr-0F2KF_GlJMIbM0eFtbYsG8" +
		"&fp=chrome&sni=yahoo.com&sid=42f2121ac05879a3&spx=%2F&flow=xtls-rprx-vision" +
		"#users-wolframM1"

	pn, err := ParseVLESS(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	o := pn.Outbound

	if o.Server != "ae.mywolfram.ru" || o.ServerPort != 58871 {
		t.Errorf("server = %s:%d", o.Server, o.ServerPort)
	}
	if o.UUID != "a0e5fd22-4aba-4861-a730-2a5b187424cd" {
		t.Errorf("UUID = %q", o.UUID)
	}
	if o.Flow != "xtls-rprx-vision" {
		t.Errorf("Flow = %q", o.Flow)
	}
	if o.Transport != nil {
		t.Errorf("Transport for tcp must be nil, got %+v", o.Transport)
	}

	if o.TLS == nil || !o.TLS.Enabled {
		t.Fatal("TLS missing or disabled")
	}
	if o.TLS.ServerName != "yahoo.com" {
		t.Errorf("SNI = %q, want yahoo.com", o.TLS.ServerName)
	}
	if o.TLS.UTLS == nil || o.TLS.UTLS.Fingerprint != "chrome" {
		t.Errorf("uTLS = %+v", o.TLS.UTLS)
	}
	if o.TLS.Reality == nil || !o.TLS.Reality.Enabled {
		t.Fatal("Reality block missing/disabled")
	}
	if o.TLS.Reality.PublicKey != "tM94SWNzuc___dzCxakr-0F2KF_GlJMIbM0eFtbYsG8" {
		t.Errorf("pbk = %q", o.TLS.Reality.PublicKey)
	}
	if o.TLS.Reality.ShortID != "42f2121ac05879a3" {
		t.Errorf("sid = %q", o.TLS.Reality.ShortID)
	}
	if pn.Label != "users-wolframM1" {
		t.Errorf("Label = %q", pn.Label)
	}
}

func TestParseVLESS_RealityWithoutSID(t *testing.T) {
	// sid is optional; absent → empty short_id.
	const uri = "vless://6a99b2ec-0d60-4607-acaa-bf666f29a787@host:443" +
		"?type=tcp&security=reality&pbk=AAAA_BBBB_CCCC&fp=chrome&sni=example.com"

	pn, err := ParseVLESS(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pn.Outbound.TLS.Reality.ShortID != "" {
		t.Errorf("ShortID should be empty, got %q", pn.Outbound.TLS.Reality.ShortID)
	}
}

func TestParseVLESS_RealityErrors(t *testing.T) {
	cases := []struct {
		name string
		uri  string
		want string
	}{
		{
			"missing pbk",
			"vless://6a99b2ec-0d60-4607-acaa-bf666f29a787@host:443?type=tcp&security=reality&fp=chrome&sni=ex.com",
			"'pbk'",
		},
		{
			"missing fp",
			"vless://6a99b2ec-0d60-4607-acaa-bf666f29a787@host:443?type=tcp&security=reality&pbk=AAA&sni=ex.com",
			"'fp'",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseVLESS(tc.uri)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

func TestParseVLESS_XHTTPReality(t *testing.T) {
	// Real URI from docs/ТЗ — xhttp + REALITY. xhttp has no native sing-box
	// equivalent in 1.13, so the parser maps it to httpupgrade and records
	// a warning in Display.Notes.
	const uri = "vless://6a99b2ec-0d60-4607-acaa-bf666f29a787@ae.mywolfram.ru:39435" +
		"?type=xhttp&encryption=none&path=%2F&host=&mode=auto&security=reality" +
		"&pbk=m1oonmPcmTO2kZLm0_vfN8D3YQ_8FrXkLOLYudI4tmA&fp=edge&sni=ya.ru" +
		"&sid=f064aec4&spx=%2F#VLESS-XHTTP-test-user-01"

	pn, err := ParseVLESS(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	o := pn.Outbound

	if o.Server != "ae.mywolfram.ru" || o.ServerPort != 39435 {
		t.Errorf("server = %s:%d", o.Server, o.ServerPort)
	}
	if o.Transport == nil {
		t.Fatal("Transport is nil")
	}
	if o.Transport.Type != "httpupgrade" {
		t.Errorf("Transport.Type = %q, want httpupgrade (mapped from xhttp)", o.Transport.Type)
	}
	if o.Transport.Path != "/" {
		t.Errorf("Transport.Path = %q, want /", o.Transport.Path)
	}
	if o.Transport.Host != "" {
		t.Errorf("Transport.Host = %q, want empty", o.Transport.Host)
	}
	if o.TLS == nil || o.TLS.Reality == nil {
		t.Fatal("TLS.Reality is missing")
	}
	if o.TLS.Reality.PublicKey != "m1oonmPcmTO2kZLm0_vfN8D3YQ_8FrXkLOLYudI4tmA" {
		t.Errorf("pbk = %q", o.TLS.Reality.PublicKey)
	}
	if o.TLS.Reality.ShortID != "f064aec4" {
		t.Errorf("sid = %q", o.TLS.Reality.ShortID)
	}
	if o.TLS.UTLS == nil || o.TLS.UTLS.Fingerprint != "edge" {
		t.Errorf("uTLS fingerprint = %+v", o.TLS.UTLS)
	}
	if o.TLS.ServerName != "ya.ru" {
		t.Errorf("SNI = %q", o.TLS.ServerName)
	}

	if pn.Display.Transport != "xhttp" {
		t.Errorf("Display.Transport = %q, want xhttp (original URI value preserved)", pn.Display.Transport)
	}
	if len(pn.Display.Notes) == 0 {
		t.Fatal("Display.Notes empty, expected xhttp-mapping warning")
	}
	if !strings.Contains(pn.Display.Notes[0], "httpupgrade") {
		t.Errorf("Notes[0] = %q; want it to mention httpupgrade", pn.Display.Notes[0])
	}
	if pn.Label != "VLESS-XHTTP-test-user-01" {
		t.Errorf("Label = %q", pn.Label)
	}
}

func TestParseVLESS_SplithttpReality(t *testing.T) {
	// type=splithttp is an older spelling of the same transport; should
	// behave identically to xhttp.
	const uri = "vless://6a99b2ec-0d60-4607-acaa-bf666f29a787@host:443" +
		"?type=splithttp&path=%2Fapi&security=reality&pbk=AAA&fp=chrome&sni=ex.com"

	pn, err := ParseVLESS(uri)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pn.Outbound.Transport == nil || pn.Outbound.Transport.Type != "httpupgrade" {
		t.Errorf("Transport = %+v", pn.Outbound.Transport)
	}
	if pn.Outbound.Transport.Path != "/api" {
		t.Errorf("Path = %q", pn.Outbound.Transport.Path)
	}
	if pn.Display.Transport != "splithttp" {
		t.Errorf("Display.Transport = %q", pn.Display.Transport)
	}
	if len(pn.Display.Notes) == 0 {
		t.Error("expected splithttp warning in Notes")
	}
}

func TestParseVLESS_Errors(t *testing.T) {
	cases := []struct {
		name string
		uri  string
		want string
	}{
		{"bad scheme", "hy2://uuid@host:443", "unexpected scheme"},
		{"missing uuid", "vless://host:443", "missing UUID"},
		{"invalid uuid", "vless://not-a-uuid@host:443", "invalid UUID"},
		{"colon in user-info", "vless://aaaa:bbbb@host:443", "':' in user-info"},
		{"missing port", "vless://6a99b2ec-0d60-4607-acaa-bf666f29a787@host", "missing port"},
		{"missing host", "vless://6a99b2ec-0d60-4607-acaa-bf666f29a787@:443", "missing host"},
		{"unsupported transport", "vless://6a99b2ec-0d60-4607-acaa-bf666f29a787@host:443?type=quic", "unsupported transport"},
		{"unsupported security", "vless://6a99b2ec-0d60-4607-acaa-bf666f29a787@host:443?security=xtls", "unsupported security"},
		{"unsupported flow", "vless://6a99b2ec-0d60-4607-acaa-bf666f29a787@host:443?security=tls&flow=xtls-rprx-direct", "unsupported flow"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseVLESS(tc.uri)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}
