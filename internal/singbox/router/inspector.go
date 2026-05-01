package router

import (
	"fmt"
	"net"
	"strings"
)

// InspectInput is the user-supplied query for the route inspector.
// Domain is either a domain or an IP literal — Inspect classifies which.
// Port == 0 means "no port match" (Port matchers are skipped, recorded
// as not-matched without claiming a hit). Protocol is "tcp"/"udp"; empty
// means "skip protocol matchers".
type InspectInput struct {
	Domain   string
	Port     int
	Protocol string
}

// RuleMatchResult captures the outcome of evaluating one rule against
// the input. Conditions describes what we actually evaluated for the
// human reader; Reason explains the decision.
type RuleMatchResult struct {
	Index      int      `json:"index"`
	Matched    bool     `json:"matched"`
	Action     string   `json:"action"`
	Outbound   string   `json:"outbound,omitempty"`
	Conditions []string `json:"conditions,omitempty"`
	Reason     string   `json:"reason,omitempty"`
}

// InspectResult is the public response of the inspector.
//   - Matches[i].Index == i, in route-rule order.
//   - Destination is the resolved final outbound (or "REJECT", or the
//     route.final value when no rule matches).
//   - MatchedRule is the index of the first *terminal* match (route or
//     reject); -1 when no rule produced a final destination.
//   - Note carries free-form caveats (e.g. unsupported features).
type InspectResult struct {
	Input       string            `json:"input"`
	InputType   string            `json:"inputType"`
	Matches     []RuleMatchResult `json:"matches"`
	Destination string            `json:"destination"`
	MatchedRule int               `json:"matchedRule"`
	Final       string            `json:"final"`
	Note        string            `json:"note,omitempty"`
}

// Inspect walks rules in priority order, evaluates each rule's matchers
// against the input, and returns a result describing both the per-rule
// decisions and the final destination outbound.
//
// Matcher semantics (AND across present matchers, mirroring sing-box):
//   - DomainSuffix: input must be a domain; matches if any suffix is a
//     tail of the input (case-insensitive).
//   - IPCIDR: input must be an IP; matches if any CIDR contains it.
//     Bare IPs (without /mask) are treated as /32 or /128 equivalents.
//   - Port: matches if input.Port is in the list. When input.Port==0 we
//     skip the matcher and record it as not evaluated — that is a
//     "no input given" signal, not a match.
//   - Protocol: matches if equal to input.Protocol (case-insensitive).
//     Empty input.Protocol skips the matcher.
//   - RuleSet: NOT supported in v1 — flagged on the InspectResult.Note
//     and the per-rule Reason; the rule is treated as no-match so the
//     walk continues to subsequent rules.
//   - SourceIPCIDR: skipped (irrelevant for this inspector — there is no
//     "source IP" in a manual probe).
//
// First terminal match (action == "route" with non-empty Outbound, or
// action == "reject") wins. Non-terminal actions ("sniff", "hijack-dns")
// continue the walk. If nothing matches, Destination falls back to
// route.final (or "direct" when final is empty).
func Inspect(input InspectInput, rules []Rule, ruleSets []RuleSet, final string) InspectResult {
	res := InspectResult{
		Input:       input.Domain,
		Matches:     []RuleMatchResult{},
		MatchedRule: -1,
		Final:       final,
	}

	// Classify input — IP literal vs domain.
	parsedIP := net.ParseIP(input.Domain)
	if parsedIP != nil {
		res.InputType = "ip"
	} else {
		res.InputType = "domain"
	}

	hasRuleSetUsage := false

	for i, rule := range rules {
		match := evaluateRule(input, parsedIP, rule, &hasRuleSetUsage)
		match.Index = i
		res.Matches = append(res.Matches, match)

		if !match.Matched {
			continue
		}

		// Terminal vs non-terminal action handling.
		switch rule.Action {
		case "route":
			if res.MatchedRule == -1 {
				res.MatchedRule = i
				if rule.Outbound != "" {
					res.Destination = rule.Outbound
				} else {
					res.Destination = "DIRECT"
				}
			}
		case "reject":
			if res.MatchedRule == -1 {
				res.MatchedRule = i
				res.Destination = "REJECT"
			}
		case "sniff", "hijack-dns":
			// Non-terminal: matched but does not set Destination; walk
			// continues so a later rule (or final) can claim it.
		default:
			// Unknown action — be conservative, treat as terminal route
			// on the rule's outbound to surface it in the UI.
			if res.MatchedRule == -1 {
				res.MatchedRule = i
				if rule.Outbound != "" {
					res.Destination = rule.Outbound
				} else {
					res.Destination = "DIRECT"
				}
			}
		}
	}

	if res.Destination == "" {
		if final != "" {
			res.Destination = final
		} else {
			res.Destination = "direct"
			res.Final = "direct"
		}
	}

	if hasRuleSetUsage {
		res.Note = "Сопоставление rule_set не поддерживается в этой версии — такие правила пропускаются как не совпавшие."
	}

	// Suppress unused-parameter warning while keeping the signature
	// stable for a future rule_set-aware implementation.
	_ = ruleSets

	return res
}

// evaluateRule returns the per-rule decision. Empty rule (no matchers)
// is defensively treated as no-match — it would otherwise sweep every
// query into a "match" bucket and confuse the UI.
func evaluateRule(input InspectInput, parsedIP net.IP, rule Rule, ruleSetUsedPtr *bool) RuleMatchResult {
	out := RuleMatchResult{
		Action:   rule.Action,
		Outbound: rule.Outbound,
	}

	// rule_set short-circuit: v1 cannot evaluate it. Flag and bail with
	// no-match so the inspector remains useful for the surrounding
	// rules without claiming false positives.
	if len(rule.RuleSet) > 0 {
		*ruleSetUsedPtr = true
		out.Conditions = append(out.Conditions, fmt.Sprintf("rule_set: %s", strings.Join(rule.RuleSet, ", ")))
		out.Reason = "rule_set inspection не реализована — считаем как несовпадение"
		return out
	}

	// SourceIPCIDR is a context we don't have for a manual probe.
	// Record it as N/A but neither match nor block.
	if len(rule.SourceIPCIDR) > 0 {
		out.Conditions = append(out.Conditions, fmt.Sprintf("source_ip_cidr: %s (пропущено — нет источника)", strings.Join(rule.SourceIPCIDR, ", ")))
	}

	// Track each matcher's outcome. AND across present matchers.
	type partial struct{ present, hit bool }
	var (
		domainPart   partial
		ipPart       partial
		portPart     partial
		protocolPart partial
	)

	// DomainSuffix
	if len(rule.DomainSuffix) > 0 {
		domainPart.present = true
		out.Conditions = append(out.Conditions, fmt.Sprintf("domain_suffix: [%s]", strings.Join(rule.DomainSuffix, ", ")))
		if parsedIP == nil {
			lower := strings.ToLower(input.Domain)
			for _, suffix := range rule.DomainSuffix {
				if matchesDomainSuffix(lower, suffix) {
					domainPart.hit = true
					break
				}
			}
		}
	}

	// IPCIDR
	if len(rule.IPCIDR) > 0 {
		ipPart.present = true
		out.Conditions = append(out.Conditions, fmt.Sprintf("ip_cidr: [%s]", strings.Join(rule.IPCIDR, ", ")))
		if parsedIP != nil {
			for _, c := range rule.IPCIDR {
				if cidrContains(c, parsedIP) {
					ipPart.hit = true
					break
				}
			}
		}
	}

	// Port — if no input port given, mark present-but-not-evaluated
	// so AND logic does not declare a match without verification.
	if len(rule.Port) > 0 {
		portPart.present = true
		ports := make([]string, 0, len(rule.Port))
		for _, p := range rule.Port {
			ports = append(ports, fmt.Sprintf("%d", p))
		}
		if input.Port == 0 {
			out.Conditions = append(out.Conditions, fmt.Sprintf("port: [%s] (пропущено — порт не задан)", strings.Join(ports, ", ")))
		} else {
			out.Conditions = append(out.Conditions, fmt.Sprintf("port: [%s]", strings.Join(ports, ", ")))
			for _, p := range rule.Port {
				if p == input.Port {
					portPart.hit = true
					break
				}
			}
		}
	}

	// Protocol
	if rule.Protocol != "" {
		protocolPart.present = true
		out.Conditions = append(out.Conditions, fmt.Sprintf("protocol: %s", rule.Protocol))
		if input.Protocol != "" && strings.EqualFold(rule.Protocol, input.Protocol) {
			protocolPart.hit = true
		}
	}

	// Determine match: at least one matcher present, AND every present
	// matcher must hit (or, for Port without input, be permissively
	// skipped — we explicitly do NOT count an unverifiable matcher as
	// a hit, so an unverified port keeps the rule as no-match).
	anyPresent := domainPart.present || ipPart.present || portPart.present || protocolPart.present
	if !anyPresent {
		out.Reason = "пустое правило — пропущено"
		return out
	}

	matched := true
	if domainPart.present && !domainPart.hit {
		matched = false
	}
	if ipPart.present && !ipPart.hit {
		matched = false
	}
	if portPart.present && !portPart.hit {
		matched = false
	}
	if protocolPart.present && !protocolPart.hit {
		matched = false
	}

	out.Matched = matched
	if matched {
		var hits []string
		if domainPart.hit {
			hits = append(hits, "domain_suffix")
		}
		if ipPart.hit {
			hits = append(hits, "ip_cidr")
		}
		if portPart.hit {
			hits = append(hits, "port")
		}
		if protocolPart.hit {
			hits = append(hits, "protocol")
		}
		out.Reason = "совпало по: " + strings.Join(hits, ", ")
	} else {
		out.Reason = "нет совпадения"
	}
	return out
}

// matchesDomainSuffix returns true when domain ends with suffix (or
// equals it). Both inputs are expected to be lowercase already.
//
// sing-box's domain_suffix semantic: a leading dot is implicit — both
// "google.com" and ".google.com" match "www.google.com". An exact
// equality also matches.
func matchesDomainSuffix(domain, suffix string) bool {
	suffix = strings.ToLower(strings.TrimPrefix(suffix, "."))
	if domain == suffix {
		return true
	}
	return strings.HasSuffix(domain, "."+suffix)
}

// cidrContains parses cidr (CIDR notation OR a bare IP literal) and
// checks whether it contains ip. Bare IP literals are treated as a
// single-host network so "ip_cidr: ['8.8.8.8']" still works.
func cidrContains(cidr string, ip net.IP) bool {
	if !strings.Contains(cidr, "/") {
		single := net.ParseIP(cidr)
		return single != nil && single.Equal(ip)
	}
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	return network.Contains(ip)
}
