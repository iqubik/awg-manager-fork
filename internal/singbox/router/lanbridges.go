package router

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	sysexec "github.com/hoaxisr/awg-manager/internal/sys/exec"
	sysiptables "github.com/hoaxisr/awg-manager/internal/sys/iptables"
)

// LANBridgeMark pairs a Linux bridge name with the NDMS hotspot mark
// that we elevate mark=0 (no-policy) DNS to so NDMS's existing
// _NDM_HOTSPOT_DNSREDIR REDIRECT picks the packet up and forwards it to
// the per-policy ndnproxy port.
type LANBridgeMark struct {
	Bridge string // kernel bridge name, e.g. "br0"
	Mark   string // hex mark, e.g. "0xffffaaa"
}

// DiscoverLANBridges returns (bridge, mark) pairs for every Linux LAN
// bridge that NDMS has at least one _NDM_HOTSPOT_DNSREDIR REDIRECT rule
// for on UDP/TCP --dport 53.
//
// We read from _NDM_HOTSPOT_DNSREDIR (nat table) rather than from
// _NDM_HOTSPOT_PREROUTING_MANGL (mangle table) because segment-policy
// binding in NDMS is optional: a bridge can have per-policy DNSREDIR
// rules even when no NDMS access policy is bound to its segment (segment
// binding only adds the per-bridge catch-all MARK in PREROUTING_MANGL).
// Reading from DNSREDIR finds bridges in both cases — including the
// common "main home segment with no segment-level policy" config that
// breaks DNS for default devices when sing-box's hijack-dns listener
// is active (issue #132).
//
// When a bridge has multiple eligible marks, we prefer any mark other
// than singboxPolicyMark — re-marking default DNS up to the sing-box
// policy mark would route it via the sing-box policy's table, which
// typically has no WAN permit (the policy exists only to feed TPROXY),
// so DNS would never resolve upstream. If singboxPolicyMark is the
// only choice, we fall back to it (still better than no rule at all).
//
// Returns empty slice (not nil, not error) when no bridges qualify;
// callers should skip the DNS-NOPOLICY install logic in that case.
func DiscoverLANBridges(ctx context.Context, singboxPolicyMark string) ([]LANBridgeMark, error) {
	result, err := sysexec.Run(ctx, sysiptables.Binary, "-w", "-t", "nat",
		"-S", "_NDM_HOTSPOT_DNSREDIR")
	if err != nil || result == nil {
		// Chain doesn't exist: router has no hotspot config (fresh
		// install, no LAN policies created yet). Nothing to elevate
		// to — return empty, caller skips.
		return []LANBridgeMark{}, nil
	}

	candidates := map[string]map[string]bool{}
	for _, line := range splitLines(result.Stdout) {
		iface, mark, ok := parseDNSRedirRule(line)
		if !ok {
			continue
		}
		if !isLinuxBridge(iface) {
			continue
		}
		if candidates[iface] == nil {
			candidates[iface] = map[string]bool{}
		}
		candidates[iface][mark] = true
	}

	bridges := make([]string, 0, len(candidates))
	for b := range candidates {
		bridges = append(bridges, b)
	}
	sort.Strings(bridges)

	out := make([]LANBridgeMark, 0, len(bridges))
	for _, b := range bridges {
		out = append(out, LANBridgeMark{Bridge: b, Mark: pickMark(candidates[b], singboxPolicyMark)})
	}
	return out, nil
}

// pickMark chooses one mark from the available set for a bridge. It
// returns any mark other than singboxPolicyMark when one exists, falling
// back to singboxPolicyMark only when it's the sole option. Marks are
// sorted before selection so the choice is deterministic across runs
// — that keeps reconcileInstalled stable when nothing actually changed.
func pickMark(marks map[string]bool, singboxPolicyMark string) string {
	ordered := make([]string, 0, len(marks))
	for m := range marks {
		ordered = append(ordered, m)
	}
	sort.Strings(ordered)

	if singboxPolicyMark != "" {
		for _, m := range ordered {
			if !strings.EqualFold(m, singboxPolicyMark) {
				return m
			}
		}
	}
	return ordered[0]
}

// parseDNSRedirRule extracts (interface, mark) from one
// _NDM_HOTSPOT_DNSREDIR rule line. Returns ok=false unless the rule
// targets DNS port 53 with a REDIRECT — sibling rules for ports 1900
// (SSDP) and 5351 (NAT-PMP) are filtered out.
//
// Example accepted line:
//
//	-A _NDM_HOTSPOT_DNSREDIR -d 192.168.0.1/32 -i br0 -p udp -m mark --mark 0xffffaae -m pkttype --pkt-type unicast -m udp --dport 53 -j REDIRECT --to-ports 41104
func parseDNSRedirRule(line string) (iface, mark string, ok bool) {
	if !strings.HasPrefix(line, "-A _NDM_HOTSPOT_DNSREDIR ") {
		return "", "", false
	}
	tokens := strings.Fields(line)
	var (
		hasDNS      bool
		hasRedirect bool
	)
	for i := 0; i < len(tokens)-1; i++ {
		switch tokens[i] {
		case "-i":
			iface = tokens[i+1]
		case "--mark":
			mark = tokens[i+1]
		case "--dport":
			if tokens[i+1] == "53" {
				hasDNS = true
			}
		case "-j":
			if tokens[i+1] == "REDIRECT" {
				hasRedirect = true
			}
		}
	}
	if !hasDNS || !hasRedirect || iface == "" || mark == "" {
		return "", "", false
	}
	return iface, mark, true
}

// isLinuxBridge reports whether the named interface is a real Linux
// bridge (has a /sys/class/net/<name>/bridge directory). WireGuard
// tunnels, physical NICs, PPP, and SSTP "bridges" that NDMS marks but
// that aren't true L2 bridges return false.
func isLinuxBridge(iface string) bool {
	info, err := os.Stat(fmt.Sprintf("/sys/class/net/%s/bridge", iface))
	return err == nil && info.IsDir()
}

// equalLANBridges reports whether two []LANBridgeMark slices have the
// same (bridge, mark) pairs in the same order. Used by reconcileInstalled
// to decide whether an iptables re-install is needed when LAN-bridge
// state on the router drifts (NDMS hotspot reconfigured, bridge added/
// removed, mark reassigned to a different policy).
func equalLANBridges(a, b []LANBridgeMark) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Bridge != b[i].Bridge || a[i].Mark != b[i].Mark {
			return false
		}
	}
	return true
}

func splitLines(s string) []string {
	out := make([]string, 0, 16)
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			if i > start {
				out = append(out, s[start:i])
			}
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
