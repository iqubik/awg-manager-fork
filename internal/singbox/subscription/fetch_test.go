package subscription

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetch_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("dmxlc3M6Ly9hYjQ0Lg==")) // base64
	}))
	defer srv.Close()

	body, ct, err := Fetch(srv.URL, nil, FetchOpts{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("ct=%q", ct)
	}
	if string(body) != "dmxlc3M6Ly9hYjQ0Lg==" {
		t.Errorf("body=%q", body)
	}
}

func TestFetch_5xxError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	defer srv.Close()
	_, _, err := Fetch(srv.URL, nil, FetchOpts{Timeout: 5 * time.Second})
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 5xx error, got %v", err)
	}
}

func TestFetch_AppliesCustomHeaders(t *testing.T) {
	var seenUA, seenCustom string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUA = r.Header.Get("User-Agent")
		seenCustom = r.Header.Get("X-Device-OS")
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	headers := []Header{
		{Name: "User-Agent", Value: "Happ/4.6.0"},
		{Name: "X-Device-OS", Value: "iOS"},
	}
	_, _, err := Fetch(srv.URL, headers, FetchOpts{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if seenUA != "Happ/4.6.0" {
		t.Errorf("UA=%q", seenUA)
	}
	if seenCustom != "iOS" {
		t.Errorf("custom=%q", seenCustom)
	}
}

func TestFetch_SkipsForbiddenHeaders(t *testing.T) {
	var seenHost string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenHost = r.Host
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	headers := []Header{
		{Name: "Host", Value: "evil.com"},
	}
	_, _, err := Fetch(srv.URL, headers, FetchOpts{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	// seenHost should be the URL's host, not "evil.com".
	if seenHost == "evil.com" {
		t.Errorf("Host header was overridden to evil.com — should be skipped")
	}
}

func TestFetch_BodyLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send 6 MiB
		large := make([]byte, 6*1024*1024)
		w.Write(large)
	}))
	defer srv.Close()

	body, _, err := Fetch(srv.URL, nil, FetchOpts{Timeout: 5 * time.Second, MaxBodyBytes: 5 * 1024 * 1024})
	if err == nil {
		t.Errorf("expected body-limit error, body len=%d", len(body))
	}
}
