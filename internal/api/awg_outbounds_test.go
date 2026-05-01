// internal/api/awg_outbounds_test.go
package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/singbox/awgoutbounds"
)

type mockAWGSvc struct {
	tags []awgoutbounds.TagInfo
	err  error
}

func (m *mockAWGSvc) ListTags(ctx context.Context) ([]awgoutbounds.TagInfo, error) {
	return m.tags, m.err
}

func TestAWGOutboundsTags_Success(t *testing.T) {
	svc := &mockAWGSvc{tags: []awgoutbounds.TagInfo{
		{Tag: "awg-x", Label: "X", Kind: "managed", Iface: "t2s0"},
	}}
	h := NewAWGOutboundsHandler(svc)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/singbox/awg-outbounds/tags", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	var env struct {
		Success bool             `json:"success"`
		Data    []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &env); err != nil {
		t.Fatalf("body: %v", err)
	}
	if !env.Success {
		t.Errorf("envelope success should be true: %s", rr.Body.String())
	}
	if len(env.Data) != 1 || env.Data[0]["tag"] != "awg-x" {
		t.Errorf("body wrong: %v", env.Data)
	}
}

func TestAWGOutboundsTags_Empty(t *testing.T) {
	svc := &mockAWGSvc{tags: nil}
	h := NewAWGOutboundsHandler(svc)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/singbox/awg-outbounds/tags", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("want 200, got %d", rr.Code)
	}
	body := strings.TrimSpace(rr.Body.String())
	// Empty result still uses the envelope shape so the frontend
	// request<T>() helper resolves data.data to an empty array.
	if body != `{"success":true,"data":[]}` {
		t.Errorf("expected envelope with empty array, got %q", body)
	}
}

func TestAWGOutboundsTags_MethodNotAllowed(t *testing.T) {
	svc := &mockAWGSvc{}
	h := NewAWGOutboundsHandler(svc)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/singbox/awg-outbounds/tags", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != 405 {
		t.Errorf("want 405, got %d", rr.Code)
	}
}
