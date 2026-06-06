package dnsroute

import (
	"reflect"
	"testing"
)

func TestActiveManualDomainsFromTextSkipsCommentAndBlankLines(t *testing.T) {
	got := activeManualDomainsFromText(`
# video services
youtube.com

  # google video cdn
.googlevideo.com
10.0.0.0/8
`)

	want := []string{
		"youtube.com",
		"googlevideo.com",
		"10.0.0.0/8",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("activeManualDomainsFromText() = %#v, want %#v", got, want)
	}
}

func TestApplyManualTextOverridesManualDomainsWhenPresent(t *testing.T) {
	raw := "# old note\nexample.com\n# disabled.example.com"
	list := DomainList{
		ManualDomains: []string{"stale.example.com"},
		ManualText:    &raw,
	}

	applyManualText(&list)

	want := []string{"example.com"}
	if !reflect.DeepEqual(list.ManualDomains, want) {
		t.Fatalf("ManualDomains = %#v, want %#v", list.ManualDomains, want)
	}
}

func TestApplyManualTextDoesNothingWhenAbsent(t *testing.T) {
	list := DomainList{
		ManualDomains: []string{"legacy.example.com"},
	}

	applyManualText(&list)

	want := []string{"legacy.example.com"}
	if !reflect.DeepEqual(list.ManualDomains, want) {
		t.Fatalf("ManualDomains = %#v, want %#v", list.ManualDomains, want)
	}
}

func TestActiveManualDomainsFromTextSkipsInvalidLines(t *testing.T) {
	got := activeManualDomainsFromText(`
# comment
youtube.com
bad domain
*.example.com
10.0.0.0/8
`)

	want := []string{
		"youtube.com",
		"10.0.0.0/8",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("activeManualDomainsFromText() = %#v, want %#v", got, want)
	}
}

func TestActiveManualDomainsFromTextSkipsInvalidCIDR(t *testing.T) {
	got := activeManualDomainsFromText(`
youtube.com
999.999.999.999/99
10.0.0.0/8
2001:db8::/32
bad:value/999
`)

	want := []string{
		"youtube.com",
		"10.0.0.0/8",
		"2001:db8::/32",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("activeManualDomainsFromText() = %#v, want %#v", got, want)
	}
}

func TestActiveManualDomainsFromTextStripsLeadingDots(t *testing.T) {
	got := activeManualDomainsFromText(`
.ru
...example.com
geosite:GOOGLE
geoip:ru
`)

	want := []string{
		"ru",
		"example.com",
		"geosite:GOOGLE",
		"geoip:ru",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("activeManualDomainsFromText() = %#v, want %#v", got, want)
	}
}
