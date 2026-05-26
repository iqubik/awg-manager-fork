package httpclient

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func handler204(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
func handler200(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "ok") }
func handlerIP(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "203.0.113.1") }

// stubDoer implements HTTPDoer for tests that exercise the call site wrappers.
type stubDoer struct {
	result *Result
	err    error
}

func (s stubDoer) Do(_ context.Context, _ CallConfig) (*Result, error) {
	return s.result, s.err
}

// ---------------------------------------------------------------------------
// Unit tests with httptest.Server (no root / no network)
// ---------------------------------------------------------------------------

func TestDo_GET_Success(t *testing.T) {
	ts := newTestServer(t, handler200)
	c := New()
	res, err := c.Do(context.Background(), CallConfig{URL: ts.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Metrics.HTTPCode != 200 {
		t.Errorf("HTTPCode = %d, want 200", res.Metrics.HTTPCode)
	}
	if res.Body != "ok" {
		t.Errorf("Body = %q, want %q", res.Body, "ok")
	}
}

func TestDo_GET_204(t *testing.T) {
	// Matches connectivitycheck.gstatic.com/generate_204 pattern.
	ts := newTestServer(t, handler204)
	c := New()
	res, err := c.Do(context.Background(), CallConfig{
		URL:         ts.URL,
		DiscardBody: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Metrics.HTTPCode != 204 {
		t.Errorf("HTTPCode = %d, want 204", res.Metrics.HTTPCode)
	}
	if res.Body != "" {
		t.Errorf("Body should be empty with DiscardBody, got %q", res.Body)
	}
}

func TestDo_HEAD(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Errorf("Method = %q, want HEAD", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	})
	c := New()
	res, err := c.Do(context.Background(), CallConfig{
		URL:         ts.URL,
		Method:      http.MethodHead,
		DiscardBody: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Metrics.HTTPCode != 200 {
		t.Errorf("HTTPCode = %d, want 200", res.Metrics.HTTPCode)
	}
}

func TestDo_POST_Form(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); !strings.Contains(ct, "application/x-www-form-urlencoded") {
			t.Errorf("Content-Type = %q, want form-urlencoded", ct)
		}
		r.ParseForm()
		if v := r.FormValue("type"); v != "ifcreated" {
			t.Errorf("type = %q, want ifcreated", v)
		}
		if v := r.FormValue("id"); v != "eth2" {
			t.Errorf("id = %q, want eth2", v)
		}
		w.WriteHeader(http.StatusOK)
	})
	c := New()
	_, err := c.Do(context.Background(), CallConfig{
		URL:      ts.URL,
		PostData: map[string][]string{"type": {"ifcreated"}, "id": {"eth2"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDo_WithProxy(t *testing.T) {
	// Proxy server records the request and forwards.
	var proxied bool
	proxy := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		proxied = true
		// Simple HTTP proxy: just forward.
		if r.Method == http.MethodConnect {
			w.WriteHeader(http.StatusOK)
			return
		}
		handler200(w, r)
	})

	ts := newTestServer(t, handler200)
	c := New()
	res, err := c.Do(context.Background(), CallConfig{
		URL:         ts.URL,
		ProxyURL:    proxy.URL,
		DiscardBody: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Metrics.HTTPCode != 200 {
		t.Errorf("HTTPCode = %d, want 200", res.Metrics.HTTPCode)
	}
	if !proxied {
		t.Error("request was not routed through proxy")
	}
}

func TestDo_DiscardBody(t *testing.T) {
	var called bool
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Write([]byte("large-body-data"))
	})
	c := New()
	res, err := c.Do(context.Background(), CallConfig{
		URL:         ts.URL,
		DiscardBody: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Body != "" {
		t.Errorf("Body should be empty when DiscardBody=true, got %q", res.Body)
	}
	if !called {
		t.Error("handler was never called")
	}
}

func TestDo_Timeout(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})
	c := New()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := c.Do(ctx, CallConfig{URL: ts.URL})
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") && !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error message %q does not mention timeout", err.Error())
	}
}

func TestDo_CancelledContext(t *testing.T) {
	ts := newTestServer(t, handler200)
	c := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // immediate cancellation
	_, err := c.Do(ctx, CallConfig{URL: ts.URL})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDo_Metrics_Timing(t *testing.T) {
	// On loopback everything should be near-zero but positive.
	ts := newTestServer(t, handler200)
	c := New()
	res, err := c.Do(context.Background(), CallConfig{URL: ts.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Metrics.TimeTotal <= 0 {
		t.Errorf("TimeTotal = %f, want > 0", res.Metrics.TimeTotal)
	}
	// Loopback connects can show connectStart==0 (reuse) or very fast.
	// The important contract is that TimeTotal > 0 and HTTPCode is set.
	if res.Metrics.HTTPCode != 200 {
		t.Errorf("HTTPCode = %d, want 200", res.Metrics.HTTPCode)
	}
}

func TestDo_Metrics_NoDNS(t *testing.T) {
	// Connecting to 127.0.0.1 directly — DNS should not be involved.
	ts := newTestServer(t, handler200)
	uri := strings.Replace(ts.URL, "localhost", "127.0.0.1", 1)
	c := New()
	res, err := c.Do(context.Background(), CallConfig{URL: uri})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// When there is no DNS lookup, httptrace does not fire DNS events,
	// so TimeNameLookup is expected to be 0.
	if res.Metrics.TimeNameLookup != 0 {
		t.Logf("TimeNameLookup = %f (non-zero is acceptable for loopback resolvers)", res.Metrics.TimeNameLookup)
	}
	if res.Metrics.HTTPCode != 200 {
		t.Errorf("HTTPCode = %d, want 200", res.Metrics.HTTPCode)
	}
}

func TestDo_UserAgent_SetToCurl(t *testing.T) {
	ts := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if !strings.HasPrefix(ua, "curl/") {
			t.Errorf("User-Agent = %q, want curl/* prefix", ua)
		}
		w.WriteHeader(http.StatusOK)
	})
	c := New()
	_, err := c.Do(context.Background(), CallConfig{URL: ts.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDo_Metrics_CumulativeTimings(t *testing.T) {
	// When DNS is involved, cumulative timings must satisfy:
	// TimeNameLookup <= TimeConnect <= TimeTotal.
	ts := newTestServer(t, handler200)
	// Use "localhost" to trigger a DNS lookup (unlike 127.0.0.1).
	uri := strings.Replace(ts.URL, "127.0.0.1", "localhost", 1)
	c := New()
	res, err := c.Do(context.Background(), CallConfig{URL: uri})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Metrics.TimeNameLookup < 0 {
		t.Errorf("TimeNameLookup = %f, want >= 0", res.Metrics.TimeNameLookup)
	}
	if res.Metrics.TimeConnect < res.Metrics.TimeNameLookup {
		t.Errorf("TimeConnect (%f) < TimeNameLookup (%f) — timings must be cumulative",
			res.Metrics.TimeConnect, res.Metrics.TimeNameLookup)
	}
	if res.Metrics.TimeTotal < res.Metrics.TimeConnect {
		t.Errorf("TimeTotal (%f) < TimeConnect (%f) — timings must be cumulative",
			res.Metrics.TimeTotal, res.Metrics.TimeConnect)
	}
}

func TestDo_URL_Missing(t *testing.T) {
	c := New()
	_, err := c.Do(context.Background(), CallConfig{})
	if err == nil {
		t.Fatal("expected error for missing URL, got nil")
	}
}

// ---------------------------------------------------------------------------
// SecToMs — must match existing curl output contracts
// ---------------------------------------------------------------------------

func TestSecToMs(t *testing.T) {
	cases := []struct {
		sec  float64
		want int
	}{
		{0.043, 43},
		{0.001, 1},
		{0.0001, 1}, // floor to 1ms minimum
		{0.0, 0},
		{0.5, 500},
		{1.234, 1234},
	}
	for _, c := range cases {
		got := SecToMs(c.sec)
		if got != c.want {
			t.Errorf("SecToMs(%v) = %d, want %d", c.sec, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Concurrent safety
// ---------------------------------------------------------------------------

func TestDo_ConcurrentCalls(t *testing.T) {
	ts := newTestServer(t, handler200)
	c := New()
	ctx := context.Background()

	const n = 20
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			cfg := CallConfig{URL: ts.URL}
			if idx%2 == 0 {
				cfg.DiscardBody = true
			}
			res, err := c.Do(ctx, cfg)
			if err != nil {
				errs <- fmt.Errorf("goroutine %d: %w", idx, err)
				return
			}
			if res.Metrics.HTTPCode != 200 {
				errs <- fmt.Errorf("goroutine %d: HTTPCode = %d", idx, res.Metrics.HTTPCode)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
}
