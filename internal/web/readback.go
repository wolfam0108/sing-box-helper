package web

import (
	"encoding/json"
	"os"
)

// outboundSnapshot is the small slice of a sing-box outbound we care about
// when answering "what is sing-box actually running right now?".
type outboundSnapshot struct {
	Type       string `json:"type"`
	Tag        string `json:"tag"`
	Server     string `json:"server"`
	ServerPort int    `json:"server_port"`
}

// proxyOutboundTypes is the set we consider "real" proxy outbounds — any
// of these in the outbounds[] array, even without tag="proxy", is what
// we'll report as the running node.
var proxyOutboundTypes = map[string]bool{
	"hysteria2":   true,
	"hysteria":    true,
	"vless":       true,
	"vmess":       true,
	"trojan":      true,
	"shadowsocks": true,
	"tuic":        true,
	"anytls":      true,
}

// readCurrentOutbound parses configPath and returns the first proxy
// outbound — preferring tag=="proxy" (our convention) and falling back
// to the first outbound whose type is in proxyOutboundTypes (so a config
// hand-written before this utility existed, like tag="hy2", still gets
// recognised).
//
// Returns nil if the file is missing, unparseable, or has no proxy
// outbound at all.
func readCurrentOutbound(configPath string) *outboundSnapshot {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}
	var doc struct {
		Outbounds []outboundSnapshot `json:"outbounds"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil
	}
	for _, o := range doc.Outbounds {
		if o.Tag == "proxy" {
			return &o
		}
	}
	for _, o := range doc.Outbounds {
		if proxyOutboundTypes[o.Type] {
			return &o
		}
	}
	return nil
}

// protocolLabel returns a human-friendly protocol name for the UI.
func protocolLabel(outboundType string) string {
	switch outboundType {
	case "hysteria2":
		return "Hysteria2"
	case "hysteria":
		return "Hysteria"
	case "vless":
		return "VLESS"
	case "vmess":
		return "VMess"
	case "trojan":
		return "Trojan"
	case "shadowsocks":
		return "Shadowsocks"
	case "tuic":
		return "TUIC"
	case "anytls":
		return "AnyTLS"
	default:
		return outboundType
	}
}
