package probe

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
)

// LANIP discovers the LAN-side IPv4 address of the router by reading the
// given interface's primary inet address. On Keenetic / Entware that
// interface is "br0" (the LAN bridge), and the address is what should be
// used as MixedListen so the test proxy is reachable from LAN clients
// (192.168.x.1 / 10.x.x.1 / etc.).
//
// Strategy: parse `ip -4 addr show <iface>` output, take the first IPv4.
// We don't use net.InterfaceByName + Addrs because on cross-build mipsle
// targets the netlink syscalls behave differently inside gVisor-style
// stacks; calling `ip` matches what the operator would do interactively
// and works reliably on KeeneticOS.
//
// On any error (iface missing, ip binary missing, no addresses) returns
// ("", err) — the caller falls back to a safe default ("0.0.0.0").
func LANIP(iface string) (string, error) {
	if iface == "" {
		iface = "br0"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "ip", "-4", "addr", "show", iface).Output()
	if err != nil {
		return "", fmt.Errorf("lanip: exec ip: %w", err)
	}
	if ip := parseIPInetLine(out); ip != "" {
		return ip, nil
	}
	return "", fmt.Errorf("lanip: no IPv4 address on %s", iface)
}

// parseIPInetLine scans `ip -4 addr show ...` output for the first
// "inet <addr>/<prefix>" line and returns just the address part.
func parseIPInetLine(out []byte) string {
	for _, raw := range bytes.Split(out, []byte("\n")) {
		line := strings.TrimSpace(string(raw))
		if !strings.HasPrefix(line, "inet ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		cidr := fields[1]
		slash := strings.IndexByte(cidr, '/')
		if slash <= 0 {
			continue
		}
		ip := cidr[:slash]
		if net.ParseIP(ip) == nil {
			continue
		}
		return ip
	}
	return ""
}

// ResolveMixedListen returns the effective listen address for the test
// mixed-proxy.
//
//   - "auto"  → call LANIP(iface); if it fails, return "0.0.0.0" (safe
//     default: still listens, just on every interface).
//   - "" / "0.0.0.0" → "0.0.0.0".
//   - anything else (concrete IP or "127.0.0.1") → returned as-is.
//
// The boolean tells whether the value came from autodetection — callers
// (UI, /api/settings) show that in the form so the user can see the
// resolved IP without overwriting their "auto" preference.
func ResolveMixedListen(configured, iface string) (effective string, autodetected bool) {
	if strings.ToLower(strings.TrimSpace(configured)) == "auto" {
		if ip, err := LANIP(iface); err == nil {
			return ip, true
		}
		return "0.0.0.0", true
	}
	if configured == "" {
		return "0.0.0.0", false
	}
	return configured, false
}
