package router

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "20-router.json")

	cfg := &RouterConfig{
		Inbounds: []Inbound{{
			Type: "tproxy", Tag: "tproxy-in", Listen: "127.0.0.1",
			ListenPort: 51271, Network: "tcp,udp", UDPTimeout: "5m", RoutingMark: 1,
		}},
		Outbounds: []Outbound{
			{Type: "direct", Tag: "awg10", BindInterface: "opkgtun10"},
		},
		Route: Route{
			RuleSet: []RuleSet{
				{Tag: "geosite-youtube", Type: "remote", Format: "binary",
					URL: "https://example.com/geosite-youtube.srs", UpdateInterval: "24h"},
			},
			Rules: []Rule{
				{Action: "sniff"},
				{RuleSet: []string{"geosite-youtube"}, Action: "route", Outbound: "awg10"},
			},
			Final: "direct",
		},
	}

	if err := SaveConfig(path, cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Inbounds) != 1 || loaded.Inbounds[0].Tag != "tproxy-in" {
		t.Errorf("inbounds: %+v", loaded.Inbounds)
	}
	if len(loaded.Outbounds) != 1 || loaded.Outbounds[0].BindInterface != "opkgtun10" {
		t.Errorf("outbounds: %+v", loaded.Outbounds)
	}
	if loaded.Route.Final != "direct" {
		t.Errorf("final: %q", loaded.Route.Final)
	}
	if len(loaded.Route.Rules) != 2 || len(loaded.Route.RuleSet) != 1 {
		t.Errorf("route: %+v", loaded.Route)
	}
}

func TestLoadConfigMissingReturnsEmpty(t *testing.T) {
	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "nonexistent.json"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || len(cfg.Inbounds) != 0 || len(cfg.Outbounds) != 0 {
		t.Error("expected empty config")
	}
}

func TestSaveProducesValidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "20-router.json")
	if err := SaveConfig(path, NewEmptyConfig()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	for _, k := range []string{"inbounds", "outbounds", "route"} {
		if _, ok := parsed[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
}

func TestNewEmptyConfig_FinalIsDirect(t *testing.T) {
	cfg := NewEmptyConfig()
	if cfg.Route.Final != "direct" {
		t.Errorf("expected Final='direct', got %q", cfg.Route.Final)
	}
}

func TestEnsureSystemRules_EnforcesFinal(t *testing.T) {
	cfg := NewEmptyConfig()
	cfg.Route.Final = ""
	cfg.EnsureSystemRules(true)
	if cfg.Route.Final != "direct" {
		t.Errorf("EnsureSystemRules should set Final='direct' when empty, got %q", cfg.Route.Final)
	}
}

func TestEnsureSystemRules_PreservesCustomFinal(t *testing.T) {
	cfg := NewEmptyConfig()
	cfg.Route.Final = "my-vpn"
	cfg.EnsureSystemRules(true)
	if cfg.Route.Final != "my-vpn" {
		t.Errorf("EnsureSystemRules should preserve non-empty Final, got %q", cfg.Route.Final)
	}
}

func TestSetRouteFinal_RejectsEmpty(t *testing.T) {
	cfg := NewEmptyConfig()
	if err := cfg.SetRouteFinal(""); err == nil {
		t.Error("expected SetRouteFinal('') to error")
	}
}

func TestRenameOutboundReferences_RewritesEveryReference(t *testing.T) {
	cfg := NewEmptyConfig()
	cfg.Route.Final = "old"
	cfg.Route.Rules = []Rule{
		{Action: "route", Outbound: "old"},
		{Type: "logical", Mode: "or", Rules: []Rule{{Action: "route", Outbound: "old"}}},
	}
	cfg.Outbounds = []Outbound{
		{Type: "selector", Tag: "group", Outbounds: []string{"old", "other"}, Default: "old"},
	}
	cfg.DNS.Servers = []DNSServer{
		{Tag: "dns", Type: "https", Server: "dns.example", Detour: "old"},
	}
	cfg.Route.RuleSet = []RuleSet{
		{Tag: "geo", Type: "remote", URL: "https://example.com/geo.srs", DownloadDetour: "old"},
	}

	cfg.renameOutboundReferences("old", "new")

	if cfg.Route.Final != "new" {
		t.Fatalf("route.final = %q, want new", cfg.Route.Final)
	}
	if cfg.Route.Rules[0].Outbound != "new" || cfg.Route.Rules[1].Rules[0].Outbound != "new" {
		t.Fatalf("rules = %+v", cfg.Route.Rules)
	}
	if cfg.Outbounds[0].Outbounds[0] != "new" || cfg.Outbounds[0].Default != "new" {
		t.Fatalf("composite outbound = %+v", cfg.Outbounds[0])
	}
	if cfg.DNS.Servers[0].Detour != "new" {
		t.Fatalf("dns detour = %q, want new", cfg.DNS.Servers[0].Detour)
	}
	if cfg.Route.RuleSet[0].DownloadDetour != "new" {
		t.Fatalf("download_detour = %q, want new", cfg.Route.RuleSet[0].DownloadDetour)
	}
}

func TestStripAutoManagedDirect(t *testing.T) {
	in := []Outbound{
		{Type: "direct", Tag: "legacy-a", BindInterface: "t2s0"},     // AWG kernel — strip
		{Type: "direct", Tag: "direct"},                              // no bind_interface — keep
		{Type: "selector", Tag: "comp", Outbounds: []string{"awg-x"}}, // composite — keep
		{Type: "direct", Tag: "legacy-b", BindInterface: "nwg0"},     // NativeWG — strip
		{Type: "direct", Tag: "ipsec-vpn", BindInterface: "ipsec0"},  // user VPN — keep
		{Type: "direct", Tag: "ike", BindInterface: "ike0"},          // user VPN — keep
	}
	got := stripAutoManagedDirect(in)
	if len(got) != 4 {
		t.Fatalf("want 4 kept, got %d (%+v)", len(got), got)
	}
	tags := map[string]bool{}
	for _, o := range got {
		tags[o.Tag] = true
	}
	for _, want := range []string{"direct", "comp", "ipsec-vpn", "ike"} {
		if !tags[want] {
			t.Errorf("expected %q kept, missing from %+v", want, got)
		}
	}
	if tags["legacy-a"] || tags["legacy-b"] {
		t.Errorf("auto-managed AWG/WG direct outbounds should be stripped: %+v", got)
	}
}

func TestIsAutoManagedIface(t *testing.T) {
	managed := []string{"opkgtun10", "awgm0", "awg-x", "wg0", "wireguard0", "nwg1", "t2s0", "Proxy3"}
	for _, n := range managed {
		if !IsAutoManagedIface(n) {
			t.Errorf("IsAutoManagedIface(%q) = false, want true", n)
		}
	}
	user := []string{"ipsec0", "ike0", "sstp0", "ppp0", "eth3", "L2TP0"}
	for _, n := range user {
		if IsAutoManagedIface(n) {
			t.Errorf("IsAutoManagedIface(%q) = true, want false", n)
		}
	}
}

func TestRuleUnmarshalJSON_PortScalar(t *testing.T) {
	// sing-box allows "port": 53 (scalar), our struct expects []int.
	raw := `{"port": 53, "action": "route", "outbound": "direct"}`
	var r Rule
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("unmarshal scalar port: %v", err)
	}
	if len(r.Port) != 1 || r.Port[0] != 53 {
		t.Errorf("want Port=[53], got %v", r.Port)
	}
}

func TestRuleUnmarshalJSON_PortArray(t *testing.T) {
	raw := `{"port": [80, 443], "action": "route", "outbound": "proxy"}`
	var r Rule
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("unmarshal array port: %v", err)
	}
	if len(r.Port) != 2 || r.Port[0] != 80 || r.Port[1] != 443 {
		t.Errorf("want Port=[80,443], got %v", r.Port)
	}
}

func TestRuleUnmarshalJSON_LogicalWithScalarPort(t *testing.T) {
	// The exact shape that triggers the original crash: a logical rule
	// with a nested sub-rule containing "port": 53 (scalar).
	raw := `{
		"type": "logical",
		"mode": "or",
		"rules": [
			{"protocol": "dns"},
			{"port": 53}
		],
		"action": "hijack-dns"
	}`
	var r Rule
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatalf("unmarshal logical rule with scalar port: %v", err)
	}
	if r.Type != "logical" || len(r.Rules) != 2 {
		t.Fatalf("unexpected shape: %+v", r)
	}
	nested := r.Rules[1]
	if len(nested.Port) != 1 || nested.Port[0] != 53 {
		t.Errorf("nested rule: want Port=[53], got %v", nested.Port)
	}
}

func TestOutboundReferencesExcludingRules(t *testing.T) {
	cfg := &RouterConfig{
		Outbounds: []Outbound{
			{Type: "selector", Tag: "sel", Outbounds: []string{"awg-del"}, Default: "awg-del"},
		},
		DNS: DNS{
			Servers: []DNSServer{{Tag: "dns1", Detour: "awg-del"}},
		},
		Route: Route{
			Rules:   []Rule{{Outbound: "awg-del"}},
			Final:   "awg-del",
			RuleSet: []RuleSet{{Tag: "rs1", DownloadDetour: "awg-del"}},
		},
	}

	locs := cfg.outboundReferencesExcludingRules("awg-del")

	for _, l := range locs {
		if strings.HasPrefix(l, "route.rules[") {
			t.Errorf("route.rules entry must be excluded, got %q", l)
		}
	}
	want := map[string]bool{
		"route.final":                             false,
		`outbounds[0="sel"].outbounds[0]`:         false,
		`outbounds[0="sel"].default`:              false,
		`dns.servers[0="dns1"].detour`:            false,
		`route.rule_set[0="rs1"].download_detour`: false,
	}
	for _, l := range locs {
		if _, ok := want[l]; !ok {
			t.Errorf("unexpected location %q", l)
			continue
		}
		want[l] = true
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("missing expected location %q", k)
		}
	}
}

func TestOutboundReferencesExcludingRules_OnlyRule(t *testing.T) {
	cfg := &RouterConfig{
		Route: Route{Rules: []Rule{{Outbound: "awg-del"}}},
	}
	if locs := cfg.outboundReferencesExcludingRules("awg-del"); len(locs) != 0 {
		t.Errorf("expected empty (rule-only refs excluded), got %v", locs)
	}
}

func TestValidateOutbound_Direct(t *testing.T) {
	// happy path
	if err := validateOutbound(Outbound{Type: "direct", Tag: "ipsec-vpn", BindInterface: "ipsec0"}); err != nil {
		t.Errorf("valid direct rejected: %v", err)
	}
	// missing tag
	if err := validateOutbound(Outbound{Type: "direct", BindInterface: "ipsec0"}); err == nil {
		t.Error("direct without tag should fail")
	}
	// missing bind_interface
	if err := validateOutbound(Outbound{Type: "direct", Tag: "x"}); err == nil {
		t.Error("direct without bind_interface should fail")
	}
	// stray composite fields
	if err := validateOutbound(Outbound{Type: "direct", Tag: "x", BindInterface: "ipsec0", Outbounds: []string{"a"}}); err == nil {
		t.Error("direct with members should fail")
	}
	if err := validateOutbound(Outbound{Type: "direct", Tag: "x", BindInterface: "ipsec0", Default: "a"}); err == nil {
		t.Error("direct with default should fail")
	}
}

func TestValidateOutbound_CompositeStillWorks(t *testing.T) {
	if err := validateOutbound(Outbound{Type: "selector", Tag: "g", Outbounds: []string{"a", "b"}, Default: "a"}); err != nil {
		t.Errorf("valid selector rejected: %v", err)
	}
	if err := validateOutbound(Outbound{Type: "urltest", Tag: "g"}); err == nil {
		t.Error("composite without members should fail")
	}
}
