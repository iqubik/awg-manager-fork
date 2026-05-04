package subscription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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

func TestService_Delete_RemovesProxy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("vless://3a3b1c2e-9999-4321-aaaa-1234567890ab@example.com:443?security=tls&sni=h\n"))
	}))
	defer srv.Close()

	store, _ := NewStore(filepath.Join(t.TempDir(), "sub.json"))
	mutator := &fakeMutator{}
	svc := NewService(store, mutator)

	sub, _ := svc.Create(context.Background(), CreateInput{Label: "test", URL: srv.URL, Enabled: true})
	if err := svc.Delete(context.Background(), sub.ID, true); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(mutator.removedProxies) != 1 {
		t.Errorf("expected 1 RemoveProxy call, got %d", len(mutator.removedProxies))
	}
}

func TestService_Delete_RemovesMembersAndInbound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("vless://3a3b1c2e-9999-4321-aaaa-1234567890ab@example.com:443?security=tls&sni=h\n"))
	}))
	defer srv.Close()

	store, _ := NewStore(filepath.Join(t.TempDir(), "sub.json"))
	mutator := &fakeMutator{}
	svc := NewService(store, mutator)
	sub, _ := svc.Create(context.Background(), CreateInput{Label: "test", URL: srv.URL, Enabled: true})

	if err := svc.Delete(context.Background(), sub.ID, true); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if len(mutator.removedOutbounds) < 2 { // selector + at least 1 member
		t.Errorf("expected >=2 outbounds removed, got %d", len(mutator.removedOutbounds))
	}
	if len(mutator.removedInbounds) != 1 {
		t.Errorf("expected 1 inbound removed, got %d", len(mutator.removedInbounds))
	}
}
