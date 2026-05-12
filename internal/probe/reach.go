// Package probe contains the diagnostic checks listed in ТЗ section 7.1
// "Проверка работы" — reachability to the node, sing-box process state,
// TUN interface, IP via tunnel, etc.
package probe

import (
	"fmt"
	"net"
	"strconv"
	"time"
)

// ProtoNetwork maps a sing-box outbound type to the network kind ("tcp"
// or "udp") used for a low-level reachability check. Outbounds we don't
// know default to "tcp" — that's correct for vless/trojan/ss/etc.
func ProtoNetwork(outboundType string) string {
	switch outboundType {
	case "hysteria2", "hysteria", "tuic":
		return "udp"
	default:
		return "tcp"
	}
}

// Reach attempts a low-level "is this address reachable from us" check.
//
// For TCP, this is a full three-way handshake — failure means the server
// is down, the wrong port, or a network/firewall block.
//
// For UDP, net.DialTimeout only binds a local socket; it does NOT send any
// bytes to the server, so a "pass" only proves the local network stack
// and DNS resolution work. We still run it because a failure here (e.g.
// "no such host") is an honest negative result.
func Reach(network, host string, port int, timeout time.Duration) error {
	if network != "tcp" && network != "udp" {
		return fmt.Errorf("probe: unsupported network %q", network)
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	conn, err := net.DialTimeout(network, addr, timeout)
	if err != nil {
		return fmt.Errorf("probe: %s reach %s: %w", network, addr, err)
	}
	_ = conn.Close()
	return nil
}
