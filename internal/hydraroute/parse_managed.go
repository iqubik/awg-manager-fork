package hydraroute

import (
	"strings"
)

// isImpossibleUseBlock is HR Neo's oversized-geoip service section header.
func isImpossibleUseBlock(listName string) bool {
	return strings.EqualFold(strings.TrimSpace(listName), "impossible to use")
}

// parseDomainConf reads a domain.conf body and returns each `## Name` block
// followed by a `domains/iface` line as a ManagedEntry. Lines that don't
// match the format are silently skipped. A leading '#' on the data line
// marks the rule disabled (HR Neo ignores commented content).
func parseDomainConf(content string) []ManagedEntry {
	var entries []ManagedEntry
	var pendingName string

	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimRight(raw, "\r")
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "##") {
			pendingName = strings.TrimSpace(strings.TrimPrefix(line, "##"))
			continue
		}

		if pendingName == "" {
			continue
		}

		disabled := false
		if strings.HasPrefix(line, "#") {
			line = strings.TrimPrefix(line, "#")
			disabled = true
		}

		slash := strings.LastIndex(line, "/")
		if slash < 0 {
			pendingName = ""
			continue
		}
		domains := splitNonEmpty(line[:slash], ",")
		entries = append(entries, ManagedEntry{
			ListName: pendingName,
			Domains:  domains,
			Iface:    line[slash+1:],
			Disabled: disabled,
		})
		pendingName = ""
	}

	return entries
}

// parseIPList reads an ip.list body and returns:
//   - regular rule entries: blocks with a `/Target` line
//   - oversized tag names: entries inside a service block whose target line
//     starts with `#/` (HR Neo's 'disabled interface' marker, e.g.
//     `#/Too-big-geoip-tag`). Only `geoip:TAG` lines in such blocks are
//     collected; other lines are discarded.
//
// For normal rules, `#/Target` or `#` on subnet lines marks the rule disabled.
//
// Empty lines or a new `##` header terminate the current block.
func parseIPList(content string) (entries []ManagedEntry, oversized []string) {
	var cur ManagedEntry
	active := false
	service := false

	flush := func() {
		if active && !service && cur.ListName != "" && len(cur.Subnets) > 0 {
			entries = append(entries, cur)
		}
		cur = ManagedEntry{}
		active = false
		service = false
	}

	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimRight(raw, "\r")

		if strings.HasPrefix(line, "##") {
			flush()
			cur = ManagedEntry{ListName: strings.TrimSpace(strings.TrimPrefix(line, "##"))}
			active = true
			continue
		}

		if line == "" {
			flush()
			continue
		}

		if strings.HasPrefix(line, "#/") {
			if active && isImpossibleUseBlock(cur.ListName) {
				service = true
			} else if active {
				cur.Iface = strings.TrimPrefix(line, "#/")
				cur.Disabled = true
			}
			continue
		}

		disabledLine := false
		if strings.HasPrefix(line, "#") {
			line = strings.TrimPrefix(line, "#")
			disabledLine = true
		}

		if !active {
			continue
		}

		if service {
			if strings.HasPrefix(line, "geoip:") {
				oversized = append(oversized, line)
			}
			continue
		}

		if strings.HasPrefix(line, "/") {
			cur.Iface = strings.TrimPrefix(line, "/")
			if disabledLine {
				cur.Disabled = true
			}
			continue
		}

		cur.Subnets = append(cur.Subnets, line)
		if disabledLine {
			cur.Disabled = true
		}
	}
	flush()

	return entries, oversized
}

// splitNonEmpty splits s by sep, trims entries, drops empties.
func splitNonEmpty(s, sep string) []string {
	parts := strings.Split(s, sep)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
