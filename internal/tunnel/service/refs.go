// internal/tunnel/service/refs.go
package service

import (
	"fmt"
	"strings"
)

// ErrTunnelReferenced is returned by Delete when the tunnel is
// referenced by deviceproxy or any sing-box-router rule. Callers
// (API handlers) typecheck this and translate to HTTP 409 with
// structured details so the UI can deeplink the user to the
// referencing config.
type ErrTunnelReferenced struct {
	TunnelID    string
	DeviceProxy bool
	RouterRules []int
	RouterOther []string
}

func (e ErrTunnelReferenced) Error() string {
	parts := []string{}
	if e.DeviceProxy {
		parts = append(parts, "device-proxy selector")
	}
	if len(e.RouterRules) > 0 {
		parts = append(parts, fmt.Sprintf("%d router rule(s)", len(e.RouterRules)))
	}
	if len(e.RouterOther) > 0 {
		parts = append(parts, fmt.Sprintf("%d router outbound reference(s)", len(e.RouterOther)))
	}
	return "tunnel " + e.TunnelID + " is referenced by: " + strings.Join(parts, ", ")
}

// DeviceProxyRefChecker reports whether tag is currently in the
// deviceproxy selector members or set as the persisted SelectedOutbound.
type DeviceProxyRefChecker interface {
	HasSelectorReference(tag string) bool
}

// RouterRefChecker returns the indices of router rules whose outbound
// equals tag, and other locations referencing the tag. Empty slice = no references.
type RouterRefChecker interface {
	RulesReferencing(tag string) []int
	OutboundReferenceLocations(tag string) []string
}

// CheckOutboundTagReferences returns ErrTunnelReferenced when tag is still
// referenced by deviceproxy or sing-box router config. displayID is echoed
// back in the error for UI (may differ from tag, e.g. AWG tunnel id vs awg-{id}).
func CheckOutboundTagReferences(tag, displayID string, dp DeviceProxyRefChecker, r RouterRefChecker) error {
	refs := ErrTunnelReferenced{TunnelID: displayID}
	if mergeTagReferences(tag, &refs, dp, r) {
		return refs
	}
	return nil
}

// CheckOutboundTagsReferenced aggregates references across multiple outbound
// tags (e.g. subscription selector + members) into a single refusal error.
func CheckOutboundTagsReferenced(displayID string, tags []string, dp DeviceProxyRefChecker, r RouterRefChecker) error {
	refs := ErrTunnelReferenced{TunnelID: displayID}
	refused := false
	for _, tag := range tags {
		if tag == "" {
			continue
		}
		if mergeTagReferences(tag, &refs, dp, r) {
			refused = true
		}
	}
	if refused {
		return refs
	}
	return nil
}

func mergeTagReferences(tag string, refs *ErrTunnelReferenced, dp DeviceProxyRefChecker, r RouterRefChecker) bool {
	refused := false
	if dp != nil && dp.HasSelectorReference(tag) {
		refs.DeviceProxy = true
		refused = true
	}
	if r != nil {
		if rules := r.RulesReferencing(tag); len(rules) > 0 {
			refs.RouterRules = mergeIntSlices(refs.RouterRules, rules)
			refused = true
		}
		if locs := r.OutboundReferenceLocations(tag); len(locs) > 0 {
			refs.RouterOther = mergeStringSlices(refs.RouterOther, locs)
			refused = true
		}
	}
	return refused
}

func mergeIntSlices(a, b []int) []int {
	seen := make(map[int]struct{}, len(a)+len(b))
	out := make([]int, 0, len(a)+len(b))
	for _, v := range append(a, b...) {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func mergeStringSlices(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, v := range append(a, b...) {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// checkTunnelReferences returns ErrTunnelReferenced if any checker
// reports references to the tunnel's awg-{id} tag, nil otherwise.
func checkTunnelReferences(tunnelID string, dp DeviceProxyRefChecker, r RouterRefChecker) error {
	return CheckOutboundTagReferences("awg-"+tunnelID, tunnelID, dp, r)
}
