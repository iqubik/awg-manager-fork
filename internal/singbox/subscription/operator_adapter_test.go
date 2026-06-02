package subscription

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"github.com/hoaxisr/awg-manager/internal/singbox/vlink"
)

// Service-link vless entries (non-UUID userinfo, localhost) must be skipped
// without aborting the refresh before real servers are added. Regression for
// subscriptions that prefix info banners before valid share-links.
func TestOperatorAdapter_AddOutbound_SkipsInvalidUUIDBeforeValid(t *testing.T) {
	dir := t.TempDir()
	orch := orchestrator.New(dir, nil)
	if err := orch.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	adapter := NewOperatorAdapter(orch, nil, nil)

	serviceLink := map[string]any{
		"type":        "vless",
		"server":      "localhost",
		"server_port": float64(80),
		"uuid":        "xxxxxxxxxx1",
	}
	serviceJSON, _ := json.Marshal(serviceLink)
	if err := adapter.AddOutbound("sub-test-bad", serviceJSON); err != nil {
		t.Fatalf("AddOutbound(service): %v", err)
	}
	if tags := adapter.DeclaredOutboundTags(); len(tags) != 0 {
		t.Fatalf("service link must not materialize, got tags %v", tags)
	}

	validLink := map[string]any{
		"type":        "vless",
		"server":      "92.255.79.106",
		"server_port": float64(443),
		"uuid":        "2f1351f5-5cdf-4f7b-bfc8-44b0ac090a26",
		"tls": map[string]any{
			"enabled":     true,
			"server_name": "hhg.mediastreamer.online",
			"reality": map[string]any{
				"enabled":    true,
				"public_key": "IkIZDyseOnYgKYyhgPLsDMqnfXLYGZmaTXMDTv3lkgY",
				"short_id":   "6aa9d760",
			},
			"utls": map[string]any{
				"enabled":     true,
				"fingerprint": "chrome",
			},
		},
	}
	validJSON, _ := json.Marshal(validLink)
	if err := adapter.AddOutbound("sub-test-good", validJSON); err != nil {
		t.Fatalf("AddOutbound(valid): %v", err)
	}
	tags := adapter.DeclaredOutboundTags()
	if len(tags) != 1 || tags[0] != "sub-test-good" {
		t.Fatalf("DeclaredOutboundTags = %v, want [sub-test-good]", tags)
	}

	drops := adapter.LastFilterDrops()
	if len(drops) != 1 || !strings.Contains(drops[0].Reason, "uuid") {
		t.Fatalf("LastFilterDrops = %+v, want one invalid uuid drop", drops)
	}
}

func TestOperatorAdapter_AddOutbound_ServiceLinksThenValidShareLinks(t *testing.T) {
	body := []byte(strings.Join([]string{
		"vless://xxxxxxxxxx1@localhost:80?x=1#📆 Осталось: 8 дней",
		"vless://xxxxxxxxxx1@localhost:444#➡️t.me/@awgmanager",
		"vless://xxxxxxxxxx3@localhost:8443?x=1#🟢 Остаток трафика LTE: 67/69GB",
		"vless://2f1351f5-5cdf-4f7b-bfc8-44b0ac090a26@92.255.79.106:443?type=tcp&security=reality&pbk=IkIZDyseOnYgKYyhgPLsDMqnfXLYGZmaTXMDTv3lkgY&fp=chrome&sni=hhg.mediastreamer.online&sid=6aa9d760&spx=%2F&flow=xtls-rprx-vision#UK",
		"vless://780649d3-869d-4413-a211-5210f9b76255@5.129.199.240:443?type=tcp&security=reality&pbk=XhLR-adXQuL-yV80ZPvLlo701PgGg-FmRVpLDGs8Qjo&fp=chrome&sni=k.mediastreamer.online&sid=b39f2d5a2f68&spx=%2F&flow=xtls-rprx-vision#NL",
	}, " "))

	lines := NormalizeBody(body, "text/plain")
	parseRes := vlink.ParseBatch(lines)
	if len(parseRes.Outbounds) != 5 {
		t.Fatalf("parsed %d outbounds, want 5 (3 service + 2 valid)", len(parseRes.Outbounds))
	}

	dir := t.TempDir()
	orch := orchestrator.New(dir, nil)
	if err := orch.Bootstrap(); err != nil {
		t.Fatal(err)
	}
	adapter := NewOperatorAdapter(orch, nil, nil)

	subID := "0e5a2268"
	for _, p := range parseRes.Outbounds {
		tag := StableTag(subID, p)
		jsonWithTag := replaceTag(p.Outbound, tag)
		if err := adapter.AddOutbound(tag, jsonWithTag); err != nil {
			t.Fatalf("AddOutbound %q: %v", tag, err)
		}
	}

	tags := adapter.DeclaredOutboundTags()
	if len(tags) != 2 {
		t.Fatalf("DeclaredOutboundTags = %d %v, want 2 valid servers", len(tags), tags)
	}
}
