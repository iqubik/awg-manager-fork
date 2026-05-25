package singbox

import "testing"

// proxyIsOurs decides whether an NDMS ProxyN belongs to awg-manager's sing-box
// management, so disable/orphan-cleanup removes it. Subscription composites are
// the regression case: their proxy carries the subscription *label* as the
// interface description (not a tunnel tag), so the tag/slot heuristics miss it —
// they must be recognised via their explicitly-tracked proxy index.
func TestProxyIsOurs(t *testing.T) {
	tunnelTags := map[string]bool{"vless-1": true}
	ourPortSlots := map[int]bool{3: true}
	subProxyIdx := map[int]bool{7: true}

	cases := []struct {
		name string
		idx  int
		desc string
		want bool
	}{
		{"tunnel matched by description tag", 0, "vless-1", true},
		{"tunnel matched by port slot (empty desc)", 3, "", true},
		{"subscription composite (label description)", 7, "Моя подписка", true},
		{"foreign proxy with description", 9, "some-other-app", false},
		{"foreign proxy empty desc unknown slot", 5, "", false},
	}
	for _, c := range cases {
		got := proxyIsOurs(c.idx, c.desc, tunnelTags, ourPortSlots, subProxyIdx)
		if got != c.want {
			t.Errorf("%s: proxyIsOurs(%d, %q) = %v, want %v", c.name, c.idx, c.desc, got, c.want)
		}
	}
}
