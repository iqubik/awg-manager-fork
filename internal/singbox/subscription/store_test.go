package subscription

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestStore(t *testing.T) (*Store, func()) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "subscriptions.json")
	s, err := NewStore(path)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s, func() { _ = os.Remove(path) }
}

func TestStore_CreateGetList(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	in := CreateInput{Label: "test", URL: "https://x", RefreshHours: 24, Enabled: true}
	got, err := s.Create(in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if got.ID == "" {
		t.Errorf("expected non-empty ID")
	}
	if got.Label != "test" {
		t.Errorf("label=%q", got.Label)
	}

	fetched, err := s.Get(got.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fetched.ID != got.ID {
		t.Errorf("Get returned wrong subscription")
	}

	all := s.List()
	if len(all) != 1 {
		t.Errorf("List len=%d want 1", len(all))
	}
}

func TestStore_Update(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	created, _ := s.Create(CreateInput{Label: "old", URL: "u", Enabled: true})
	newLabel := "new"
	patch := UpdatePatch{Label: &newLabel}
	updated, err := s.Update(created.ID, patch)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Label != "new" {
		t.Errorf("label=%q", updated.Label)
	}
}

func TestStore_Delete(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	created, _ := s.Create(CreateInput{Label: "del", URL: "u"})
	if err := s.Delete(created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(created.ID); err == nil {
		t.Error("expected error on Get after Delete")
	}
}

func TestStore_PersistsAcrossReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subscriptions.json")
	s1, _ := NewStore(path)
	s1.Create(CreateInput{Label: "persisted", URL: "u"})
	s2, err := NewStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if len(s2.List()) != 1 {
		t.Errorf("expected 1 subscription after reload, got %d", len(s2.List()))
	}
}

func TestStore_UpdateState(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	created, _ := s.Create(CreateInput{Label: "state", URL: "u"})
	now := time.Now()
	res := RefreshResult{When: now, Added: 3, Updated: 1}
	if err := s.UpdateState(created.ID, res); err != nil {
		t.Fatalf("UpdateState: %v", err)
	}
	got, _ := s.Get(created.ID)
	if got.LastFetched.IsZero() {
		t.Errorf("expected LastFetched updated")
	}
}

func TestStore_ConcurrentReadWrite(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.List()
		}()
		go func(i int) {
			defer wg.Done()
			s.Create(CreateInput{Label: "c", URL: "u"})
		}(i)
	}
	wg.Wait()
	if len(s.List()) != 50 {
		t.Errorf("len=%d want 50", len(s.List()))
	}
}

func TestStore_SetIsDefaultRoute_AtomicallyClearsOthers(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	a, _ := s.Create(CreateInput{Label: "a", URL: "u"})
	b, _ := s.Create(CreateInput{Label: "b", URL: "u"})

	if err := s.SetIsDefaultRoute(a.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetIsDefaultRoute(b.ID, true); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(a.ID)
	if got.IsDefaultRoute {
		t.Error("a.IsDefaultRoute should be false after b became default")
	}
	got2, _ := s.Get(b.ID)
	if !got2.IsDefaultRoute {
		t.Error("b.IsDefaultRoute should be true")
	}
}

func TestStore_MaybeRefresh(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()

	never, _ := s.Create(CreateInput{Label: "manual", URL: "u", RefreshHours: 0, Enabled: true})
	due, _ := s.Create(CreateInput{Label: "due", URL: "u", RefreshHours: 1, Enabled: true})
	disabled, _ := s.Create(CreateInput{Label: "off", URL: "u", RefreshHours: 1, Enabled: false})
	_ = never
	_ = disabled

	picked := s.MaybeRefresh(time.Now().Add(2 * time.Hour))
	if len(picked) != 1 {
		t.Errorf("expected 1 due, got %d", len(picked))
	}
	if len(picked) > 0 && picked[0].ID != due.ID {
		t.Errorf("picked wrong subscription: %v", picked)
	}
}
