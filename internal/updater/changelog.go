package updater

import (
	"regexp"
	"sort"
	"strings"

	"github.com/hoaxisr/awg-manager/internal/sys/semver"
)

// Entry is one version's changelog as parsed from CHANGELOG.md.
type Entry struct {
	Version string  `json:"version"`
	Date    string  `json:"date"`
	Groups  []Group `json:"groups"`
}

// Group is a Keep-a-Changelog section (Added/Fixed/...) within a version.
type Group struct {
	Heading string   `json:"heading"`
	Items   []string `json:"items"`
}

// versionLine matches "## [2.8.0] - 2026-04-17" with optional whitespace.
var versionLine = regexp.MustCompile(`^##\s+\[([^\]]+)\]\s+-\s+(\S+)\s*$`)

// groupLine matches "### Added" (any Keep-a-Changelog heading).
var groupLine = regexp.MustCompile(`^###\s+(.+?)\s*$`)

// itemLine matches "- something" or "* something".
var itemLine = regexp.MustCompile(`^[-*]\s+(.+?)\s*$`)

// proseLine matches markdown headings that are not version/group lines.
var proseLine = regexp.MustCompile(`^#{1,6}\s`)

// hrLine matches horizontal rules (---, ***, ___).
var hrLine = regexp.MustCompile(`^(-{3,}|\*{3,}|_{3,})$`)

// ParseChangelog returns version-keyed entries parsed from a CHANGELOG.md body.
// Unparseable leftover lines are skipped silently so a partial file still
// produces usable data.
func ParseChangelog(md string) (map[string]Entry, error) {
	out := make(map[string]Entry)
	var cur *Entry
	var curGroup *Group

	flushGroup := func() {
		if cur != nil && curGroup != nil && len(curGroup.Items) > 0 {
			cur.Groups = append(cur.Groups, *curGroup)
		}
		curGroup = nil
	}
	flushEntry := func() {
		flushGroup()
		if cur != nil {
			out[cur.Version] = *cur
			cur = nil
		}
	}

	appendLine := func(line, trimmed string) {
		if proseLine.MatchString(trimmed) || hrLine.MatchString(trimmed) {
			return
		}
		// Prose before the first ### becomes a group with an empty heading.
		if curGroup == nil {
			curGroup = &Group{}
		}
		if m := itemLine.FindStringSubmatch(trimmed); m != nil {
			curGroup.Items = append(curGroup.Items, m[1])
			return
		}
		if len(curGroup.Items) > 0 && strings.HasPrefix(line, " ") {
			last := len(curGroup.Items) - 1
			curGroup.Items[last] = curGroup.Items[last] + "\n" + trimmed
			return
		}
		curGroup.Items = append(curGroup.Items, trimmed)
	}

	for _, raw := range strings.Split(md, "\n") {
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if m := versionLine.FindStringSubmatch(trimmed); m != nil {
			flushEntry()
			cur = &Entry{Version: m[1], Date: m[2]}
			continue
		}
		if cur == nil {
			continue
		}
		if m := groupLine.FindStringSubmatch(trimmed); m != nil {
			flushGroup()
			curGroup = &Group{Heading: m[1]}
			continue
		}
		appendLine(line, trimmed)
	}
	flushEntry()
	return out, nil
}

// Single returns the entry for an exact version, or nil if no entry
// matches.
func Single(entries map[string]Entry, version string) *Entry {
	key := semver.Base(version)
	if e, ok := entries[key]; ok {
		return &e
	}
	if key != version {
		if e, ok := entries[version]; ok {
			return &e
		}
	}
	return nil
}

// MinorLine returns changelog entries sharing major.minor with version,
// with entry version <= version, sorted newest-first. Used when no
// upgrade is pending so "what's new" covers the whole 2.11.x line.
// Revision-aware: develop entries like "2.11.2+r95" are compared using
// CompareWithRevision so same-base revisions are ordered correctly.
func MinorLine(entries map[string]Entry, version string) []Entry {
	base := semver.Base(version)
	parts := strings.Split(base, ".")
	if len(parts) < 2 {
		if e := Single(entries, version); e != nil {
			return []Entry{*e}
		}
		return nil
	}
	prefix := parts[0] + "." + parts[1] + "."
	out := make([]Entry, 0)
	for _, e := range entries {
		if !strings.HasPrefix(e.Version+".", prefix) {
			continue
		}
		if semver.CompareWithRevision(e.Version, version) > 0 {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		return semver.CompareWithRevision(out[i].Version, out[j].Version) > 0
	})
	return out
}

// Slice returns entries where fromVer < v <= toVer, sorted newest-first.
// Revision-aware: develop entries like "2.11.2+r95" are compared using
// CompareWithRevision so same-base revisions (e.g. r93→r95) are handled
// correctly. Stable versions (no +rN) behave identically to semver.Compare.
func Slice(entries map[string]Entry, fromVer, toVer string) []Entry {
	if semver.CompareWithRevision(fromVer, toVer) >= 0 {
		return nil
	}
	out := make([]Entry, 0)
	for _, e := range entries {
		if semver.CompareWithRevision(e.Version, fromVer) > 0 && semver.CompareWithRevision(e.Version, toVer) <= 0 {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return semver.CompareWithRevision(out[i].Version, out[j].Version) > 0
	})
	return out
}

func mergeChangelogEntries(primary, secondary map[string]Entry) map[string]Entry {
	out := make(map[string]Entry, len(primary)+len(secondary))
	for version, entry := range secondary {
		out[version] = cloneEntry(entry)
	}
	for version, primaryEntry := range primary {
		if secondaryEntry, ok := out[version]; ok {
			out[version] = mergeChangelogEntry(primaryEntry, secondaryEntry)
			continue
		}
		out[version] = cloneEntry(primaryEntry)
	}
	return out
}

func mergeChangelogEntry(primary, secondary Entry) Entry {
	merged := Entry{
		Version: primary.Version,
		Date:    primary.Date,
		Groups:  make([]Group, 0, len(primary.Groups)+len(secondary.Groups)),
	}
	if merged.Version == "" {
		merged.Version = secondary.Version
	}
	if merged.Date == "" {
		merged.Date = secondary.Date
	}

	groupIndex := make(map[string]int, len(primary.Groups)+len(secondary.Groups))
	appendGroup := func(group Group) {
		if idx, ok := groupIndex[group.Heading]; ok {
			merged.Groups[idx].Items = appendUniqueItems(merged.Groups[idx].Items, group.Items)
			return
		}
		groupIndex[group.Heading] = len(merged.Groups)
		merged.Groups = append(merged.Groups, Group{
			Heading: group.Heading,
			Items:   append([]string(nil), group.Items...),
		})
	}

	for _, group := range primary.Groups {
		appendGroup(group)
	}
	for _, group := range secondary.Groups {
		appendGroup(group)
	}

	return merged
}

func appendUniqueItems(existing, incoming []string) []string {
	seen := make(map[string]struct{}, len(existing)+len(incoming))
	for _, item := range existing {
		seen[item] = struct{}{}
	}
	for _, item := range incoming {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		existing = append(existing, item)
	}
	return existing
}

func cloneEntry(entry Entry) Entry {
	cloned := Entry{
		Version: entry.Version,
		Date:    entry.Date,
		Groups:  make([]Group, 0, len(entry.Groups)),
	}
	for _, group := range entry.Groups {
		cloned.Groups = append(cloned.Groups, Group{
			Heading: group.Heading,
			Items:   append([]string(nil), group.Items...),
		})
	}
	return cloned
}
