package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type parsePayload struct {
	Name string `json:"name"`
}

func TestParseJSON_Valid(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"ok"}`))
	rr := httptest.NewRecorder()

	got, ok := parseJSON[parsePayload](rr, req, http.MethodPost)
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if got.Name != "ok" {
		t.Fatalf("Name = %q, want %q", got.Name, "ok")
	}
}

func TestParseJSON_WrongMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(`{"name":"ok"}`))
	rr := httptest.NewRecorder()

	_, ok := parseJSON[parsePayload](rr, req, http.MethodPost)
	if ok {
		t.Fatal("ok = true, want false")
	}
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestParseJSON_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))
	rr := httptest.NewRecorder()

	_, ok := parseJSON[parsePayload](rr, req, http.MethodPost)
	if ok {
		t.Fatal("ok = true, want false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rr.Body.String(), "INVALID_JSON") {
		t.Fatalf("body %q does not contain INVALID_JSON", rr.Body.String())
	}
}

func TestParseJSON_WhitespaceBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("   \n\t "))
	rr := httptest.NewRecorder()

	_, ok := parseJSON[parsePayload](rr, req, http.MethodPost)
	if ok {
		t.Fatal("ok = true, want false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rr.Body.String(), "INVALID_JSON") {
		t.Fatalf("body %q does not contain INVALID_JSON", rr.Body.String())
	}
}

func TestParseJSON_BOM(t *testing.T) {
	body := string([]byte{0xEF, 0xBB, 0xBF}) + `{"name":"bom"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()

	got, ok := parseJSON[parsePayload](rr, req, http.MethodPost)
	if !ok {
		t.Fatal("ok = false, want true")
	}
	if got.Name != "bom" {
		t.Fatalf("Name = %q, want %q", got.Name, "bom")
	}
}

func TestParseJSON_Malformed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":`))
	rr := httptest.NewRecorder()

	_, ok := parseJSON[parsePayload](rr, req, http.MethodPost)
	if ok {
		t.Fatal("ok = true, want false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rr.Body.String(), "INVALID_JSON") {
		t.Fatalf("body %q does not contain INVALID_JSON", rr.Body.String())
	}
}

func TestParseJSON_OversizedBody(t *testing.T) {
	body := strings.Repeat("x", maxBodySize+1)
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	rr := httptest.NewRecorder()

	_, ok := parseJSON[parsePayload](rr, req, http.MethodPost)
	if ok {
		t.Fatal("ok = true, want false")
	}
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rr.Body.String(), "INVALID_BODY") {
		t.Fatalf("body %q does not contain INVALID_BODY", rr.Body.String())
	}
}
