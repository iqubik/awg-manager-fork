package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchChangelog_CachesAcrossCalls(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte("## [1.0.0] - 2026-01-01\n\n### Added\n- item\n"))
	}))
	defer srv.Close()

	c := newChangelogFetcher(srv.URL, "", 10*time.Minute, nil)

	for i := 0; i < 3; i++ {
		entries, err := c.Fetch(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := entries["1.0.0"]; !ok {
			t.Fatalf("iter %d: 1.0.0 missing", i)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("expected 1 HTTP hit, got %d", got)
	}
}

func TestFetchChangelog_ErrorNotCached(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := newChangelogFetcher(srv.URL, "", 10*time.Minute, nil)

	for i := 0; i < 2; i++ {
		if _, err := c.Fetch(context.Background()); err == nil {
			t.Fatalf("iter %d: expected error, got nil", i)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Errorf("errors must NOT populate cache; expected 2 HTTP hits, got %d", got)
	}
}

func TestFetchChangelog_MergesPrimaryAndSecondary(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(
			"## [1.0.0] - 2026-01-01\n\n### Added\n- fork item\n\n### Fixed\n- shared item\n",
		))
	}))
	defer primary.Close()

	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(
			"## [1.0.0] - 2025-12-30\n\n### Fixed\n- shared item\n- upstream item\n\n## [0.9.0] - 2025-12-01\n\n### Added\n- old upstream item\n",
		))
	}))
	defer secondary.Close()

	c := newChangelogFetcher(primary.URL, secondary.URL, 10*time.Minute, nil)
	entries, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	entry := entries["1.0.0"]
	if entry.Date != "2026-01-01" {
		t.Fatalf("date = %q, want primary date", entry.Date)
	}
	if len(entry.Groups) != 2 {
		t.Fatalf("groups = %+v", entry.Groups)
	}
	if entry.Groups[0].Heading != "Added" || len(entry.Groups[0].Items) != 1 || entry.Groups[0].Items[0] != "fork item" {
		t.Fatalf("primary Added group mismatch: %+v", entry.Groups[0])
	}
	if entry.Groups[1].Heading != "Fixed" {
		t.Fatalf("second group heading = %q, want Fixed", entry.Groups[1].Heading)
	}
	if len(entry.Groups[1].Items) != 2 || entry.Groups[1].Items[0] != "shared item" || entry.Groups[1].Items[1] != "upstream item" {
		t.Fatalf("merged Fixed items = %+v", entry.Groups[1].Items)
	}
	if _, ok := entries["0.9.0"]; !ok {
		t.Fatal("secondary-only entry missing")
	}
}

func TestFetchChangelog_FallsBackToSecondaryWhenPrimaryFails(t *testing.T) {
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer primary.Close()

	secondary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("## [1.0.0] - 2026-01-01\n\n### Added\n- upstream only\n"))
	}))
	defer secondary.Close()

	c := newChangelogFetcher(primary.URL, secondary.URL, 10*time.Minute, nil)
	entries, err := c.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if entries["1.0.0"].Groups[0].Items[0] != "upstream only" {
		t.Fatalf("unexpected fallback entries: %+v", entries["1.0.0"])
	}
}

func TestFetchChangelog_RejectsHTML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<!DOCTYPE html><html><body>bad gateway</body></html>"))
	}))
	defer srv.Close()

	c := newChangelogFetcher(srv.URL, "", 10*time.Minute, nil)
	_, err := c.Fetch(context.Background())
	if err == nil {
		t.Fatal("expected html rejection error")
	}
	if got := err.Error(); got != "changelog source returned html instead of markdown" {
		t.Fatalf("error = %q", got)
	}
}
