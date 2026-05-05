package api

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/singbox/subscription"
)

func TestToSubscriptionDTO_PreservesMemberLabel(t *testing.T) {
	in := subscription.Subscription{
		ID:    "sub-abc",
		Label: "Test",
		URL:   "https://example.com",
		Members: []subscription.MemberInfo{
			{Tag: "sub-abc-aaaa", Label: "🇺🇸 LA-1", Protocol: "vless", Server: "h", Port: 443},
			{Tag: "sub-abc-bbbb", Label: "", Protocol: "trojan", Server: "h", Port: 444},
		},
		MemberTags: []string{"sub-abc-aaaa", "sub-abc-bbbb"},
		OrphanTags: []string{},
	}
	dto := toSubscriptionDTO(in)
	if len(dto.Members) != 2 {
		t.Fatalf("Members=%d want 2", len(dto.Members))
	}
	if dto.Members[0].Label != "🇺🇸 LA-1" {
		t.Errorf("Members[0].Label=%q want 🇺🇸 LA-1", dto.Members[0].Label)
	}
	// Verify it serializes correctly with omitempty.
	raw, _ := json.Marshal(dto.Members[0])
	if !strings.Contains(string(raw), `"label":"🇺🇸 LA-1"`) {
		t.Errorf("JSON doesn't contain Label: %s", raw)
	}
	raw2, _ := json.Marshal(dto.Members[1])
	if strings.Contains(string(raw2), `"label"`) {
		t.Errorf("empty Label should be omitted, got: %s", raw2)
	}
}
