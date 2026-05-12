// Package parser converts node URIs (vless://, hysteria2://, ...) into
// sing-box outbound JSON structures.
package parser

// Outbound is the subset of a sing-box outbound configuration that we
// generate. Fields are emitted into JSON only if non-zero (omitempty).
//
// The tag is always "proxy" so that the rest of the config (route.final)
// can reference it without knowing the protocol.
type Outbound struct {
	Type       string      `json:"type"`
	Tag        string      `json:"tag"`
	Server     string      `json:"server,omitempty"`
	ServerPort int         `json:"server_port,omitempty"`
	Password   string      `json:"password,omitempty"`
	UUID       string      `json:"uuid,omitempty"`
	Flow       string      `json:"flow,omitempty"`
	TLS        *TLSConfig  `json:"tls,omitempty"`
	Obfs       *ObfsConfig `json:"obfs,omitempty"`
	Transport  *Transport  `json:"transport,omitempty"`
}

// TLSConfig mirrors sing-box's outbound TLS settings.
type TLSConfig struct {
	Enabled    bool        `json:"enabled"`
	ServerName string      `json:"server_name,omitempty"`
	Insecure   bool        `json:"insecure,omitempty"`
	ALPN       []string    `json:"alpn,omitempty"`
	UTLS       *UTLSConfig `json:"utls,omitempty"`
	Reality    *Reality    `json:"reality,omitempty"`
}

// ObfsConfig is the Hysteria2 obfuscation block. Only "salamander" is
// supported by Hysteria2 today.
type ObfsConfig struct {
	Type     string `json:"type"`
	Password string `json:"password,omitempty"`
}

// UTLSConfig configures uTLS fingerprinting (used with REALITY etc.).
type UTLSConfig struct {
	Enabled     bool   `json:"enabled"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

// Reality holds the REALITY TLS extension parameters.
type Reality struct {
	Enabled   bool   `json:"enabled"`
	PublicKey string `json:"public_key"`
	ShortID   string `json:"short_id,omitempty"`
}

// Transport describes a non-default transport wrapper (ws / grpc / httpupgrade / ...).
type Transport struct {
	Type        string `json:"type"`
	Host        string `json:"host,omitempty"`
	Path        string `json:"path,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
}

// ParsedNode is the parser result: the outbound itself plus
// human-readable metadata for the UI.
type ParsedNode struct {
	// Outbound is the generated sing-box outbound block.
	Outbound *Outbound

	// Label is the friendly name from the URI fragment (#name), if any.
	Label string

	// Display is a redacted view for showing in the UI without leaking secrets.
	Display Display
}

// Display is what the web UI renders in the "Распознанные параметры" panel.
type Display struct {
	Protocol  string `json:"protocol"`
	Server    string `json:"server"`
	Port      int    `json:"port"`
	SNI       string `json:"sni,omitempty"`
	TLSVerify bool   `json:"tls_verify"`
	Transport string `json:"transport"`

	// Notes carries optional warnings ("experimental xhttp support", etc.).
	Notes []string `json:"notes,omitempty"`
}
