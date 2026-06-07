package subscription

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/hoaxisr/awg-manager/internal/singbox/vlink"
)

// MaxSubscriptionInfoItems caps provider info banners (days left, traffic, bot links).
const MaxSubscriptionInfoItems = 4

// SubscriptionInfoItem is a non-proxy line from the subscription body shown at the top of the UI.
type SubscriptionInfoItem struct {
	ID     string `json:"id"`               // stable key (tag or hash of label)
	Label  string `json:"label"`            // usually URI fragment
	Tag    string `json:"tag,omitempty"`    // sub-<id8>-<hash> when parsed
	Source string `json:"source,omitempty"` // "auto" (reserved; not shown in UI)
}

// RejectedMember is a share-link that was parsed but not materialized in sing-box.
type RejectedMember struct {
	Tag      string `json:"tag,omitempty"`
	Label    string `json:"label,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Server   string `json:"server,omitempty"`
	Port     uint16 `json:"port,omitempty"`
	Reason   string `json:"reason"`
}

// partitionResult splits parsed outbounds into materializable members, info banners, and rejects.
type partitionResult struct {
	Valid    []vlink.ParsedOutbound
	Info     []SubscriptionInfoItem
	Rejected []RejectedMember
}

func partitionParsedOutbounds(subID string, parsed []vlink.ParsedOutbound) partitionResult {
	out := partitionResult{
		Valid:    make([]vlink.ParsedOutbound, 0, len(parsed)),
		Info:     []SubscriptionInfoItem{},
		Rejected: []RejectedMember{},
	}
	infoSeen := map[string]bool{}
	rejectedSeen := map[string]bool{}
	for _, p := range parsed {
		ob, err := outboundMap(p.Outbound)
		if err != nil {
			r := RejectedMember{
				Label:    p.Label,
				Protocol: p.Protocol,
				Reason:   "invalid outbound json",
			}
			out.Rejected = appendRejectedIfNew(out.Rejected, rejectedSeen, r)
			continue
		}
		tag := StableTag(subID, p)
		reason := classifyOutbound(ob)
		if reason == "" {
			out.Valid = append(out.Valid, p)
			continue
		}
		// Info heuristics apply only to structurally invalid links (banners, localhost service lines).
		if looksLikeSubscriptionInfo(p, ob) {
			item := SubscriptionInfoItem{
				ID:     infoItemID(tag, p.Label),
				Label:  infoLabel(p),
				Tag:    tag,
				Source: "auto",
			}
			if infoSeen[item.ID] {
				continue
			}
			if len(out.Info) >= MaxSubscriptionInfoItems {
				out.Rejected = appendRejectedIfNew(out.Rejected, rejectedSeen, rejectedFromParsed(tag, p, ob, "info slot full (max 4)"))
				continue
			}
			infoSeen[item.ID] = true
			out.Info = append(out.Info, item)
			continue
		}
		out.Rejected = appendRejectedIfNew(out.Rejected, rejectedSeen, rejectedFromParsed(tag, p, ob, reason))
	}
	return out
}

func mergeInfoItems(pinned []SubscriptionInfoItem, auto []SubscriptionInfoItem) []SubscriptionInfoItem {
	seen := map[string]bool{}
	out := make([]SubscriptionInfoItem, 0, MaxSubscriptionInfoItems)
	for _, it := range pinned {
		id := strings.TrimSpace(it.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		keep := it
		if keep.Source == "" {
			keep.Source = "auto"
		}
		out = append(out, keep)
		if len(out) >= MaxSubscriptionInfoItems {
			return out
		}
	}
	for _, it := range auto {
		id := strings.TrimSpace(it.ID)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		it.Source = "auto"
		out = append(out, it)
		if len(out) >= MaxSubscriptionInfoItems {
			return out
		}
	}
	return out
}

func rejectedFromPrunedTags(sub *Subscription, tags []string) []RejectedMember {
	if len(tags) == 0 {
		return nil
	}
	byTag := map[string]MemberInfo{}
	for _, m := range sub.Members {
		byTag[m.Tag] = m
	}
	out := make([]RejectedMember, 0, len(tags))
	for _, tag := range tags {
		if m, ok := byTag[tag]; ok {
			out = append(out, RejectedMember{
				Tag:      m.Tag,
				Label:    m.Label,
				Protocol: m.Protocol,
				Server:   m.Server,
				Port:     m.Port,
				Reason:   "not materialized in sing-box config",
			})
			continue
		}
		out = append(out, RejectedMember{Tag: tag, Reason: "not materialized in sing-box config"})
	}
	return out
}

func appendRejectedIfNew(dst []RejectedMember, seen map[string]bool, r RejectedMember) []RejectedMember {
	key := rejectedKey(r)
	if key != "" && seen[key] {
		return dst
	}
	if key != "" {
		seen[key] = true
	}
	return append(dst, r)
}

func appendRejectedUnique(dst []RejectedMember, add ...RejectedMember) []RejectedMember {
	seen := map[string]bool{}
	for _, r := range dst {
		key := rejectedKey(r)
		if key != "" {
			seen[key] = true
		}
	}
	for _, r := range add {
		dst = appendRejectedIfNew(dst, seen, r)
	}
	return dst
}

func rejectedKey(r RejectedMember) string {
	if r.Tag != "" {
		return "tag:" + r.Tag
	}
	if r.Label != "" {
		return "label:" + r.Label
	}
	return "reason:" + r.Reason
}

const reasonRemovedFromInfo = "убрано из информации провайдера"

func rejectedFromInfoItem(it SubscriptionInfoItem) RejectedMember {
	return RejectedMember{
		Tag:    it.Tag,
		Label:  it.Label,
		Reason: reasonRemovedFromInfo,
	}
}

func rejectedFromParsed(tag string, p vlink.ParsedOutbound, ob map[string]any, reason string) RejectedMember {
	return RejectedMember{
		Tag:      tag,
		Label:    infoLabel(p),
		Protocol: p.Protocol,
		Server:   stringOf(ob["server"]),
		Port:     p.Port,
		Reason:   reason,
	}
}

func infoLabel(p vlink.ParsedOutbound) string {
	if strings.TrimSpace(p.Label) != "" {
		return strings.TrimSpace(p.Label)
	}
	if p.Server != "" {
		return p.Server
	}
	return p.Protocol
}

func infoItemID(tag, label string) string {
	if tag != "" {
		return tag
	}
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(label))))
	return "info-" + hex.EncodeToString(sum[:4])
}

func outboundMap(raw json.RawMessage) (map[string]any, error) {
	var ob map[string]any
	if err := json.Unmarshal(raw, &ob); err != nil {
		return nil, err
	}
	return ob, nil
}

// looksLikeSubscriptionInfo detects provider banner links among outbounds that
// already failed classifyOutbound (days left, traffic, bot, localhost, etc.).
func looksLikeSubscriptionInfo(p vlink.ParsedOutbound, ob map[string]any) bool {
	server := strings.ToLower(strings.TrimSpace(stringOf(ob["server"])))
	if server == "localhost" || server == "127.0.0.1" || server == "::1" {
		return true
	}
	label := strings.ToLower(infoLabel(p))
	if label == "" {
		return false
	}
	keywords := []string{
		"осталось", "остаток", "дней", "день", "days left",
		"traffic", "трафик", "t.me/", "telegram",
		"бот", "bot", "подписк", "expired", "истек",
	}
	for _, kw := range keywords {
		if strings.Contains(label, kw) {
			return true
		}
	}
	if labelHasTrafficLTE(label) || labelHasDataQuotaGB(label) {
		return true
	}
	// Calendar / countdown banners without a real host (not 🟢 — used in node names like 🇱🇻🟢LTE).
	if p.Server == "" || server == "" {
		if strings.ContainsRune(label, '📆') || strings.ContainsRune(label, '⏳') {
			return true
		}
	}
	// Fake userinfo on vless combined with non-routable host.
	if p.Protocol == "vless" {
		uuid := stringOf(ob["uuid"])
		if uuid != "" && !isValidUUID(uuid) && (server == "localhost" || server == "127.0.0.1") {
			return true
		}
	}
	return false
}

// labelHasTrafficLTE matches LTE as a traffic/tech banner, not as a suffix on country node names (…🟢LTE).
func labelHasTrafficLTE(label string) bool {
	for _, p := range []string{
		" lte ", " lte/", " lte:", " lte-", " lte|",
		"lte ", "lte:", "lte-", "lte/",
		"lte трафик", "lte traffic", "трафик lte", "traffic lte",
		"lte-трафик", "lte/internet",
	} {
		if strings.Contains(label, p) {
			return true
		}
	}
	return strings.HasPrefix(label, "lte ") || strings.HasPrefix(label, "lte:") || strings.HasPrefix(label, "lte-")
}

// labelHasDataQuotaGB matches quota banners (100 GB), not arbitrary "gb" inside words.
func labelHasDataQuotaGB(label string) bool {
	for _, p := range []string{
		" gb", "gb ", "gb/", "gb:", "gb-", "gb|",
		"гб ", " гб", "гб/", "гб:", "гб-",
		"100gb", "50gb", "500gb",
	} {
		if strings.Contains(label, p) {
			return true
		}
	}
	return strings.HasPrefix(label, "gb ") || strings.HasPrefix(label, "гб ")
}

// filterDismissedInfo drops auto info lines the user removed from the UI.
func filterDismissedInfo(items []SubscriptionInfoItem, dismissed []string) []SubscriptionInfoItem {
	if len(items) == 0 || len(dismissed) == 0 {
		return items
	}
	seen := map[string]bool{}
	for _, id := range dismissed {
		id = strings.TrimSpace(id)
		if id != "" {
			seen[id] = true
		}
	}
	out := make([]SubscriptionInfoItem, 0, len(items))
	for _, it := range items {
		if seen[strings.TrimSpace(it.ID)] {
			continue
		}
		out = append(out, it)
	}
	return out
}

// findInfoItem returns index of info entry by id, or -1.
func findInfoItem(items []SubscriptionInfoItem, itemID string) int {
	id := strings.TrimSpace(itemID)
	for i, it := range items {
		if strings.TrimSpace(it.ID) == id {
			return i
		}
	}
	return -1
}

// findRejected returns index of rejected entry by tag or id.
func findRejected(rejected []RejectedMember, tag string) int {
	for i, r := range rejected {
		if r.Tag == tag {
			return i
		}
	}
	return -1
}
