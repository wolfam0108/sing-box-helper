package parser

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ParseSOCKS converts a SOCKS URI (socks://, socks4://, socks4a://,
// socks5://, socks5h://) into a sing-box "type":"socks" outbound.
//
// URI format:
//
//	socks5://[user:pass@]host:port[#friendly-name]
//	socks5h://[user:pass@]host:port[#friendly-name]      → version "5", DNS-on-server is sing-box default for socks5 anyway
//	socks4://host:port[#friendly-name]                    → version "4", auth dropped if present
//	socks4a://host:port[#friendly-name]                   → version "4a"
//	socks://[user:pass@]host:port[#friendly-name]         → version "5" (default)
//
// Rationale for SOCKS support — section 5 of the ТЗ: this is not "yet another
// remote proxy". It lets sing-box-helper wrap *locally running* proxy clients
// (naive-proxy, mieru-client, xray-core, ...) that expose only a SOCKS
// listener. sing-box pulls packets from TUN → forwards into the local SOCKS
// → that client takes them out by its own protocol.
func ParseSOCKS(raw string) (*ParsedNode, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}

	scheme := strings.ToLower(u.Scheme)
	version, err := socksVersion(scheme)
	if err != nil {
		return nil, err
	}

	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("socks: missing host")
	}
	portStr := u.Port()
	if portStr == "" {
		return nil, fmt.Errorf("socks: missing port")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("socks: invalid port %q", portStr)
	}

	var notes []string
	var username, password string

	if u.User != nil {
		username = u.User.Username()
		pw, hasPw := u.User.Password()
		password = pw

		switch version {
		case "4", "4a":
			// SOCKS4 has no auth — drop and warn.
			username = ""
			password = ""
			notes = append(notes, "SOCKS4 не поддерживает auth, учётные данные из URI проигнорированы")
		case "5":
			if username != "" && !hasPw {
				return nil, fmt.Errorf("socks5: username without password is not supported (use user:pass@host or no auth)")
			}
		}
	}

	notes = append(notes, locationHint(host)...)

	out := &Outbound{
		Type:       "socks",
		Tag:        "proxy",
		Server:     host,
		ServerPort: port,
		Version:    version,
		Username:   username,
		Password:   password,
	}

	transport := "—"
	if version == "5" {
		transport = "SOCKS5"
	} else {
		transport = "SOCKS" + version
	}

	return &ParsedNode{
		Outbound: out,
		Label:    u.Fragment,
		Display: Display{
			Protocol:  "SOCKS",
			Server:    host,
			Port:      port,
			Transport: transport,
			TLSVerify: false,
			Notes:     notes,
		},
	}, nil
}

func socksVersion(scheme string) (string, error) {
	switch scheme {
	case "socks", "socks5", "socks5h":
		return "5", nil
	case "socks4":
		return "4", nil
	case "socks4a":
		return "4a", nil
	default:
		return "", fmt.Errorf("socks: unknown scheme %q", scheme)
	}
}

// locationHint produces UI notes about whether the SOCKS upstream is local
// (typical "wrap naive-proxy / mieru-client" scenario) or remote without TLS
// (credentials and traffic go in plaintext — should be flagged).
func locationHint(host string) []string {
	if isLocalHost(host) {
		return []string{"Локальный SOCKS-upstream — типичный сценарий для naive-proxy / mieru-client на этом же роутере."}
	}
	return []string{"Удалённый SOCKS без TLS — учётные данные и трафик идут в открытом виде. Используйте только в доверенной сети или через TLS-туннель."}
}

func isLocalHost(host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}
