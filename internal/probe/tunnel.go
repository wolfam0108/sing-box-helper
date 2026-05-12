package probe

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ipReporter is the public service we curl to ask "what IP do you see?".
// It must answer over plain HTTPS with a single-line IP, no JSON.
const ipReporter = "https://api.ipify.org"

// DirectIP curls ipReporter through the router's default route — i.e.
// without going through any tunnel. Used to compare against TunnelIP and
// confirm the tunnel actually changes the egress.
func DirectIP(timeout time.Duration) (string, error) {
	return runCurl(timeout, nil)
}

// TunnelIP curls ipReporter binding the socket to the named interface
// (typically "singtun"), so the traffic goes through sing-box's TUN
// inbound and out via the configured outbound.
//
// Returns ("", err) if curl fails — including the "TLS connect error"
// case observed with xhttp nodes on sing-box 1.13, where the outbound
// connects but the protocol handshake gets rejected.
func TunnelIP(iface string, timeout time.Duration) (string, error) {
	if iface == "" {
		return "", fmt.Errorf("probe: empty interface name")
	}
	return runCurl(timeout, []string{"--interface", iface})
}

func runCurl(timeout time.Duration, extra []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout+2*time.Second)
	defer cancel()

	args := []string{
		"-sS",
		"--max-time", strconv.Itoa(int(timeout.Seconds())),
	}
	args = append(args, extra...)
	args = append(args, ipReporter)

	cmd := exec.CommandContext(ctx, "curl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("curl: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}
