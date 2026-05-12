package parser

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// uuidRegex matches RFC-style UUIDs (any version, hex digits). VLESS clients
// generate version-4 UUIDs but we don't enforce the version nibble.
var uuidRegex = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ParseVLESS parses a vless:// URI into a ParsedNode.
//
// Expected URI shape:
//
//	vless://uuid@host:port?type=ws&security=tls&sni=cdn.example.com&path=/ws&host=cdn.example.com&fp=chrome#name
//
// Supported:
//
//	transports: tcp, ws, grpc, h2, httpupgrade, xhttp (mapped to httpupgrade),
//	            splithttp (mapped to httpupgrade)
//	security:   none, tls, reality
//
// xhttp/splithttp from xray have no native equivalent in sing-box 1.13;
// the closest stable transport is httpupgrade and we map to it. A
// warning is recorded in Display.Notes so the UI can show it.
func ParseVLESS(raw string) (*ParsedNode, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("vless: parse url: %w", err)
	}
	if !strings.EqualFold(u.Scheme, "vless") {
		return nil, fmt.Errorf("vless: unexpected scheme %q (want vless://)", u.Scheme)
	}

	uuid, err := extractVLESSUUID(u)
	if err != nil {
		return nil, err
	}

	host := u.Hostname()
	if host == "" {
		return nil, errors.New("vless: missing host")
	}
	portStr := u.Port()
	if portStr == "" {
		return nil, errors.New("vless: missing port")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("vless: invalid port %q", portStr)
	}

	q := u.Query()

	transportType, err := pickVLESSTransport(q.Get("type"))
	if err != nil {
		return nil, err
	}
	security, err := pickVLESSSecurity(q.Get("security"))
	if err != nil {
		return nil, err
	}

	out := &Outbound{
		Type:       "vless",
		Tag:        "proxy",
		Server:     host,
		ServerPort: port,
		UUID:       uuid,
	}

	if flow := strings.TrimSpace(q.Get("flow")); flow != "" {
		if flow != "xtls-rprx-vision" {
			return nil, fmt.Errorf("vless: unsupported flow %q (only 'xtls-rprx-vision' is supported)", flow)
		}
		out.Flow = flow
	}

	switch security {
	case "tls":
		out.TLS = buildVLESSTLS(host, q)
	case "reality":
		tls, errTLS := buildVLESSReality(host, q)
		if errTLS != nil {
			return nil, errTLS
		}
		out.TLS = tls
	}

	if transportType != "tcp" {
		out.Transport = buildVLESSTransport(transportType, q)
	}

	label := ""
	if u.Fragment != "" {
		if dec, errDec := url.QueryUnescape(u.Fragment); errDec == nil {
			label = dec
		} else {
			label = u.Fragment
		}
	}

	display := Display{
		Protocol:  "VLESS",
		Server:    host,
		Port:      port,
		Transport: transportType,
	}
	if out.TLS != nil {
		display.SNI = out.TLS.ServerName
		display.TLSVerify = !out.TLS.Insecure
	}
	if transportType == "xhttp" || transportType == "splithttp" {
		display.Notes = append(display.Notes,
			fmt.Sprintf("Транспорт %q отображён в sing-box httpupgrade — sing-box 1.13 не имеет нативного xhttp-транспорта. Проверьте подключение.", transportType))
	}

	return &ParsedNode{
		Outbound: out,
		Label:    label,
		Display:  display,
	}, nil
}

// extractVLESSUUID validates the URI's user-info as a UUID. VLESS does not use
// password fields, so any ':' in user-info is treated as an error.
func extractVLESSUUID(u *url.URL) (string, error) {
	if u.User == nil {
		return "", errors.New("vless: missing UUID (user-info part)")
	}
	if _, hasPw := u.User.Password(); hasPw {
		return "", errors.New("vless: unexpected ':' in user-info (UUID must not contain a colon)")
	}
	uuid := u.User.Username()
	if !uuidRegex.MatchString(uuid) {
		return "", fmt.Errorf("vless: invalid UUID format %q", uuid)
	}
	return uuid, nil
}

func pickVLESSTransport(raw string) (string, error) {
	t := strings.ToLower(strings.TrimSpace(raw))
	if t == "" {
		t = "tcp"
	}
	switch t {
	case "tcp", "ws", "grpc", "h2", "httpupgrade", "xhttp", "splithttp":
		return t, nil
	default:
		return "", fmt.Errorf("vless: unsupported transport %q", t)
	}
}

func pickVLESSSecurity(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	if s == "" {
		s = "none"
	}
	switch s {
	case "none", "tls", "reality":
		return s, nil
	default:
		return "", fmt.Errorf("vless: unsupported security %q", s)
	}
}

func buildVLESSTLS(host string, q url.Values) *TLSConfig {
	sni := strings.TrimSpace(q.Get("sni"))
	if sni == "" {
		sni = host
	}
	// "allowInsecure" is the 3x-ui name; "insecure" is the Hiddify/NekoBox name.
	insecure := parseBoolFlag(q.Get("allowInsecure")) || parseBoolFlag(q.Get("insecure"))

	tls := &TLSConfig{
		Enabled:    true,
		ServerName: sni,
		Insecure:   insecure,
	}
	if alpn := strings.TrimSpace(q.Get("alpn")); alpn != "" {
		parts := strings.Split(alpn, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		tls.ALPN = parts
	}
	if fp := strings.TrimSpace(q.Get("fp")); fp != "" {
		tls.UTLS = &UTLSConfig{Enabled: true, Fingerprint: fp}
	}
	return tls
}

// buildVLESSReality builds a TLS block with the REALITY extension enabled.
//
// Required URI parameters:
//
//	pbk  — server public key (base64-url), maps to tls.reality.public_key
//	fp   — uTLS fingerprint (chrome / firefox / safari / ios / edge / random),
//	       maps to tls.utls.fingerprint
//	sni  — TLS server_name (the real target hostname being mimicked, e.g.
//	       "yahoo.com"). Falls back to the URI host only as a last resort —
//	       REALITY without an explicit sni is almost always misconfigured.
//
// Optional:
//
//	sid  — short_id (hex), maps to tls.reality.short_id (default: empty)
//	alpn — comma-separated ALPN list
//	spx  — spider_x; accepted and ignored (sing-box doesn't expose it as
//	       of 1.13).
func buildVLESSReality(host string, q url.Values) (*TLSConfig, error) {
	pbk := strings.TrimSpace(q.Get("pbk"))
	if pbk == "" {
		return nil, errors.New("vless: REALITY requires 'pbk' (public_key) in URI")
	}
	fp := strings.TrimSpace(q.Get("fp"))
	if fp == "" {
		return nil, errors.New("vless: REALITY requires 'fp' (uTLS fingerprint) in URI")
	}
	sni := strings.TrimSpace(q.Get("sni"))
	if sni == "" {
		sni = host
	}

	tls := &TLSConfig{
		Enabled:    true,
		ServerName: sni,
		UTLS: &UTLSConfig{
			Enabled:     true,
			Fingerprint: fp,
		},
		Reality: &Reality{
			Enabled:   true,
			PublicKey: pbk,
			ShortID:   strings.TrimSpace(q.Get("sid")),
		},
	}
	if alpn := strings.TrimSpace(q.Get("alpn")); alpn != "" {
		parts := strings.Split(alpn, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		tls.ALPN = parts
	}
	return tls, nil
}

// buildVLESSTransport builds a Transport block for non-tcp transports.
// It is the caller's responsibility to not call this for type=tcp.
//
// xhttp and splithttp from xray are mapped to sing-box's httpupgrade — see
// the package doc on ParseVLESS for the rationale.
func buildVLESSTransport(transportType string, q url.Values) *Transport {
	singboxType := transportType
	if singboxType == "xhttp" || singboxType == "splithttp" {
		singboxType = "httpupgrade"
	}
	tr := &Transport{Type: singboxType}
	switch singboxType {
	case "ws", "h2", "httpupgrade":
		tr.Host = strings.TrimSpace(q.Get("host"))
		tr.Path = strings.TrimSpace(q.Get("path"))
		if tr.Path == "" {
			tr.Path = "/"
		}
	case "grpc":
		tr.ServiceName = strings.TrimSpace(q.Get("serviceName"))
		if tr.ServiceName == "" {
			// some panels use the kebab-case spelling
			tr.ServiceName = strings.TrimSpace(q.Get("service-name"))
		}
	}
	return tr
}
