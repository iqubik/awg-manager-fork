package downloader

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadAllSendsBodyAndReturnsHeaders(t *testing.T) {
	var gotMethod string
	var gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		gotBody = string(body)
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("X-Test-Token", "secret")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	svc := NewService(Deps{})
	body, meta, err := svc.ReadAll(context.Background(), Request{
		Purpose:       "test-post",
		URL:           server.URL,
		Method:        http.MethodPost,
		Body:          []byte("payload"),
		MaxBodyBytes:  128,
		AllowedStatus: []int{http.StatusCreated},
	})
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(body) != "ok" {
		t.Fatalf("body = %q, want ok", string(body))
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("method = %q, want POST", gotMethod)
	}
	if gotBody != "payload" {
		t.Fatalf("request body = %q, want payload", gotBody)
	}
	if meta.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", meta.StatusCode, http.StatusCreated)
	}
	if meta.Headers.Get("X-Test-Token") != "secret" {
		t.Fatalf("response header X-Test-Token = %q, want secret", meta.Headers.Get("X-Test-Token"))
	}
}
