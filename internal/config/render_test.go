package config

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/wolfram0108/sing-box-helper/internal/parser"
)

// renderForURI is a tiny helper: parse a URI with the dispatcher and render
// the config with default settings, returning the JSON-as-map for easy
// path assertions.
func renderForURI(t *testing.T, uri string) map[string]any {
	t.Helper()
	pn, err := parser.Parse(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	raw, err := Render(pn, DefaultSettings())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal back: %v\nraw:\n%s", err, raw)
	}
	return got
}

// must casts an interface to T with a fatal error on mismatch.
func mustMap(t *testing.T, v any, path string) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%s: not a map, got %T", path, v)
	}
	return m
}

func mustSlice(t *testing.T, v any, path string) []any {
	t.Helper()
	s, ok := v.([]any)
	if !ok {
		t.Fatalf("%s: not a slice, got %T", path, v)
	}
	return s
}

func TestRender_FixedSkeleton(t *testing.T) {
	got := renderForURI(t, "hysteria2://pw@example.com:443")

	// log
	log := mustMap(t, got["log"], "log")
	if log["level"] != "info" {
		t.Errorf("log.level = %v, want info", log["level"])
	}
	if log["timestamp"] != true {
		t.Errorf("log.timestamp = %v", log["timestamp"])
	}

	// dns
	dns := mustMap(t, got["dns"], "dns")
	if dns["strategy"] != "ipv4_only" {
		t.Errorf("dns.strategy = %v", dns["strategy"])
	}
	dnsServers := mustSlice(t, dns["servers"], "dns.servers")
	if len(dnsServers) != 1 {
		t.Fatalf("dns.servers length = %d", len(dnsServers))
	}
	dns0 := mustMap(t, dnsServers[0], "dns.servers[0]")
	if dns0["server"] != "1.1.1.1" || dns0["tag"] != "dns-direct" || dns0["type"] != "udp" {
		t.Errorf("dns.servers[0] = %+v", dns0)
	}

	// inbounds
	inbounds := mustSlice(t, got["inbounds"], "inbounds")
	if len(inbounds) != 2 {
		t.Fatalf("inbounds length = %d, want 2 (tun + mixed)", len(inbounds))
	}
	tun := mustMap(t, inbounds[0], "inbounds[0]")
	if tun["type"] != "tun" {
		t.Errorf("inbounds[0].type = %v", tun["type"])
	}
	if tun["interface_name"] != "singtun" {
		t.Errorf("inbounds[0].interface_name = %v", tun["interface_name"])
	}
	if tun["auto_route"] != false {
		t.Errorf("CRITICAL: inbounds[0].auto_route = %v, must be false (keen-pbr conflict)", tun["auto_route"])
	}
	if tun["strict_route"] != false {
		t.Errorf("inbounds[0].strict_route = %v", tun["strict_route"])
	}
	if tun["sniff"] != true {
		t.Errorf("inbounds[0].sniff = %v", tun["sniff"])
	}
	addr := mustSlice(t, tun["address"], "inbounds[0].address")
	if len(addr) != 1 || addr[0] != "198.18.0.1/30" {
		t.Errorf("inbounds[0].address = %v", addr)
	}
	mixed := mustMap(t, inbounds[1], "inbounds[1]")
	if mixed["type"] != "mixed" || mixed["tag"] != "test-proxy" {
		t.Errorf("inbounds[1] = %+v", mixed)
	}

	// outbounds
	outbounds := mustSlice(t, got["outbounds"], "outbounds")
	if len(outbounds) != 2 {
		t.Fatalf("outbounds length = %d, want 2 (proxy + direct)", len(outbounds))
	}
	proxy := mustMap(t, outbounds[0], "outbounds[0]")
	if proxy["tag"] != "proxy" {
		t.Errorf("outbounds[0].tag = %v, must be 'proxy'", proxy["tag"])
	}
	direct := mustMap(t, outbounds[1], "outbounds[1]")
	if direct["type"] != "direct" || direct["tag"] != "direct" {
		t.Errorf("outbounds[1] = %+v", direct)
	}

	// route
	route := mustMap(t, got["route"], "route")
	if route["final"] != "proxy" {
		t.Errorf("route.final = %v, want proxy", route["final"])
	}
	rules := mustSlice(t, route["rules"], "route.rules")
	if len(rules) != 2 {
		t.Errorf("route.rules length = %d", len(rules))
	}

	// experimental.clash_api
	exp := mustMap(t, got["experimental"], "experimental")
	clash := mustMap(t, exp["clash_api"], "experimental.clash_api")
	if clash["external_controller"] != "0.0.0.0:9090" {
		t.Errorf("clash_api.external_controller = %v", clash["external_controller"])
	}
}

func TestRender_Hysteria2(t *testing.T) {
	got := renderForURI(t,
		"hysteria2://OkGilZTuWOaHp7ii6XMyRf@np.mywolfram.ru:2053")

	proxy := mustMap(t, mustSlice(t, got["outbounds"], "outbounds")[0], "proxy")
	if proxy["type"] != "hysteria2" {
		t.Errorf("type = %v", proxy["type"])
	}
	if proxy["server"] != "np.mywolfram.ru" {
		t.Errorf("server = %v", proxy["server"])
	}
	if proxy["server_port"].(float64) != 2053 {
		t.Errorf("server_port = %v", proxy["server_port"])
	}
	if proxy["password"] != "OkGilZTuWOaHp7ii6XMyRf" {
		t.Errorf("password = %v", proxy["password"])
	}
	tls := mustMap(t, proxy["tls"], "proxy.tls")
	if tls["enabled"] != true || tls["server_name"] != "np.mywolfram.ru" {
		t.Errorf("tls = %+v", tls)
	}
}

func TestRender_VLESSReality(t *testing.T) {
	got := renderForURI(t,
		"vless://a0e5fd22-4aba-4861-a730-2a5b187424cd@ae.mywolfram.ru:58871"+
			"?type=tcp&encryption=none&security=reality&pbk=tM94SWNzuc___dzCxakr-0F2KF_GlJMIbM0eFtbYsG8"+
			"&fp=chrome&sni=yahoo.com&sid=42f2121ac05879a3&spx=%2F&flow=xtls-rprx-vision"+
			"#users-wolframM1")

	proxy := mustMap(t, mustSlice(t, got["outbounds"], "outbounds")[0], "proxy")
	if proxy["type"] != "vless" || proxy["tag"] != "proxy" {
		t.Errorf("type/tag = %v/%v", proxy["type"], proxy["tag"])
	}
	if proxy["uuid"] != "a0e5fd22-4aba-4861-a730-2a5b187424cd" {
		t.Errorf("uuid = %v", proxy["uuid"])
	}
	if proxy["flow"] != "xtls-rprx-vision" {
		t.Errorf("flow = %v", proxy["flow"])
	}
	tls := mustMap(t, proxy["tls"], "proxy.tls")
	if tls["server_name"] != "yahoo.com" {
		t.Errorf("sni = %v", tls["server_name"])
	}
	reality := mustMap(t, tls["reality"], "proxy.tls.reality")
	if reality["public_key"] != "tM94SWNzuc___dzCxakr-0F2KF_GlJMIbM0eFtbYsG8" {
		t.Errorf("public_key = %v", reality["public_key"])
	}
	if reality["short_id"] != "42f2121ac05879a3" {
		t.Errorf("short_id = %v", reality["short_id"])
	}
	utls := mustMap(t, tls["utls"], "proxy.tls.utls")
	if utls["fingerprint"] != "chrome" {
		t.Errorf("utls.fingerprint = %v", utls["fingerprint"])
	}
}

func TestRender_VLESSXHTTPReality(t *testing.T) {
	got := renderForURI(t,
		"vless://6a99b2ec-0d60-4607-acaa-bf666f29a787@ae.mywolfram.ru:39435"+
			"?type=xhttp&encryption=none&path=%2F&host=&mode=auto&security=reality"+
			"&pbk=m1oonmPcmTO2kZLm0_vfN8D3YQ_8FrXkLOLYudI4tmA&fp=edge&sni=ya.ru"+
			"&sid=f064aec4&spx=%2F#VLESS-XHTTP-test-user-01")

	proxy := mustMap(t, mustSlice(t, got["outbounds"], "outbounds")[0], "proxy")
	tr := mustMap(t, proxy["transport"], "proxy.transport")
	if tr["type"] != "httpupgrade" {
		t.Errorf("transport.type = %v, want httpupgrade (mapped from xhttp)", tr["type"])
	}
	if tr["path"] != "/" {
		t.Errorf("transport.path = %v", tr["path"])
	}
	tls := mustMap(t, proxy["tls"], "proxy.tls")
	if tls["server_name"] != "ya.ru" {
		t.Errorf("sni = %v", tls["server_name"])
	}
	utls := mustMap(t, tls["utls"], "proxy.tls.utls")
	if utls["fingerprint"] != "edge" {
		t.Errorf("utls.fingerprint = %v", utls["fingerprint"])
	}
}

func TestRender_SOCKS5_LocalNoAuth(t *testing.T) {
	got := renderForURI(t, "socks5://127.0.0.1:1080#mieru-local")
	proxy := mustMap(t, mustSlice(t, got["outbounds"], "outbounds")[0], "proxy")
	if proxy["type"] != "socks" || proxy["tag"] != "proxy" {
		t.Errorf("type/tag = %v/%v", proxy["type"], proxy["tag"])
	}
	if proxy["server"] != "127.0.0.1" || proxy["server_port"].(float64) != 1080 {
		t.Errorf("server/port: %+v", proxy)
	}
	if proxy["version"] != "5" {
		t.Errorf("version = %v", proxy["version"])
	}
	if _, has := proxy["username"]; has {
		t.Errorf("username must be absent when no auth: %v", proxy["username"])
	}
	if _, has := proxy["password"]; has {
		t.Errorf("password must be absent when no auth: %v", proxy["password"])
	}
	if _, has := proxy["tls"]; has {
		t.Errorf("tls must be absent for plain socks: %v", proxy["tls"])
	}
}

func TestRender_SOCKS5_WithAuth(t *testing.T) {
	got := renderForURI(t, "socks5://alice:s3cret@127.0.0.1:1080")
	proxy := mustMap(t, mustSlice(t, got["outbounds"], "outbounds")[0], "proxy")
	if proxy["username"] != "alice" || proxy["password"] != "s3cret" {
		t.Errorf("auth: %+v", proxy)
	}
}

func TestRender_DisabledOptions(t *testing.T) {
	pn, err := parser.Parse("hysteria2://pw@host:443")
	if err != nil {
		t.Fatal(err)
	}
	s := DefaultSettings()
	s.EnableMixed = false
	s.EnableClashAPI = false

	raw, err := Render(pn, s)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	_ = json.Unmarshal(raw, &got)

	inbounds := mustSlice(t, got["inbounds"], "inbounds")
	if len(inbounds) != 1 {
		t.Errorf("inbounds length = %d, want 1 (only tun)", len(inbounds))
	}
	if _, exists := got["experimental"]; exists {
		t.Errorf("experimental key must be absent when EnableClashAPI=false, got %v", got["experimental"])
	}
}

func TestRender_NilNode(t *testing.T) {
	_, err := Render(nil, DefaultSettings())
	if err == nil || !strings.Contains(err.Error(), "nil") {
		t.Errorf("err = %v, want nil-node error", err)
	}
}
