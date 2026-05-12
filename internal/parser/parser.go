package parser

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// Parse looks at the URI scheme and dispatches to the right protocol parser.
//
// Supported schemes:
//
//	hysteria2:// (alias hy2://) — see ParseHysteria2
//	vless://                    — see ParseVLESS
//
// Other schemes (vmess, trojan, ss, tuic, anytls, ...) are tracked for v2.
func Parse(raw string) (*ParsedNode, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("empty URI")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse url: %w", err)
	}
	switch strings.ToLower(u.Scheme) {
	case "hysteria2", "hy2":
		return ParseHysteria2(raw)
	case "vless":
		return ParseVLESS(raw)
	default:
		return nil, fmt.Errorf("unsupported URI scheme %q (supported: hysteria2://, hy2://, vless://)", u.Scheme)
	}
}
