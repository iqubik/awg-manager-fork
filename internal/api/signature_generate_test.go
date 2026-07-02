package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSignatureGenerate_ValidProtocol(t *testing.T) {
	body := []byte(`{"protocol":"quic_initial","mtu":1280}`)
	req := httptest.NewRequest(http.MethodPost, "/api/signature/generate", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	NewSignatureHandler().Generate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var resp SignatureGenerateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success || !resp.Data.OK {
		t.Fatalf("response = %+v, want success=true and ok=true", resp)
	}
	if resp.Data.Protocol != "quic_initial" {
		t.Fatalf("protocol = %q, want quic_initial", resp.Data.Protocol)
	}
	if resp.Data.Packets.I1 == "" {
		t.Fatal("I1 is empty")
	}
}

func TestSignatureGenerate_TLSAlias(t *testing.T) {
	body := []byte(`{"protocol":"tls"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/signature/generate", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	NewSignatureHandler().Generate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var resp SignatureGenerateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Data.Protocol != "tls_client_hello" {
		t.Fatalf("protocol = %q, want tls_client_hello", resp.Data.Protocol)
	}
}

func TestSignatureGenerate_InvalidProtocol(t *testing.T) {
	body := []byte(`{"protocol":"unknown"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/signature/generate", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	NewSignatureHandler().Generate(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusBadRequest, rr.Body.String())
	}
	if !bytes.Contains(rr.Body.Bytes(), []byte("UNKNOWN_PROTOCOL")) {
		t.Fatalf("body %q does not contain UNKNOWN_PROTOCOL", rr.Body.String())
	}
}

func TestSignatureGenerate_ZeroMTUFallsBackToDefault(t *testing.T) {
	body := []byte(`{"protocol":"http3","mtu":0}`)
	req := httptest.NewRequest(http.MethodPost, "/api/signature/generate", bytes.NewReader(body))
	rr := httptest.NewRecorder()

	NewSignatureHandler().Generate(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	var resp SignatureGenerateResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success || !resp.Data.OK || resp.Data.Packets.I1 == "" {
		t.Fatalf("response = %+v, want success=true and non-empty packets", resp)
	}
}
