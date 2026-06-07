package dnsroute

import (
	"reflect"
	"testing"
)

func TestApplyExcludesTextOverridesExcludesWhenPresent(t *testing.T) {
	raw := `
# excluded domains
.youtube.com
bad domain
*.example.com
10.0.0.0/8
999.999.999.999/99
# disabled.example.com
`
	list := DomainList{
		Excludes:       []string{"stale.example.com"},
		ExcludeSubnets: []string{"192.168.0.0/16"},
		ExcludesText:   &raw,
	}

	applyExcludesText(&list)

	wantExcludes := []string{
		"youtube.com",
		"10.0.0.0/8",
	}
	if !reflect.DeepEqual(list.Excludes, wantExcludes) {
		t.Fatalf("Excludes = %#v, want %#v", list.Excludes, wantExcludes)
	}
	if list.ExcludeSubnets != nil {
		t.Fatalf("ExcludeSubnets = %#v, want nil before split", list.ExcludeSubnets)
	}
}

func TestApplyExcludesTextDoesNothingWhenAbsent(t *testing.T) {
	list := DomainList{
		Excludes:       []string{"legacy.example.com"},
		ExcludeSubnets: []string{"10.0.0.0/8"},
	}

	applyExcludesText(&list)

	wantExcludes := []string{"legacy.example.com"}
	wantSubnets := []string{"10.0.0.0/8"}

	if !reflect.DeepEqual(list.Excludes, wantExcludes) {
		t.Fatalf("Excludes = %#v, want %#v", list.Excludes, wantExcludes)
	}
	if !reflect.DeepEqual(list.ExcludeSubnets, wantSubnets) {
		t.Fatalf("ExcludeSubnets = %#v, want %#v", list.ExcludeSubnets, wantSubnets)
	}
}

func TestExcludesTextCanBeSplitIntoDomainsAndSubnets(t *testing.T) {
	raw := `
# comments are ignored
.example.com
10.0.0.0/8
2001:db8::/32
999.999.999.999/99
`
	list := DomainList{
		ExcludesText: &raw,
	}

	applyExcludesText(&list)

	domains, subnets := splitDomainsAndSubnets(deduplicateDomains(list.Excludes))

	wantDomains := []string{"example.com"}
	wantSubnets := []string{"10.0.0.0/8", "2001:db8::/32"}

	if !reflect.DeepEqual(domains, wantDomains) {
		t.Fatalf("domains = %#v, want %#v", domains, wantDomains)
	}
	if !reflect.DeepEqual(subnets, wantSubnets) {
		t.Fatalf("subnets = %#v, want %#v", subnets, wantSubnets)
	}
}
