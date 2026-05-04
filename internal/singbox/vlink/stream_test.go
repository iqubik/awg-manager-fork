package vlink

import (
	"net/url"
	"testing"
)

func parseQuery(t *testing.T, s string) url.Values {
	t.Helper()
	q, err := url.ParseQuery(s)
	if err != nil {
		t.Fatalf("parseQuery: %v", err)
	}
	return q
}

func TestBuildStreamFromQuery_TCPNoTLS(t *testing.T) {
	q := parseQuery(t, "type=tcp&security=none")
	s, err := BuildStreamFromQuery(q, "example.com")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.Network != "tcp" {
		t.Errorf("network=%q, want tcp", s.Network)
	}
	if s.TLS != nil {
		t.Errorf("expected nil TLS, got %+v", s.TLS)
	}
}

func TestBuildStreamFromQuery_WSWithTLS(t *testing.T) {
	q := parseQuery(t, "type=ws&security=tls&path=/abc%3Fed%3D2048&host=cdn.example.com&sni=foo.com&alpn=h2,http/1.1&fp=chrome")
	s, err := BuildStreamFromQuery(q, "example.com")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.Network != "ws" {
		t.Errorf("network=%q, want ws", s.Network)
	}
	if s.Path != "/abc" {
		t.Errorf("path=%q, want /abc (ed= stripped)", s.Path)
	}
	if s.EarlyData != 2048 {
		t.Errorf("earlyData=%d, want 2048", s.EarlyData)
	}
	if s.Host != "cdn.example.com" {
		t.Errorf("host=%q", s.Host)
	}
	if s.TLS == nil {
		t.Fatalf("expected TLS")
	}
	if s.TLS.ServerName != "foo.com" {
		t.Errorf("sni=%q", s.TLS.ServerName)
	}
	if len(s.TLS.ALPN) != 2 || s.TLS.ALPN[0] != "h2" || s.TLS.ALPN[1] != "http/1.1" {
		t.Errorf("alpn=%v", s.TLS.ALPN)
	}
	if s.TLS.UTLSFingerprint != "chrome" {
		t.Errorf("fp=%q", s.TLS.UTLSFingerprint)
	}
}

func TestBuildStreamFromQuery_GRPC(t *testing.T) {
	q := parseQuery(t, "type=grpc&security=tls&serviceName=mysvc")
	s, err := BuildStreamFromQuery(q, "h")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.Network != "grpc" {
		t.Errorf("network=%q", s.Network)
	}
	if s.ServiceName != "mysvc" {
		t.Errorf("service=%q", s.ServiceName)
	}
}

func TestBuildStreamFromQuery_H2_AliasedToHTTP(t *testing.T) {
	q := parseQuery(t, "type=h2&security=tls&path=/api&host=h.example.com")
	s, err := BuildStreamFromQuery(q, "h")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.Network != "http" {
		t.Errorf("network=%q, want http (h2 alias)", s.Network)
	}
	if s.Path != "/api" {
		t.Errorf("path=%q", s.Path)
	}
}

func TestBuildStreamFromQuery_ModeGunOverridesType(t *testing.T) {
	q := parseQuery(t, "type=tcp&mode=gun&serviceName=g")
	s, err := BuildStreamFromQuery(q, "h")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.Network != "grpc" {
		t.Errorf("network=%q, want grpc (mode=gun override)", s.Network)
	}
}

func TestBuildStreamFromQuery_Reality(t *testing.T) {
	q := parseQuery(t, "type=tcp&security=reality&pbk=PUBLIC_KEY&sid=abcdef,fffeee&fp=firefox&sni=example.com")
	s, err := BuildStreamFromQuery(q, "h")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if s.TLS == nil || s.TLS.Reality == nil {
		t.Fatalf("expected reality config")
	}
	if s.TLS.Reality.PublicKey != "PUBLIC_KEY" {
		t.Errorf("pbk=%q", s.TLS.Reality.PublicKey)
	}
	if s.TLS.Reality.ShortID != "abcdef" {
		t.Errorf("sid=%q (must be first comma segment)", s.TLS.Reality.ShortID)
	}
}

func TestBuildStreamFromQuery_RealitySidTooLong_Rejected(t *testing.T) {
	// 17 hex chars — over the 16 limit
	q := parseQuery(t, "type=tcp&security=reality&pbk=K&sid=00000000000000001")
	_, err := BuildStreamFromQuery(q, "h")
	if err == nil {
		t.Errorf("expected error on sid > 16 hex chars")
	}
}
