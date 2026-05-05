package subscription

import (
	"encoding/json"
	"testing"
)

func TestBuildSelector(t *testing.T) {
	memberTags := []string{"sub-abc-1111", "sub-abc-2222", "sub-abc-3333"}
	sel := BuildSelector("sub-abc", memberTags, "sub-abc-1111")
	var ob map[string]any
	json.Unmarshal(sel, &ob)
	if ob["type"] != "selector" || ob["tag"] != "sub-abc" {
		t.Errorf("selector wrong: %+v", ob)
	}
	if def, _ := ob["default"].(string); def != "sub-abc-1111" {
		t.Errorf("default=%v", def)
	}
	outs := ob["outbounds"].([]any)
	if len(outs) != 3 {
		t.Errorf("outbounds len=%d", len(outs))
	}
}

func TestBuildSelector_DefaultsToFirstWhenEmpty(t *testing.T) {
	memberTags := []string{"sub-abc-1111", "sub-abc-2222"}
	sel := BuildSelector("sub-abc", memberTags, "")
	var ob map[string]any
	json.Unmarshal(sel, &ob)
	if def, _ := ob["default"].(string); def != "sub-abc-1111" {
		t.Errorf("default should fall back to first member, got %v", def)
	}
}

func TestBuildMixedInbound(t *testing.T) {
	mb := BuildMixedInbound("sub-abc-in", 11080)
	var ob map[string]any
	json.Unmarshal(mb, &ob)
	if ob["type"] != "mixed" || ob["tag"] != "sub-abc-in" {
		t.Errorf("inbound wrong: %+v", ob)
	}
	if ob["listen"] != "127.0.0.1" || ob["listen_port"] != float64(11080) {
		t.Errorf("listen wrong: %+v", ob)
	}
}

func TestBuildRouteRule(t *testing.T) {
	rr := BuildRouteRule("sub-abc-in", "sub-abc")
	var ob map[string]any
	json.Unmarshal(rr, &ob)
	if ob["inbound"] != "sub-abc-in" || ob["outbound"] != "sub-abc" {
		t.Errorf("route rule wrong: %+v", ob)
	}
}
