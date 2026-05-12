package config

// Settings carries per-router and per-user customization controlling the
// generated sing-box config.json. See docs/ТЗ — sing-box-helper.md section 4
// for the categorization of fields (router-dependent vs user choice vs fixed).
//
// YAML tags match the on-disk file format
// (/opt/etc/singbox-helper/config.yaml). YAML parsing itself will come in a
// later iteration; for now Settings is built in code via DefaultSettings.
type Settings struct {
	// Log
	LogLevel     string `yaml:"log_level"     json:"log_level"`
	LogTimestamp bool   `yaml:"log_timestamp" json:"log_timestamp"`

	// DNS used by sing-box itself (only to resolve the upstream node hostname)
	UpstreamDNS string `yaml:"upstream_dns" json:"upstream_dns"`
	DNSStrategy string `yaml:"dns_strategy" json:"dns_strategy"`

	// TUN inbound
	TunInterfaceName string `yaml:"tun_interface_name" json:"tun_interface_name"`
	TunAddress       string `yaml:"tun_address"        json:"tun_address"`
	TunMTU           int    `yaml:"tun_mtu"            json:"tun_mtu"`
	TunStack         string `yaml:"tun_stack"          json:"tun_stack"`

	// Mixed inbound (self-test proxy)
	EnableMixed     bool   `yaml:"enable_mixed"       json:"enable_mixed"`
	MixedListen     string `yaml:"mixed_listen"       json:"mixed_listen"`
	MixedListenPort int    `yaml:"mixed_listen_port"  json:"mixed_listen_port"`

	// Clash API (for dashboards like Yacd, Zashboard)
	EnableClashAPI bool   `yaml:"enable_clash_api"  json:"enable_clash_api"`
	ClashAPIListen string `yaml:"clash_api_listen"  json:"clash_api_listen"`
	ClashAPIUIDir  string `yaml:"clash_api_ui_dir"  json:"clash_api_ui_dir"`
}

// DefaultSettings returns settings matching the defaults documented in the
// ТЗ (section 12). They're safe for any Keenetic / Entware install — the
// only field that may want overriding is MixedListen, which can be set to
// a concrete LAN IP for cleaner exposure.
func DefaultSettings() Settings {
	return Settings{
		LogLevel:         "info",
		LogTimestamp:     true,
		UpstreamDNS:      "1.1.1.1",
		DNSStrategy:      "ipv4_only",
		TunInterfaceName: "singtun",
		TunAddress:       "198.18.0.1/30",
		TunMTU:           1500,
		TunStack:         "gvisor",
		EnableMixed:      true,
		MixedListen:      "0.0.0.0",
		MixedListenPort:  7891,
		EnableClashAPI:   true,
		ClashAPIListen:   "0.0.0.0:9090",
		ClashAPIUIDir:    "/opt/share/dashboard",
	}
}
