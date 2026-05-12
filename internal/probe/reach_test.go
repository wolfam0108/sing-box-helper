package probe

import (
	"net"
	"testing"
	"time"
)

func TestProtoNetwork(t *testing.T) {
	cases := map[string]string{
		"vless":      "tcp",
		"trojan":     "tcp",
		"hysteria2":  "udp",
		"hysteria":   "udp",
		"tuic":       "udp",
		"":           "tcp",
		"unknown":    "tcp",
	}
	for in, want := range cases {
		if got := ProtoNetwork(in); got != want {
			t.Errorf("ProtoNetwork(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestReach_TCPLoopback(t *testing.T) {
	// Start a tiny TCP listener and verify Reach connects to it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().(*net.TCPAddr)

	if err := Reach("tcp", "127.0.0.1", addr.Port, 500*time.Millisecond); err != nil {
		t.Errorf("Reach to a live listener failed: %v", err)
	}
}

func TestReach_TCPNoListener(t *testing.T) {
	// Use port 1 which is privileged and never has a listener on dev machines.
	err := Reach("tcp", "127.0.0.1", 1, 200*time.Millisecond)
	if err == nil {
		t.Error("expected error reaching dead port, got nil")
	}
}

func TestReach_BadNetwork(t *testing.T) {
	err := Reach("sctp", "127.0.0.1", 80, time.Second)
	if err == nil {
		t.Fatal("expected error for unsupported network")
	}
}
