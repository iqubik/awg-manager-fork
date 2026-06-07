package dnsroute

// applyExcludesText derives active Excludes from raw ExcludesText when ExcludesText
// is present in the request. Full-line comments are ignored and never reach
// Excludes/ExcludeSubnets/NDMS.
func applyExcludesText(list *DomainList) {
	if list == nil || list.ExcludesText == nil {
		return
	}

	// Reuse the same active-line parser as manual domains:
	// - skip blanks
	// - skip full-line # comments
	// - skip invalid lines/CIDR
	// - strip leading dots for non-geo entries
	list.Excludes = activeManualDomainsFromText(*list.ExcludesText)

	// ExcludeSubnets are derived later by splitDomainsAndSubnets(list.Excludes).
	// Clear stale values so ExcludesText becomes the single source of truth.
	list.ExcludeSubnets = nil
}
