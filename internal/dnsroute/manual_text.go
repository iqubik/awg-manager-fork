package dnsroute

import (
	"net"
	"strings"
)

// applyManualText derives active ManualDomains from raw ManualText when ManualText
// is present in the request. Full-line comments are ignored and never reach
// Domains/Subnets/NDMS.
func applyManualText(list *DomainList) {
	if list == nil || list.ManualText == nil {
		return
	}
	list.ManualDomains = activeManualDomainsFromText(*list.ManualText)
}

func activeManualDomainsFromText(text string) []string {
	lines := strings.Split(text, "\n")
	active := make([]string, 0, len(lines))

	for _, raw := range lines {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		if !isValidManualDomainLine(line) {
			continue
		}

		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "geosite:") || strings.HasPrefix(lower, "geoip:") {
			active = append(active, line)
			continue
		}

		normalized := strings.TrimLeft(line, ".")
		if normalized == "" {
			continue
		}
		active = append(active, normalized)
	}

	return active
}

func isBareLabel(line string) bool {
	if line == "" {
		return false
	}
	for i, r := range line {
		ok := (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '-'
		if !ok {
			return false
		}
		if (i == 0 || i == len(line)-1) && r == '-' {
			return false
		}
	}
	return true
}

func isValidManualDomainLine(line string) bool {
	if line == "" {
		return false
	}
	if strings.ContainsAny(line, " \t") || strings.Contains(line, "*") {
		return false
	}

	lower := strings.ToLower(line)
	if strings.HasPrefix(lower, "geosite:") || strings.HasPrefix(lower, "geoip:") {
		return true
	}

	if strings.Contains(line, "/") {
		_, _, err := net.ParseCIDR(line)
		return err == nil
	}

	if isBareLabel(line) {
		return true
	}

	return strings.Contains(line, ".")
}
