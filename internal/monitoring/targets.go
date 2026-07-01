// Package monitoring provides observability-only multi-target probes through
// running tunnels. The matrix view in the UI renders the resulting cells.
//
// This package does NOT influence pingcheck restart logic — it is purely a
// visual extension. The base target list is hardcoded; dynamic targets are
// derived from each tunnel's configured pingcheck target (so the user can
// see how the "active" target performs alongside the base ones).
package monitoring

// Target is a single monitoring probe target.
//
// URL is the preferred test endpoint for sing-box rows (Clash API
// /proxies/<tag>/delay). When empty, the scheduler falls back to the
// same stable generate_204 URL used by the sing-box delay checker.
// AWG rows ignore URL and probe Host directly via HTTP bound to the
// tunnel interface.
type Target struct {
	ID   string `json:"id"`
	Host string `json:"host"`
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// Tunnel is a running tunnel relevant for monitoring (subset of the full
// tunnel record). Built per scheduler tick from the TunnelLister + storage.
type Tunnel struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	IfaceName       string `json:"ifaceName"`
	PingcheckTarget string `json:"pingcheckTarget"` // empty when restart pingcheck disabled
	SelfTarget      string `json:"selfTarget"`      // host the connectivity-check probes; empty when method=disabled/handshake
	SelfMethod      string `json:"selfMethod"`      // "http", "ping", "handshake", "disabled"

	// Source identifies which lister produced this tunnel: "awg",
	// "system", "singbox". Drives row visual hints in the matrix UI.
	Source string `json:"source,omitempty"`
	// Backend is the AWG backend kind for managed tunnels: "kernel" or
	// "nativewg". Empty for non-AWG rows.
	Backend string `json:"backend,omitempty"`
	// AWGVersion is derived from the managed tunnel interface obfuscation
	// params: "awg2.0" | "awg1.5" | "awg1.0" | "wg". Empty for non-AWG rows.
	AWGVersion string `json:"awgVersion,omitempty"`
	// DefaultRoute marks managed AWG tunnels configured as default route.
	DefaultRoute bool `json:"defaultRoute,omitempty"`
	// Subscription marks sing-box rows sourced from subscription members.
	Subscription bool `json:"subscription,omitempty"`
	// Sing-box protocol/security/transport hints used by monitoring badges.
	Protocol  string `json:"protocol,omitempty"`
	Security  string `json:"security,omitempty"`
	Transport string `json:"transport,omitempty"`
	// SingboxTag is the rendered sing-box row tag (e.g. member tag or
	// standalone outbound tag) for Source=="singbox" tunnels; empty
	// otherwise.
	SingboxTag string `json:"singboxTag,omitempty"`
	// ProbeTag is the outbound tag the monitoring scheduler should
	// probe through Clash delay. For regular sing-box rows this equals
	// SingboxTag; for subscription rows it prefers the selector/urltest
	// tag so monitoring matches the stable subscription-card delay path.
	ProbeTag string `json:"probeTag,omitempty"`
	// ClashDelay is the last-recorded sing-box urltest delay (ms) for
	// this tunnel. 0 means: not a urltest member, or no delay recorded
	// yet, or Clash unreachable.
	ClashDelay int `json:"clashDelay,omitempty"`
	// UrltestGroup is the tag of the urltest composite group this
	// tunnel belongs to (when ClashDelay is non-zero). Empty otherwise.
	UrltestGroup string `json:"urltestGroup,omitempty"`
}

// BaseTargets is the hardcoded base list. Together with each tunnel's self
// target this forms the monitoring matrix the user worked with before the
// temporary self-only regression.
var BaseTargets = []Target{
	{ID: "cf-1.1.1.1", Host: "1.1.1.1", Name: "Cloudflare DNS", URL: "https://1.1.1.1/"},
	{ID: "g-8.8.8.8", Host: "8.8.8.8", Name: "Google DNS", URL: "https://8.8.8.8/"},
	// Quad9 HTTPS by bare IP is not a reliable Clash delay target on
	// real sing-box deployments; HTTP by IP stays stable while AWG rows
	// still probe Host directly and ignore URL entirely.
	{ID: "q-9.9.9.9", Host: "9.9.9.9", Name: "Quad9 DNS", URL: "http://9.9.9.9"},
}

// EffectiveTargets returns BaseTargets ∪ unique pingcheck targets ∪ unique
// self-check targets from tunnels. Synthesised entries get id "pc-<host>"
// (restart pingcheck target) or "cc-<host>" (connectivity-check self
// target). Base order is preserved; dynamic entries are appended in tunnel
// iteration order, deduplicated by Host.
func EffectiveTargets(tunnels []Tunnel) []Target {
	seen := make(map[string]bool, len(BaseTargets))
	for _, target := range BaseTargets {
		seen[target.Host] = true
	}
	out := make([]Target, 0, len(BaseTargets)+len(tunnels)*2)
	out = append(out, BaseTargets...)
	for _, tun := range tunnels {
		if tun.PingcheckTarget != "" && !seen[tun.PingcheckTarget] {
			seen[tun.PingcheckTarget] = true
			out = append(out, Target{
				ID:   "pc-" + tun.PingcheckTarget,
				Host: tun.PingcheckTarget,
				Name: tun.PingcheckTarget,
			})
		}
		if tun.SelfTarget != "" && !seen[tun.SelfTarget] {
			seen[tun.SelfTarget] = true
			out = append(out, Target{
				ID:   "cc-" + tun.SelfTarget,
				Host: tun.SelfTarget,
				Name: tun.SelfTarget,
			})
		}
	}
	return out
}
