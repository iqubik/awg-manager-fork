package main

import (
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/updater"
)

func TestGenerate_GroupsAndOrder(t *testing.T) {
	commits := []Commit{
		{Subject: "feat(frontend): селектор канала"},
		{Subject: "fix(updater): выбор версии"},
		{Subject: "refactor(x): cleanup"},
		{Subject: "perf(y): faster loop"},
	}
	got := Generate(commits, "2.11.2+r95", "2026-05-25")
	want := "## [2.11.2+r95] - 2026-05-25\n\n" +
		"### Добавлено\n- feat(frontend): селектор канала\n\n" +
		"### Исправлено\n- fix(updater): выбор версии\n\n" +
		"### Рефакторинг\n- refactor(x): cleanup\n\n" +
		"### Производительность\n- perf(y): faster loop\n\n"
	if got != want {
		t.Errorf("Generate mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestGenerate_FiltersNoise(t *testing.T) {
	commits := []Commit{
		{Subject: "chore: bump deps"},
		{Subject: "ci: tweak workflow"},
		{Subject: "docs: readme"},
		{Subject: "test(x): add test"},
		{Subject: "style: format"},
		{Subject: "Merge branch 'develop' into x"},
		{Subject: "Update .gitignore"},
		{Subject: "upd(singbox): bump"},
		{Subject: "fix(scope) - dash separator not colon"},
	}
	if got := Generate(commits, "1.0.0", "2026-01-01"); got != "" {
		t.Errorf("expected empty output for all-noise input, got:\n%s", got)
	}
}

func TestGenerate_PrefixVariants(t *testing.T) {
	commits := []Commit{
		{Subject: "fix: no scope"},
		{Subject: "feat(a,b): comma scope"},
		{Subject: "feat!: breaking no scope"},
		{Subject: "fix(x)!: breaking with scope"},
	}
	got := Generate(commits, "1.0.0", "2026-01-01")
	want := "## [1.0.0] - 2026-01-01\n\n" +
		"### Добавлено\n- feat(a,b): comma scope\n- feat!: breaking no scope\n\n" +
		"### Исправлено\n- fix: no scope\n- fix(x)!: breaking with scope\n\n"
	if got != want {
		t.Errorf("prefix variants mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestGenerate_EmptyInput(t *testing.T) {
	if got := Generate(nil, "1.0.0", "2026-01-01"); got != "" {
		t.Errorf("expected empty for nil input, got %q", got)
	}
}

func TestGenerate_RoundTripParses(t *testing.T) {
	commits := []Commit{
		{Subject: "feat(frontend): селектор канала"},
		{Subject: "fix(updater): выбор версии"},
		{Subject: "refactor(x): cleanup"},
		{Subject: "chore: bump"},
		{Subject: "Merge branch 'develop'"},
	}
	block := Generate(commits, "2.11.2+r95", "2026-05-25")

	entries, err := updater.ParseChangelog(block)
	if err != nil {
		t.Fatalf("ParseChangelog error: %v", err)
	}
	e, ok := entries["2.11.2+r95"]
	if !ok {
		t.Fatalf("version 2.11.2+r95 not parsed; got keys %v", keysOf(entries))
	}
	if e.Date != "2026-05-25" {
		t.Errorf("date = %q, want 2026-05-25", e.Date)
	}
	if len(e.Groups) != 3 {
		t.Fatalf("groups = %d, want 3 (Добавлено/Исправлено/Рефакторинг)", len(e.Groups))
	}
	wantHeadings := []string{"Добавлено", "Исправлено", "Рефакторинг"}
	for i, want := range wantHeadings {
		if e.Groups[i].Heading != want {
			t.Errorf("group[%d].Heading = %q, want %q", i, e.Groups[i].Heading, want)
		}
		if len(e.Groups[i].Items) != 1 {
			t.Errorf("group[%d] items = %d, want 1", i, len(e.Groups[i].Items))
		}
	}
}

func TestParseCommits_StructuredBodyAndTrailers(t *testing.T) {
	input := "\x1eabc123\x1ffix(dev): align local build version with VERSION file\x1fлокальная dev-сборка теперь берёт base VERSION и добавляет +r0 сама.\n\nSigned-off-by: bot\n"
	commits := ParseCommits(input)
	if len(commits) != 1 {
		t.Fatalf("len(commits) = %d, want 1", len(commits))
	}
	if commits[0].Hash != "abc123" {
		t.Fatalf("hash = %q", commits[0].Hash)
	}
	if commits[0].Body != "локальная dev-сборка теперь берёт base VERSION и добавляет +r0 сама." {
		t.Fatalf("body = %q", commits[0].Body)
	}
}

func TestGenerate_IncludesCommentLine(t *testing.T) {
	commits := []Commit{{
		Subject: "fix(dev): align local build version with VERSION file",
		Body:    "локальная dev-сборка теперь берёт base VERSION и добавляет +r0 сама.",
	}}
	got := Generate(commits, "2.12.3.10+r2", "2026-06-12")
	want := "## [2.12.3.10+r2] - 2026-06-12\n\n" +
		"### Исправлено\n" +
		"- fix(dev): align local build version with VERSION file\n" +
		"  Комментарий: локальная dev-сборка теперь берёт base VERSION и добавляет +r0 сама.\n\n"
	if got != want {
		t.Fatalf("Generate mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestParseCommits_TruncatesUTF8BodyByRune(t *testing.T) {
	longBody := strings.Repeat("я", 520)
	input := "\x1eabc123\x1ffix(dev): utf8 body\x1f" + longBody

	commits := ParseCommits(input)
	if len(commits) != 1 {
		t.Fatalf("len(commits) = %d, want 1", len(commits))
	}
	if !strings.HasSuffix(commits[0].Body, "…") {
		t.Fatalf("body should end with ellipsis, got %q", commits[0].Body[len(commits[0].Body)-8:])
	}
	runes := []rune(commits[0].Body)
	if len(runes) != 501 {
		t.Fatalf("len(runes) = %d, want 501", len(runes))
	}
	for i, r := range runes[:500] {
		if r != 'я' {
			t.Fatalf("rune %d = %q, want %q", i, r, 'я')
		}
	}
	if runes[500] != '…' {
		t.Fatalf("last rune = %q, want ellipsis", runes[500])
	}
}

func keysOf(m map[string]updater.Entry) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}
