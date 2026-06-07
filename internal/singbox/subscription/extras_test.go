package subscription

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/singbox/vlink"
)

func TestLooksLikeSubscriptionInfo_Localhost(t *testing.T) {
	p := vlink.ParsedOutbound{
		Protocol: "vless",
		Server:   "localhost",
		Port:     80,
		Label:    "📆 Осталось: 8 дней",
		Outbound: mustOutboundJSON(t, map[string]any{
			"type": "vless", "server": "localhost", "server_port": float64(80), "uuid": "xxxxxxxxxx1",
		}),
	}
	ob, _ := outboundMap(p.Outbound)
	if !looksLikeSubscriptionInfo(p, ob) {
		t.Fatal("expected info heuristic for localhost banner")
	}
}

func TestLooksLikeSubscriptionInfo_LatviaNodeNotInfo(t *testing.T) {
	p := vlink.ParsedOutbound{
		Protocol: "vless",
		Server:   "lv1.provider.example",
		Port:     443,
		Label:    "Латвия🇱🇻🟢LTE",
		Outbound: mustOutboundJSON(t, map[string]any{
			"type": "vless", "server": "lv1.provider.example", "server_port": float64(443),
			"uuid": "2f1351f5-5cdf-4f7b-bfc8-44b0ac090a26",
		}),
	}
	ob, _ := outboundMap(p.Outbound)
	if looksLikeSubscriptionInfo(p, ob) {
		t.Fatal("country node name with 🟢LTE suffix must not be classified as provider info")
	}
}

func TestLooksLikeSubscriptionInfo_LTEQuotaBanner(t *testing.T) {
	p := vlink.ParsedOutbound{
		Protocol: "vless",
		Server:   "localhost",
		Port:     80,
		Label:    "📶 LTE трафик: 10 GB",
		Outbound: mustOutboundJSON(t, map[string]any{
			"type": "vless", "server": "localhost", "server_port": float64(80), "uuid": "xxxxxxxxxx1",
		}),
	}
	ob, _ := outboundMap(p.Outbound)
	if !looksLikeSubscriptionInfo(p, ob) {
		t.Fatal("expected LTE traffic banner to be info")
	}
}

func TestPartitionParsedOutbounds_LatviaGoesValidNotInfo(t *testing.T) {
	subID := "b39ba864b5e2462e3e398757"
	p := mkParsedExtras(t, "vless", "lv1.provider.example", 443, "2f1351f5-5cdf-4f7b-bfc8-44b0ac090a26", "Латвия🇱🇻🟢LTE")
	parts := partitionParsedOutbounds(subID, []vlink.ParsedOutbound{p})
	if len(parts.Info) != 0 {
		t.Fatalf("info=%+v want none for Latvia node", parts.Info)
	}
	if len(parts.Valid) != 1 {
		t.Fatalf("valid=%d want 1", len(parts.Valid))
	}
}

func TestPartitionParsedOutbounds_InfoAndValid(t *testing.T) {
	subID := "b39ba864b5e2462e3e398757"
	lines := []vlink.ParsedOutbound{
		mkParsedExtras(t, "vless", "localhost", 80, "xxxxxxxxxx1", "📆 Осталось: 8 дней"),
		mkParsedExtras(t, "vless", "92.255.79.106", 443, "2f1351f5-5cdf-4f7b-bfc8-44b0ac090a26", "UK"),
	}
	parts := partitionParsedOutbounds(subID, lines)
	if len(parts.Info) != 1 {
		t.Fatalf("info=%d want 1", len(parts.Info))
	}
	if len(parts.Valid) != 1 {
		t.Fatalf("valid=%d want 1", len(parts.Valid))
	}
	if len(parts.Rejected) != 0 {
		t.Fatalf("rejected=%v", parts.Rejected)
	}
}

func TestPartitionParsedOutbounds_InvalidGoesRejected(t *testing.T) {
	subID := "testsub1"
	p := mkParsedExtras(t, "vless", "evil.example", 443, "not-a-uuid", "bad")
	parts := partitionParsedOutbounds(subID, []vlink.ParsedOutbound{p})
	if len(parts.Rejected) != 1 {
		t.Fatalf("rejected=%v", parts.Rejected)
	}
	if len(parts.Info) != 0 {
		t.Fatalf("info=%v want none for non-banner invalid link", parts.Info)
	}
	if !strings.Contains(parts.Rejected[0].Reason, "uuid") {
		t.Fatalf("reason=%q", parts.Rejected[0].Reason)
	}
}

func TestPartitionParsedOutbounds_InvalidBannerGoesInfoNotRejected(t *testing.T) {
	subID := "testsub1"
	p := mkParsedExtras(t, "vless", "evil.example", 443, "not-a-uuid", "📆 Осталось: 3 дня")
	parts := partitionParsedOutbounds(subID, []vlink.ParsedOutbound{p})
	if len(parts.Info) != 1 {
		t.Fatalf("info=%v want banner despite invalid uuid", parts.Info)
	}
	if len(parts.Rejected) != 0 {
		t.Fatalf("rejected=%v", parts.Rejected)
	}
}

func TestPartitionParsedOutbounds_DedupesRejectedByTag(t *testing.T) {
	subID := "c0cf56f3cc03cba2"
	dup := mkParsedExtras(t, "vless", "evil.example", 443, "not-a-uuid", "banner A")
	dup2 := mkParsedExtras(t, "vless", "evil.example", 443, "not-a-uuid", "banner B")
	parts := partitionParsedOutbounds(subID, []vlink.ParsedOutbound{dup, dup2})
	if len(parts.Rejected) != 1 {
		t.Fatalf("rejected=%d want 1 (same StableTag), got %+v", len(parts.Rejected), parts.Rejected)
	}
}

func TestFilterDismissedInfo(t *testing.T) {
	items := []SubscriptionInfoItem{
		{ID: "a", Label: "keep"},
		{ID: "b", Label: "hide"},
	}
	got := filterDismissedInfo(items, []string{"b"})
	if len(got) != 1 || got[0].ID != "a" {
		t.Fatalf("got %+v", got)
	}
}

func TestMergeInfoItems_UserPinnedWins(t *testing.T) {
	pinned := []SubscriptionInfoItem{{ID: "a", Label: "user pin", Source: "user"}}
	auto := []SubscriptionInfoItem{{ID: "b", Label: "auto", Source: "auto"}, {ID: "c", Label: "c", Source: "auto"}}
	got := mergeInfoItems(pinned, auto)
	if len(got) != 3 || got[0].Label != "user pin" {
		t.Fatalf("got %+v", got)
	}
}

func TestMergeInfoItems_CapsAtMax(t *testing.T) {
	pinned := []SubscriptionInfoItem{
		{ID: "u1", Label: "pin 1", Source: "user"},
		{ID: "u2", Label: "pin 2", Source: "user"},
		{ID: "u3", Label: "pin 3", Source: "user"},
	}
	auto := []SubscriptionInfoItem{
		{ID: "a1", Label: "auto 1", Source: "auto"},
		{ID: "a2", Label: "auto 2", Source: "auto"},
		{ID: "a3", Label: "auto 3", Source: "auto"},
	}
	got := mergeInfoItems(pinned, auto)
	if len(got) != MaxSubscriptionInfoItems {
		t.Fatalf("len=%d want %d: %+v", len(got), MaxSubscriptionInfoItems, got)
	}
	for i, want := range []string{"u1", "u2", "u3", "a1"} {
		if got[i].ID != want {
			t.Fatalf("got[%d].ID=%q want %q", i, got[i].ID, want)
		}
	}
}

func TestPartitionParsedOutbounds_InfoSlotFullGoesRejected(t *testing.T) {
	subID := "b39ba864b5e2462e3e398757"
	lines := make([]vlink.ParsedOutbound, 0, MaxSubscriptionInfoItems+1)
	for i := 0; i < MaxSubscriptionInfoItems+1; i++ {
		lines = append(lines, mkParsedExtras(t, "vless", "localhost", 80+i, "xxxxxxxxxx1",
			fmt.Sprintf("📆 Осталось: %d дней", i+1)))
	}
	parts := partitionParsedOutbounds(subID, lines)
	if len(parts.Info) != MaxSubscriptionInfoItems {
		t.Fatalf("info=%d want %d", len(parts.Info), MaxSubscriptionInfoItems)
	}
	if len(parts.Rejected) != 1 {
		t.Fatalf("rejected=%+v want 1 overflow", parts.Rejected)
	}
	if !strings.Contains(parts.Rejected[0].Reason, "info slot full") {
		t.Fatalf("reason=%q", parts.Rejected[0].Reason)
	}
}

func TestRejectedFromPrunedTags(t *testing.T) {
	sub := &Subscription{
		Members: []MemberInfo{
			{Tag: "sub-demo-aaaabbbb", Label: "UK node", Protocol: "vless", Server: "uk.example", Port: 443},
		},
	}
	got := rejectedFromPrunedTags(sub, []string{"sub-demo-aaaabbbb", "sub-demo-deadbeef"})
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	if got[0].Label != "UK node" || got[0].Reason != "not materialized in sing-box config" {
		t.Fatalf("known pruned: %+v", got[0])
	}
	if got[1].Tag != "sub-demo-deadbeef" || got[1].Label != "" {
		t.Fatalf("unknown pruned: %+v", got[1])
	}
}

func mkParsedExtras(t *testing.T, proto, server string, port int, uuid, label string) vlink.ParsedOutbound {
	t.Helper()
	raw := mustOutboundJSON(t, map[string]any{
		"type": proto, "server": server, "server_port": float64(port), "uuid": uuid,
	})
	return vlink.ParsedOutbound{
		Protocol: proto,
		Server:   server,
		Port:     uint16(port),
		Label:    label,
		Outbound: raw,
	}
}

func mustOutboundJSON(t *testing.T, ob map[string]any) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(ob)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
