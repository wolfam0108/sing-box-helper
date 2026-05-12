package probe

import "testing"

func TestParseIPInetLine_KeeneticBr0(t *testing.T) {
	// Real-world output of `ip -4 addr show br0` on KeeneticOS.
	sample := []byte(
		"5: br0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP qlen 1000\n" +
			"    inet 192.168.10.1/24 brd 192.168.10.255 scope global br0\n" +
			"       valid_lft forever preferred_lft forever\n")
	got := parseIPInetLine(sample)
	if got != "192.168.10.1" {
		t.Errorf("parseIPInetLine = %q, want 192.168.10.1", got)
	}
}

func TestParseIPInetLine_Empty(t *testing.T) {
	if got := parseIPInetLine([]byte("")); got != "" {
		t.Errorf("empty input → %q", got)
	}
}

func TestParseIPInetLine_NoIPv4(t *testing.T) {
	sample := []byte("3: eth0: <BROADCAST,MULTICAST> mtu 1500\n")
	if got := parseIPInetLine(sample); got != "" {
		t.Errorf("no-inet input → %q", got)
	}
}

func TestResolveMixedListen_Passthrough(t *testing.T) {
	v, auto := ResolveMixedListen("127.0.0.1", "br0")
	if v != "127.0.0.1" || auto {
		t.Errorf("got %q auto=%v, want 127.0.0.1 false", v, auto)
	}
	v, auto = ResolveMixedListen("0.0.0.0", "br0")
	if v != "0.0.0.0" || auto {
		t.Errorf("got %q auto=%v, want 0.0.0.0 false", v, auto)
	}
}

func TestResolveMixedListen_EmptyDefaultsToAllIface(t *testing.T) {
	v, auto := ResolveMixedListen("", "br0")
	if v != "0.0.0.0" || auto {
		t.Errorf("got %q auto=%v, want 0.0.0.0 false", v, auto)
	}
}

// "auto" with a missing interface must fall back to 0.0.0.0, autodetected=true.
// We can't reliably stub the exec.Command call portably, so we run against
// an obviously non-existent interface — `ip` will fail, ResolveMixedListen
// must fall back gracefully.
func TestResolveMixedListen_AutoFallback(t *testing.T) {
	v, auto := ResolveMixedListen("auto", "no-such-iface-xyzzy")
	if v != "0.0.0.0" || !auto {
		t.Errorf("got %q auto=%v, want 0.0.0.0 true (fallback)", v, auto)
	}
}
