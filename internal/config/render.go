package config

import (
	"encoding/json"
	"errors"

	"github.com/wolfram0108/sing-box-helper/internal/parser"
	"github.com/wolfram0108/sing-box-helper/internal/probe"
)

// LANInterface is the name of the LAN bridge on Keenetic that holds the
// router's LAN-side IPv4. Used when MixedListen is "auto".
const LANInterface = "br0"

// Render assembles a complete sing-box config.json from the parsed node
// and the runtime settings, returning indented JSON bytes ready to be
// written to /opt/etc/sing-box/config.json.
//
// The fixed parts of the config (auto_route=false, sniff=true, the
// direct outbound, the sniff + hijack-dns rules, route.final="proxy")
// are intentionally hardcoded — see ТЗ section 4.В for the rationale.
func Render(node *parser.ParsedNode, s Settings) ([]byte, error) {
	if node == nil || node.Outbound == nil {
		return nil, errors.New("config: nil parsed node or outbound")
	}

	cfg := Config{
		Log: &Log{
			Level:     s.LogLevel,
			Timestamp: s.LogTimestamp,
		},
		DNS: &DNS{
			Servers: []DNSServer{
				{Type: "udp", Tag: "dns-direct", Server: s.UpstreamDNS},
			},
			Strategy: s.DNSStrategy,
		},
		Inbounds: []any{
			TunInbound{
				Type:          "tun",
				Tag:           "tun-in",
				InterfaceName: s.TunInterfaceName,
				Address:       []string{s.TunAddress},
				MTU:           s.TunMTU,
				AutoRoute:     false,
				StrictRoute:   false,
				Stack:         s.TunStack,
				Sniff:         true,
			},
		},
		Outbounds: []any{
			node.Outbound,
			DirectOutbound{Type: "direct", Tag: "direct"},
		},
		Route: &Route{
			Final: "proxy",
			Rules: []RouteRule{
				{Action: "sniff"},
				{Protocol: "dns", Action: "hijack-dns"},
			},
		},
	}

	if s.EnableMixed {
		listen, _ := probe.ResolveMixedListen(s.MixedListen, LANInterface)
		cfg.Inbounds = append(cfg.Inbounds, MixedInbound{
			Type:       "mixed",
			Tag:        "test-proxy",
			Listen:     listen,
			ListenPort: s.MixedListenPort,
		})
	}

	if s.EnableClashAPI {
		cfg.Experimental = &Experimental{
			ClashAPI: &ClashAPI{
				ExternalController: s.ClashAPIListen,
				ExternalUI:         s.ClashAPIUIDir,
			},
		}
	}

	return json.MarshalIndent(cfg, "", "  ")
}
