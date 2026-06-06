package subscription

import (
	"testing"

	"github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

// A urltest group references a member tag ("gone") that no slot declares.
// The cross-slot validator reports it as unknown-outbound for the
// subscriptions slot; cleanCrossSlotUnknownRefs must drop the dangling
// member ref (and only it) so flush can retry instead of dead-ending.
func TestCleanCrossSlotUnknownRefs(t *testing.T) {
	cfg := &slotConfig{
		Outbounds: []any{
			map[string]any{"type": "vless", "tag": "s1"},
			map[string]any{"type": "vless", "tag": "s2"},
			map[string]any{"type": "urltest", "tag": "g", "outbounds": []any{"s1", "s2", "gone"}},
		},
	}
	res := orchestrator.ValidationResult{Errors: []orchestrator.ValidationError{
		{Slot: orchestrator.SlotSubscriptions, Kind: "unknown-outbound", Tag: "gone", InRule: `outbounds[2="g"].outbounds[2]`},
		{Slot: "router", Kind: "unknown-outbound", Tag: "other"},          // different slot — not ours to clean
		{Slot: orchestrator.SlotSubscriptions, Kind: "duplicate-outbound", Tag: "s1"}, // wrong kind — ignore
	}}

	cleaned := cleanCrossSlotUnknownRefs(cfg, res)

	if len(cleaned) != 1 || cleaned[0] != "gone" {
		t.Fatalf("cleaned = %v, want [gone]", cleaned)
	}
	g, _ := cfg.Outbounds[2].(map[string]any)
	members, _ := g["outbounds"].([]any)
	if len(members) != 2 || stringOf(members[0]) != "s1" || stringOf(members[1]) != "s2" {
		t.Fatalf("group members = %v, want [s1 s2]", members)
	}
}

// When the group loses ALL members to dangling-ref cleanup, the group
// itself is dropped (cascade) — verifies cleanReferencesToTag cascade is
// reached via the cross-slot path.
func TestCleanCrossSlotUnknownRefs_DropsEmptyGroup(t *testing.T) {
	cfg := &slotConfig{
		Outbounds: []any{
			map[string]any{"type": "urltest", "tag": "g", "outbounds": []any{"gone1", "gone2"}},
		},
	}
	res := orchestrator.ValidationResult{Errors: []orchestrator.ValidationError{
		{Slot: orchestrator.SlotSubscriptions, Kind: "unknown-outbound", Tag: "gone1"},
		{Slot: orchestrator.SlotSubscriptions, Kind: "unknown-outbound", Tag: "gone2"},
	}}

	cleaned := cleanCrossSlotUnknownRefs(cfg, res)

	if len(cleaned) != 2 {
		t.Fatalf("cleaned = %v, want 2 tags", cleaned)
	}
	if len(cfg.Outbounds) != 0 {
		t.Fatalf("empty group should be dropped, got %d outbounds", len(cfg.Outbounds))
	}
}

// Root-cause-B: a server flush() dropped (not in the declared/emitted set)
// must be pruned from the stored members, so later group rebuilds don't
// re-reference it as a dangling member. ActiveMember reconciliation is left
// to Store.SetMembers; here we only verify the member pruning.
func TestFilterDeclaredMembers(t *testing.T) {
	members := []MemberInfo{{Tag: "s1"}, {Tag: "s2"}, {Tag: "dropped"}}
	declared := map[string]bool{"s1": true, "s2": true, "grp": true} // "dropped" missing

	kept, droppedTags := filterDeclaredMembers(members, declared)

	if len(kept) != 2 || kept[0].Tag != "s1" || kept[1].Tag != "s2" {
		t.Fatalf("kept = %+v, want [s1 s2]", kept)
	}
	if len(droppedTags) != 1 || droppedTags[0] != "dropped" {
		t.Fatalf("droppedTags = %v, want [dropped]", droppedTags)
	}
}

// When the mutator can't report declared tags (empty), members pass through
// unchanged — never nuke everything on missing data.
func TestFilterDeclaredMembers_EmptyDeclaredIsNoop(t *testing.T) {
	members := []MemberInfo{{Tag: "s1"}, {Tag: "s2"}}
	kept, droppedTags := filterDeclaredMembers(members, map[string]bool{})
	if len(kept) != 2 || len(droppedTags) != 0 {
		t.Fatalf("empty declared must be no-op: kept=%v dropped=%v", kept, droppedTags)
	}
}
