package main

import (
	"fmt"
	"regexp"
	"strings"
)

type Commit struct {
	Hash    string
	Subject string
	Body    string
}

// commitRe matches a conventional-commit subject with a strict ":" separator,
// optional "(scope)" and optional "!" breaking marker. Capture group 1 is the
// type. " - " separators and non-conventional subjects do not match.
var commitRe = regexp.MustCompile(`^(feat|fix|refactor|perf)(\([^)]+\))?!?:`)

// sections maps a commit type to its changelog heading; slice order is the
// emission order of sections.
var sections = []struct {
	typ     string
	heading string
}{
	{"feat", "Добавлено"},
	{"fix", "Исправлено"},
	{"refactor", "Рефакторинг"},
	{"perf", "Производительность"},
}

var trailerRe = regexp.MustCompile(`(?im)^(co-authored-by|signed-off-by|reviewed-by):\s+.+$`)

// ParseCommits supports two stdin formats:
// 1. Legacy: one conventional-commit subject per line.
// 2. Structured: <RS><hash><US><subject><US><body> records from git log.
func ParseCommits(input string) []Commit {
	if strings.ContainsRune(input, '\x1e') || strings.ContainsRune(input, '\x1f') {
		return parseStructuredCommits(input)
	}
	return parseLegacyCommits(input)
}

func parseLegacyCommits(input string) []Commit {
	lines := strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n")
	commits := make([]Commit, 0, len(lines))
	for _, line := range lines {
		subject := strings.TrimSpace(line)
		if subject == "" {
			continue
		}
		commits = append(commits, Commit{Subject: subject})
	}
	return commits
}

func parseStructuredCommits(input string) []Commit {
	input = strings.ReplaceAll(input, "\r\n", "\n")
	records := strings.Split(input, "\x1e")
	commits := make([]Commit, 0, len(records))
	for _, record := range records {
		record = strings.TrimSpace(record)
		if record == "" {
			continue
		}
		parts := strings.SplitN(record, "\x1f", 3)
		commit := Commit{}
		if len(parts) > 0 {
			commit.Hash = strings.TrimSpace(parts[0])
		}
		if len(parts) > 1 {
			commit.Subject = strings.TrimSpace(parts[1])
		}
		if len(parts) > 2 {
			commit.Body = cleanCommitBody(parts[2])
		}
		if commit.Subject == "" {
			continue
		}
		commits = append(commits, commit)
	}
	return commits
}

func cleanCommitBody(body string) string {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}

	lines := strings.Split(body, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		if trailerRe.MatchString(strings.TrimSpace(line)) {
			continue
		}
		filtered = append(filtered, strings.TrimRight(line, " \t"))
	}
	body = strings.TrimSpace(strings.Join(filtered, "\n"))
	if body == "" {
		return ""
	}

	paragraphs := strings.Split(body, "\n\n")
	first := strings.TrimSpace(paragraphs[0])
	if first == "" {
		return ""
	}

	first = strings.Join(strings.Fields(first), " ")
	const maxLen = 500
	runes := []rune(first)
	if len(runes) > maxLen {
		first = strings.TrimSpace(string(runes[:maxLen])) + "…"
	}
	return first
}

// Generate turns commits into a single Keep-a-Changelog block parseable by
// internal/updater.ParseChangelog. Only feat/fix/refactor/perf are kept.
func Generate(commits []Commit, version, date string) string {
	buckets := map[string][]Commit{}
	for _, commit := range commits {
		subject := strings.TrimSpace(commit.Subject)
		if subject == "" || strings.HasPrefix(subject, "Merge ") {
			continue
		}
		m := commitRe.FindStringSubmatch(subject)
		if m == nil {
			continue
		}
		commit.Subject = subject
		buckets[m[1]] = append(buckets[m[1]], commit)
	}

	total := 0
	for _, sec := range sections {
		total += len(buckets[sec.typ])
	}
	if total == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## [%s] - %s\n\n", version, date)
	for _, sec := range sections {
		items := buckets[sec.typ]
		if len(items) == 0 {
			continue
		}
		fmt.Fprintf(&b, "### %s\n", sec.heading)
		for _, it := range items {
			fmt.Fprintf(&b, "- %s\n", it.Subject)
			if it.Body != "" {
				fmt.Fprintf(&b, "  Комментарий: %s\n", it.Body)
			}
		}
		b.WriteString("\n")
	}
	return b.String()
}
