// Package config builds the sing-box config.json out of a parsed node and
// user-controlled settings.
package config

// Config is the subset of sing-box config.json that we generate.
type Config struct {
	Log          *Log          `json:"log,omitempty"`
	DNS          *DNS          `json:"dns,omitempty"`
	Inbounds     []any         `json:"inbounds,omitempty"`
	Outbounds    []any         `json:"outbounds,omitempty"`
	Route        *Route        `json:"route,omitempty"`
	Experimental *Experimental `json:"experimental,omitempty"`
}

type Log struct {
	Level     string `json:"level,omitempty"`
	Timestamp bool   `json:"timestamp"`
}

type DNS struct {
	Servers  []DNSServer `json:"servers,omitempty"`
	Strategy string      `json:"strategy,omitempty"`
}

type DNSServer struct {
	Type   string `json:"type,omitempty"`
	Tag    string `json:"tag,omitempty"`
	Server string `json:"server,omitempty"`
}

// TunInbound is the TUN inbound that keen-pbr attaches its routing rules to.
type TunInbound struct {
	Type          string   `json:"type"`
	Tag           string   `json:"tag"`
	InterfaceName string   `json:"interface_name"`
	Address       []string `json:"address"`
	MTU           int      `json:"mtu"`
	AutoRoute     bool     `json:"auto_route"`
	StrictRoute   bool     `json:"strict_route"`
	Stack         string   `json:"stack"`
	Sniff         bool     `json:"sniff"`
}

// MixedInbound is a HTTP+SOCKS5 proxy on a fixed port, used for self-tests.
type MixedInbound struct {
	Type       string `json:"type"`
	Tag        string `json:"tag"`
	Listen     string `json:"listen"`
	ListenPort int    `json:"listen_port"`
}

// DirectOutbound is the "raw internet" outbound that sing-box uses for DNS
// resolution and any direct (non-tunneled) traffic.
type DirectOutbound struct {
	Type string `json:"type"`
	Tag  string `json:"tag"`
}

type Route struct {
	Final string      `json:"final"`
	Rules []RouteRule `json:"rules,omitempty"`
}

type RouteRule struct {
	Action   string `json:"action,omitempty"`
	Protocol string `json:"protocol,omitempty"`
}

type Experimental struct {
	ClashAPI *ClashAPI `json:"clash_api,omitempty"`
}

type ClashAPI struct {
	ExternalController string `json:"external_controller"`
	ExternalUI         string `json:"external_ui,omitempty"`
}
