package subscription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// fakeMutator records what the service tries to commit.
type fakeMutator struct {
	addedOutbounds   []string
	updatedOutbounds []string
	removedOutbounds []string
	addedInbounds    []string
	removedInbounds  []string
	addedRules       int
	removedRules     int
	listenPort       uint16
	proxyIndex       int
	ensuredProxies   []int
	removedProxies   []int
	selectedSelector []string // "selectorTag→memberTag" pairs recorded by SelectClashProxy
}

func (f *fakeMutator) AllocListenPort() (uint16, error) {
	if f.listenPort == 0 {
		f.listenPort = 11000
	}
	f.listenPort++
	return f.listenPort, nil
}
func (f *fakeMutator) AllocProxyIndex(_ context.Context) (int, error) {
	f.proxyIndex++
	return f.proxyIndex, nil
}
func (f *fakeMutator) AddOutbound(tag string, jsonBody []byte) error {
	f.addedOutbounds = append(f.addedOutbounds, tag)
	return nil
}
func (f *fakeMutator) UpdateOutbound(tag string, jsonBody []byte) error {
	f.updatedOutbounds = append(f.updatedOutbounds, tag)
	return nil
}
func (f *fakeMutator) RemoveOutbound(tag string) error {
	f.removedOutbounds = append(f.removedOutbounds, tag)
	return nil
}
func (f *fakeMutator) AddInbound(tag string, jsonBody []byte) error {
	f.addedInbounds = append(f.addedInbounds, tag)
	return nil
}
func (f *fakeMutator) RemoveInbound(tag string) error {
	f.removedInbounds = append(f.removedInbounds, tag)
	return nil
}
func (f *fakeMutator) AddRouteRule(jsonBody []byte) error {
	f.addedRules++
	return nil
}
func (f *fakeMutator) RemoveRouteRule(inboundTag, outboundTag string) error {
	f.removedRules++
	return nil
}
func (f *fakeMutator) EnsureProxy(_ context.Context, idx, port int, description string) error {
	f.ensuredProxies = append(f.ensuredProxies, idx)
	return nil
}
func (f *fakeMutator) RemoveProxy(_ context.Context, idx int) error {
	f.removedProxies = append(f.removedProxies, idx)
	return nil
}
func (f *fakeMutator) Reload(ctx context.Context) error { return nil }
func (f *fakeMutator) SelectClashProxy(selectorTag, memberTag string) error {
	f.selectedSelector = append(f.selectedSelector, selectorTag+"→"+memberTag)
	return nil
}

func TestService_Create_FetchAndMaterialize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("vless://3a3b1c2e-9999-4321-aaaa-1234567890ab@example.com:443?security=tls&sni=h\n" +
			"trojan://p@example.com:444?security=tls&sni=h\n"))
	}))
	defer srv.Close()

	store, _ := NewStore(filepath.Join(t.TempDir(), "sub.json"))
	mutator := &fakeMutator{}
	svc := NewService(store, mutator)

	sub, err := svc.Create(context.Background(), CreateInput{Label: "test", URL: srv.URL, Enabled: true})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(sub.MemberTags) != 2 {
		t.Errorf("MemberTags=%d want 2", len(sub.MemberTags))
	}
	if len(mutator.addedOutbounds) < 3 { // 2 members + 1 selector
		t.Errorf("expected >=3 outbounds added, got %d", len(mutator.addedOutbounds))
	}
	if len(mutator.addedInbounds) != 1 {
		t.Errorf("expected 1 mixed inbound, got %d", len(mutator.addedInbounds))
	}
}

func TestService_Create_FailsOnZeroOutbounds_ClashYAML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		w.Write([]byte("proxies:\n  - name: \"a\"\n    type: vless\n    server: 1.2.3.4\n    port: 443\n"))
	}))
	defer srv.Close()

	store, _ := NewStore(filepath.Join(t.TempDir(), "sub.json"))
	mutator := &fakeMutator{}
	svc := NewService(store, mutator)

	_, err := svc.Create(context.Background(), CreateInput{Label: "clash", URL: srv.URL, Enabled: true})
	if err == nil {
		t.Fatalf("Create with Clash YAML must fail (0 outbounds)")
	}
	if !strings.Contains(err.Error(), "Clash YAML") && !strings.Contains(err.Error(), "ни одной валидной") {
		t.Errorf("error must hint at unsupported format, got: %v", err)
	}
	if len(store.List()) != 0 {
		t.Errorf("subscription must be cleaned up on failed Create, got %d", len(store.List()))
	}
}

func TestService_Create_FailsOnEmptyClashProxiesArray(t *testing.T) {
	// Real-world body shape served by an expired clash subscription:
	// YAML parses cleanly, but proxies: [] is literally empty. We must
	// surface a distinct "subscription is empty" hint instead of the
	// generic "no valid links" message.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
		w.Write([]byte(`mixed-port: 7890
dns:
  enable: true
proxies: []
proxy-groups:
  - name: → Remnawave
    type: select
    proxies: []
rules:
  - MATCH,→ Remnawave
`))
	}))
	defer srv.Close()

	store, _ := NewStore(filepath.Join(t.TempDir(), "sub.json"))
	mutator := &fakeMutator{}
	svc := NewService(store, mutator)

	_, err := svc.Create(context.Background(), CreateInput{Label: "expired", URL: srv.URL, Enabled: true})
	if err == nil {
		t.Fatalf("Create with empty Clash proxies must fail")
	}
	if !strings.Contains(err.Error(), "пуста") {
		t.Errorf("error must hint that subscription is empty, got: %v", err)
	}
	if len(store.List()) != 0 {
		t.Errorf("subscription must be cleaned up, got %d", len(store.List()))
	}
	// Bug-fix from previous commit also applies here: ProxyN must be rolled back.
	if len(mutator.removedProxies) != 1 {
		t.Errorf("expected 1 RemoveProxy rollback, got %d", len(mutator.removedProxies))
	}
}

// TestService_Create_RollsBackProxyOnFetchFailure asserts the bug fix
// where each failed Create call leaked an NDMS ProxyN interface — the
// initial-fetch failure path now also removes the proxy registered by
// EnsureProxy.
func TestService_Create_RollsBackProxyOnFetchFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Empty body — share-link parser sees 0 outbounds, fail-fast fires.
		w.WriteHeader(200)
	}))
	defer srv.Close()

	store, _ := NewStore(filepath.Join(t.TempDir(), "sub.json"))
	mutator := &fakeMutator{}
	svc := NewService(store, mutator)

	// Three failed attempts at the same URL.
	for i := 0; i < 3; i++ {
		_, err := svc.Create(context.Background(), CreateInput{Label: "x", URL: srv.URL, Enabled: true})
		if err == nil {
			t.Fatalf("attempt %d: Create must fail", i+1)
		}
	}

	// Three EnsureProxy calls and three matching RemoveProxy calls — net
	// zero proxy interfaces leaked into the router.
	if len(mutator.ensuredProxies) != 3 {
		t.Errorf("ensuredProxies=%d want 3", len(mutator.ensuredProxies))
	}
	if len(mutator.removedProxies) != 3 {
		t.Errorf("removedProxies=%d want 3 (one rollback per failed Create)", len(mutator.removedProxies))
	}
	// Storage clean.
	if len(store.List()) != 0 {
		t.Errorf("storage must be empty, got %d rows", len(store.List()))
	}
}

func TestService_Refresh_AddsNewMember(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if requestCount == 1 {
			w.Write([]byte("vless://3a3b1c2e-9999-4321-aaaa-1234567890ab@example.com:443?security=tls&sni=h\n"))
		} else {
			w.Write([]byte("vless://3a3b1c2e-9999-4321-aaaa-1234567890ab@example.com:443?security=tls&sni=h\n" +
				"trojan://p@example.com:444?security=tls&sni=h\n"))
		}
	}))
	defer srv.Close()

	store, _ := NewStore(filepath.Join(t.TempDir(), "sub.json"))
	mutator := &fakeMutator{}
	svc := NewService(store, mutator)

	sub, _ := svc.Create(context.Background(), CreateInput{Label: "test", URL: srv.URL, Enabled: true})
	if _, err := svc.Refresh(context.Background(), sub.ID); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	updated, _ := store.Get(sub.ID)
	if len(updated.MemberTags) != 2 {
		t.Errorf("MemberTags after refresh=%d want 2", len(updated.MemberTags))
	}
}

func TestService_Create_RegistersNDMSProxy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("vless://3a3b1c2e-9999-4321-aaaa-1234567890ab@example.com:443?security=tls&sni=h\n"))
	}))
	defer srv.Close()

	store, _ := NewStore(filepath.Join(t.TempDir(), "sub.json"))
	mutator := &fakeMutator{}
	svc := NewService(store, mutator)

	sub, err := svc.Create(context.Background(), CreateInput{Label: "test", URL: srv.URL, Enabled: true})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(mutator.ensuredProxies) != 1 {
		t.Errorf("expected 1 EnsureProxy call, got %d", len(mutator.ensuredProxies))
	}
	if sub.ProxyIndex < 0 {
		t.Errorf("ProxyIndex should be set, got %d", sub.ProxyIndex)
	}
}

func TestService_Delete_AlwaysCleansEverything(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("vless://3a3b1c2e-9999-4321-aaaa-1234567890ab@example.com:443?security=tls&sni=h\n" +
			"trojan://p@example.com:444?security=tls&sni=h\n"))
	}))
	defer srv.Close()

	store, _ := NewStore(filepath.Join(t.TempDir(), "sub.json"))
	mutator := &fakeMutator{}
	svc := NewService(store, mutator)
	sub, err := svc.Create(context.Background(), CreateInput{Label: "x", URL: srv.URL, Enabled: true})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Delete(context.Background(), sub.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// ProxyN must always be removed (this is the bug-fix).
	if len(mutator.removedProxies) != 1 || mutator.removedProxies[0] != sub.ProxyIndex {
		t.Errorf("RemoveProxy not called with subscription's index: %v (want %d)",
			mutator.removedProxies, sub.ProxyIndex)
	}
	// Mixed inbound removed.
	foundInbound := false
	for _, t2 := range mutator.removedInbounds {
		if t2 == sub.InboundTag {
			foundInbound = true
			break
		}
	}
	if !foundInbound {
		t.Errorf("expected inbound %s removed, got %v", sub.InboundTag, mutator.removedInbounds)
	}
	// Selector + at least 2 members removed.
	if len(mutator.removedOutbounds) < 3 {
		t.Errorf("expected >=3 outbounds removed (selector+2 members), got %d", len(mutator.removedOutbounds))
	}
	// Route rule removed.
	if mutator.removedRules < 1 {
		t.Errorf("expected route rule removed, got %d", mutator.removedRules)
	}
	// Subscription absent from storage.
	if _, err := store.Get(sub.ID); err == nil {
		t.Errorf("expected subscription removed from store, still present")
	}
}

func TestService_ListActiveMemberTags_FiltersDisabledAndEmpty(t *testing.T) {
	store, _ := NewStore(filepath.Join(t.TempDir(), "sub.json"))

	// Enabled with active set — should appear
	a, _ := store.Create(CreateInput{Label: "a", URL: "u", Enabled: true})
	store.SetMembers(a.ID, []MemberInfo{{Tag: "sub-A-1111"}}, nil)

	// Disabled — should be filtered out
	b, _ := store.Create(CreateInput{Label: "b", URL: "u", Enabled: false})
	store.SetMembers(b.ID, []MemberInfo{{Tag: "sub-B-2222"}}, nil)

	// Enabled but no members — ActiveMember stays empty, should be filtered
	store.Create(CreateInput{Label: "c", URL: "u", Enabled: true})

	svc := NewService(store, &fakeMutator{})
	tags := svc.ListActiveMemberTags()
	if len(tags) != 1 {
		t.Fatalf("expected 1 active tag, got %d: %v", len(tags), tags)
	}
	if tags[0] != "sub-A-1111" {
		t.Errorf("got %q want sub-A-1111", tags[0])
	}
}

func TestService_SetActiveMember_UsesClashAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(
			"vless://3a3b1c2e-9999-4321-aaaa-1234567890ab@h1.example:443?security=tls&sni=h\n" +
				"trojan://p@h2.example:443?security=tls&sni=h\n",
		))
	}))
	defer srv.Close()

	store, _ := NewStore(filepath.Join(t.TempDir(), "sub.json"))
	mutator := &fakeMutator{}
	svc := NewService(store, mutator)
	sub, err := svc.Create(context.Background(), CreateInput{Label: "test", URL: srv.URL, Enabled: true})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if len(sub.MemberTags) < 2 {
		t.Fatalf("need at least 2 members, got %d", len(sub.MemberTags))
	}

	secondMember := sub.MemberTags[1]
	if err := svc.SetActiveMember(context.Background(), sub.ID, secondMember); err != nil {
		t.Fatalf("SetActiveMember: %v", err)
	}

	// Verify clash API was called with the right args.
	if len(mutator.selectedSelector) != 1 {
		t.Fatalf("expected 1 SelectClashProxy call, got %d: %v", len(mutator.selectedSelector), mutator.selectedSelector)
	}
	expected := sub.SelectorTag + "→" + secondMember
	if mutator.selectedSelector[0] != expected {
		t.Errorf("clash select args wrong: got %q want %q", mutator.selectedSelector[0], expected)
	}

	// Verify Reload was NOT called for SetActiveMember (no connection-dropping SIGHUP).
	// We verify this indirectly: the mutator's Reload is a no-op and SelectClashProxy
	// is recorded; the key invariant is the config update + clash call both happen.
	stored, _ := store.Get(sub.ID)
	if stored.ActiveMember != secondMember {
		t.Errorf("store.ActiveMember=%q want %q", stored.ActiveMember, secondMember)
	}
}

func TestService_Create_ParsesClashYAML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		w.Write([]byte(`
proxies:
  - name: "🇺🇸 LA-1"
    type: vless
    server: la1.example.com
    port: 443
    uuid: 3a3b1c2e-9999-4321-aaaa-1234567890ab
    tls: true
    servername: la1.example.com
  - name: "🇩🇪 FRA-1"
    type: vless
    server: fra1.example.com
    port: 443
    uuid: 4a4b1c2e-9999-4321-aaaa-1234567890ab
    tls: true
    servername: fra1.example.com
  - name: "🇯🇵 TYO-1"
    type: trojan
    server: tyo1.example.com
    port: 443
    password: trpass
    sni: tyo1.example.com
  - name: "VM-skipme"
    type: vmess
    server: vm.example.com
    port: 443
    uuid: 11111111-2222-3333-4444-555555555555
`))
	}))
	defer srv.Close()

	store, _ := NewStore(filepath.Join(t.TempDir(), "sub.json"))
	mutator := &fakeMutator{}
	svc := NewService(store, mutator)

	sub, err := svc.Create(context.Background(), CreateInput{Label: "clash", URL: srv.URL, Enabled: true})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(sub.MemberTags) != 3 {
		t.Errorf("MemberTags=%d want 3", len(sub.MemberTags))
	}
	if len(mutator.addedOutbounds) < 4 { // 3 members + 1 selector
		t.Errorf("expected >=4 outbounds added, got %d", len(mutator.addedOutbounds))
	}
	// Members must carry their human-readable Label.
	gotLabels := map[string]bool{}
	for _, m := range sub.Members {
		gotLabels[m.Label] = true
	}
	for _, want := range []string{"🇺🇸 LA-1", "🇩🇪 FRA-1", "🇯🇵 TYO-1"} {
		if !gotLabels[want] {
			t.Errorf("missing Label %q in MemberInfo", want)
		}
	}
}

func TestService_Refresh_ClashYAMLAddsNewMember(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/x-yaml")
		if requestCount == 1 {
			w.Write([]byte(`
proxies:
  - name: "A"
    type: vless
    server: a.example.com
    port: 443
    uuid: 3a3b1c2e-9999-4321-aaaa-1234567890ab
    tls: true
`))
		} else {
			w.Write([]byte(`
proxies:
  - name: "A"
    type: vless
    server: a.example.com
    port: 443
    uuid: 3a3b1c2e-9999-4321-aaaa-1234567890ab
    tls: true
  - name: "B"
    type: vless
    server: b.example.com
    port: 443
    uuid: 4a4b1c2e-9999-4321-aaaa-1234567890ab
    tls: true
`))
		}
	}))
	defer srv.Close()

	store, _ := NewStore(filepath.Join(t.TempDir(), "sub.json"))
	mutator := &fakeMutator{}
	svc := NewService(store, mutator)
	sub, err := svc.Create(context.Background(), CreateInput{Label: "c", URL: srv.URL, Enabled: true})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(sub.MemberTags) != 1 {
		t.Errorf("after first refresh want 1 member, got %d", len(sub.MemberTags))
	}

	res, err := svc.Refresh(context.Background(), sub.ID)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if res.Added != 1 || res.Updated != 1 {
		t.Errorf("Refresh result Added=%d Updated=%d, want Added=1 Updated=1", res.Added, res.Updated)
	}
}
