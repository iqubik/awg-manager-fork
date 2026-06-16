package updater

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseChangelog_SingleVersion(t *testing.T) {
	md := `# Changelog

## [2.8.0] - 2026-04-17

### Added
- feat: first feature
- feat: second feature

### Fixed
- fix: bug
`
	got, err := ParseChangelog(md)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Entry{
		"2.8.0": {
			Version: "2.8.0",
			Date:    "2026-04-17",
			Groups: []Group{
				{Heading: "Added", Items: []string{"feat: first feature", "feat: second feature"}},
				{Heading: "Fixed", Items: []string{"fix: bug"}},
			},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestParseChangelog_MultipleVersions(t *testing.T) {
	md := `# Changelog

## [2.8.0] - 2026-04-17

### Added
- feat: latest

## [2.7.11] - 2026-04-16

### Fixed
- fix: older
`
	got, err := ParseChangelog(md)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 versions, got %d", len(got))
	}
	if got["2.8.0"].Groups[0].Items[0] != "feat: latest" {
		t.Errorf("2.8.0 wrong: %+v", got["2.8.0"])
	}
	if got["2.7.11"].Groups[0].Items[0] != "fix: older" {
		t.Errorf("2.7.11 wrong: %+v", got["2.7.11"])
	}
}

func TestParseChangelog_EmptyGroupsDropped(t *testing.T) {
	md := `## [1.0.0] - 2026-01-01

### Added

### Fixed
- fix: real
`
	got, _ := ParseChangelog(md)
	if len(got["1.0.0"].Groups) != 1 {
		t.Errorf("empty 'Added' must not produce a group: %+v", got["1.0.0"].Groups)
	}
	if got["1.0.0"].Groups[0].Heading != "Fixed" {
		t.Errorf("unexpected heading: %+v", got["1.0.0"].Groups)
	}
}

func TestParseChangelog_StarBulletsAndWhitespace(t *testing.T) {
	md := "## [1.0.0] - 2026-01-01\n\n### Added\n\n*  starred with trailing space  \n-   dashed\n"
	got, _ := ParseChangelog(md)
	items := got["1.0.0"].Groups[0].Items
	if len(items) != 2 || items[0] != "starred with trailing space" || items[1] != "dashed" {
		t.Errorf("items = %+v", items)
	}
}

func TestParseChangelog_UnknownHeadingPreserved(t *testing.T) {
	md := `## [1.0.0] - 2026-01-01

### Custom
- item
`
	got, _ := ParseChangelog(md)
	if got["1.0.0"].Groups[0].Heading != "Custom" {
		t.Errorf("unknown heading must pass through, got %+v", got["1.0.0"].Groups)
	}
}

func TestParseChangelog_TextBeforeFirstVersionIgnored(t *testing.T) {
	md := `# Changelog

Some intro paragraph.

- a stray bullet

## [1.0.0] - 2026-01-01

### Fixed
- fix: one
`
	got, _ := ParseChangelog(md)
	if len(got) != 1 {
		t.Errorf("intro must be ignored, got %+v", got)
	}
}

func TestParseChangelog_GroupBeforeVersionIgnored(t *testing.T) {
	md := `### Added
- item without a version
`
	got, _ := ParseChangelog(md)
	if len(got) != 0 {
		t.Errorf("group without version must not emit an entry: %+v", got)
	}
}

func TestParseChangelog_Empty(t *testing.T) {
	got, err := ParseChangelog("")
	if err != nil || len(got) != 0 {
		t.Errorf("empty input should be (empty map, nil): %v %v", got, err)
	}
}

func TestParseChangelog_ProseLinesUnderGroup(t *testing.T) {
	md := `## [2.11.1] - 2026-05-23

### Два hot-fix в догонку

**Proxy интерфейсы роутера** - исключены из наблюдения.
**Запросы к роутеру** - скорректированы для Wireguard/peer и Wireguard/wireguard/asc путей.
`
	got, err := ParseChangelog(md)
	if err != nil {
		t.Fatal(err)
	}
	entry := got["2.11.1"]
	if len(entry.Groups) != 1 {
		t.Fatalf("groups = %+v", entry.Groups)
	}
	want := []string{
		"**Proxy интерфейсы роутера** - исключены из наблюдения.",
		"**Запросы к роутеру** - скорректированы для Wireguard/peer и Wireguard/wireguard/asc путей.",
	}
	if !reflect.DeepEqual(entry.Groups[0].Items, want) {
		t.Errorf("items = %+v, want %+v", entry.Groups[0].Items, want)
	}
}

func TestParseChangelog_IntroBeforeFirstGroup(t *testing.T) {
	md := `## [2.11.0] - 2026-05-23

Релиз посвящён производительности и стабильности: разговор с роутером ускорился.

### Производительность

- **NDMS-запросы ускорены** — средняя задержка снижена.
`
	got, err := ParseChangelog(md)
	if err != nil {
		t.Fatal(err)
	}
	entry := got["2.11.0"]
	if len(entry.Groups) != 2 {
		t.Fatalf("groups = %+v, want intro + Производительность", entry.Groups)
	}
	if entry.Groups[0].Heading != "" {
		t.Errorf("intro heading = %q, want empty", entry.Groups[0].Heading)
	}
	if len(entry.Groups[0].Items) != 1 || !strings.Contains(entry.Groups[0].Items[0], "производительности") {
		t.Errorf("intro items = %+v", entry.Groups[0].Items)
	}
	if entry.Groups[1].Heading != "Производительность" {
		t.Errorf("second group = %+v", entry.Groups[1])
	}
}

func TestParseChangelog_HorizontalRulesIgnored(t *testing.T) {
	md := `## [1.0.0] - 2026-01-01

Intro before rule.

---

### Added
- one
`
	got, _ := ParseChangelog(md)
	items := got["1.0.0"].Groups
	var all []string
	for _, g := range items {
		all = append(all, g.Items...)
	}
	for _, item := range all {
		if item == "---" {
			t.Errorf("horizontal rule must not become a changelog item: %+v", all)
		}
	}
}

func TestParseChangelog_MixedBulletsAndProse(t *testing.T) {
	md := `## [2.11.2] - 2026-05-23

### Исправлено

- **Batch запросы** - Исправления batch
**Extra prose** - без маркера
`
	got, _ := ParseChangelog(md)
	items := got["2.11.2"].Groups[0].Items
	if len(items) != 2 {
		t.Fatalf("items = %+v", items)
	}
	if items[0] != "**Batch запросы** - Исправления batch" {
		t.Errorf("bullet item = %q", items[0])
	}
	if items[1] != "**Extra prose** - без маркера" {
		t.Errorf("prose item = %q", items[1])
	}
}

func TestParseChangelog_IndentedContinuationBelongsToPreviousItem(t *testing.T) {
	md := `## [2.12.3.10+r1] - 2026-06-12

### Исправлено
- fix(dev): subject
  Комментарий: body text
`
	got, err := ParseChangelog(md)
	if err != nil {
		t.Fatal(err)
	}
	items := got["2.12.3.10+r1"].Groups[0].Items
	if len(items) != 1 {
		t.Fatalf("items = %+v", items)
	}
	if items[0] != "fix(dev): subject\nКомментарий: body text" {
		t.Fatalf("item = %q", items[0])
	}
}
