package parser

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ParseHysteria2 parses a hysteria2:// or hy2:// URI into a ParsedNode.
//
// Expected URI shape:
//
//	hysteria2://password@host:port?sni=example.com&insecure=0&obfs=salamander&obfs-password=xxx#name
//
// hy2:// is treated as an alias for hysteria2://.
//
// Supported query parameters:
//
//	sni            — TLS Server Name. Defaults to host if absent.
//	insecure       — "1"/"true"/"yes" skips certificate verification (default: verify).
//	alpn           — comma-separated ALPN list, e.g. "h3".
//	obfs           — only "salamander" is accepted by Hysteria2.
//	obfs-password  — required when obfs=salamander.
func ParseHysteria2(raw string) (*ParsedNode, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, fmt.Errorf("hysteria2: parse url: %w", err)
	}
	if u.Scheme != "hysteria2" && u.Scheme != "hy2" {
		return nil, fmt.Errorf("hysteria2: unexpected scheme %q (want hysteria2:// or hy2://)", u.Scheme)
	}

	if u.User == nil {
		return nil, errors.New("hysteria2: missing password (user-info part of URI)")
	}
	// Hy2 puts the whole password as the "user" of user-info. If the password
	// itself contains a colon, url.Parse splits it into user+password — rejoin.
	password := u.User.Username()
	if pw, hasPw := u.User.Password(); hasPw {
		password = password + ":" + pw
	}
	if password == "" {
		return nil, errors.New("hysteria2: empty password")
	}

	host := u.Hostname()
	if host == "" {
		return nil, errors.New("hysteria2: missing host")
	}
	portStr := u.Port()
	if portStr == "" {
		return nil, errors.New("hysteria2: missing port")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("hysteria2: invalid port %q", portStr)
	}

	q := u.Query()

	sni := strings.TrimSpace(q.Get("sni"))
	if sni == "" {
		sni = host
	}

	insecure := parseBoolFlag(q.Get("insecure"))

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

	out := &Outbound{
		Type:       "hysteria2",
		Tag:        "proxy",
		Server:     host,
		ServerPort: port,
		Password:   password,
		TLS:        tls,
	}

	if obfs := strings.ToLower(strings.TrimSpace(q.Get("obfs"))); obfs != "" {
		if obfs != "salamander" {
			return nil, fmt.Errorf("hysteria2: unsupported obfs %q (only 'salamander' is supported)", obfs)
		}
		obfsPw := q.Get("obfs-password")
		if obfsPw == "" {
			return nil, errors.New("hysteria2: obfs=salamander requires obfs-password")
		}
		out.Obfs = &ObfsConfig{Type: "salamander", Password: obfsPw}
	}

	label := ""
	if u.Fragment != "" {
		if dec, errDec := url.QueryUnescape(u.Fragment); errDec == nil {
			label = dec
		} else {
			label = u.Fragment
		}
	}

	return &ParsedNode{
		Outbound: out,
		Label:    label,
		Display: Display{
			Protocol:  "Hysteria2",
			Server:    host,
			Port:      port,
			SNI:       sni,
			TLSVerify: !insecure,
			Transport: "native",
		},
	}, nil
}

// parseBoolFlag returns true for "1"/"true"/"yes"/"on" (case-insensitive).
// Anything else, including empty string, returns false.
func parseBoolFlag(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
