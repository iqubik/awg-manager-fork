package vlink

import (
	"net/url"
	"reflect"
	"testing"
)

func TestIsClashYAML(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"proxies header", "proxies:\n  - name: a\n    type: vless\n", true},
		{"proxies header with leading whitespace lines", "\n# comment\nproxies:\n  - name: a\n", true},
		{"proxies inline empty", "proxies: []\n", true},
		{"proxies null", "proxies: null\n", true},
		{"proxies followed by document body", "---\nproxies:\n  - name: a\n", true},
		{"share-link vless", "vless://uuid@host:443?security=tls\n", false},
		{"html", "<!DOCTYPE html><html></html>", false},
		{"base64", "dmxlc3M6Ly91dWlkQGhvc3Q6NDQz", false},
		{"empty", "", false},
		{"plain trojan link", "trojan://pass@host:443?sni=h", false},
		{"clash-like but no proxies", "rules:\n  - DOMAIN,a.com,DIRECT\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsClashYAML([]byte(tc.body)); got != tc.want {
				t.Errorf("IsClashYAML(%q) = %v, want %v", tc.body, got, tc.want)
			}
		})
	}
}

func TestClashFieldsToValues_TLSWS(t *testing.T) {
	in := map[string]any{
		"name":               "x",
		"server":             "example.com",
		"port":               443,
		"tls":                true,
		"servername":         "sni.example.com",
		"skip-cert-verify":   true,
		"alpn":               []any{"h2", "http/1.1"},
		"client-fingerprint": "chrome",
		"network":            "ws",
		"ws-opts": map[string]any{
			"path": "/xyz",
			"headers": map[string]any{
				"Host": "host.example.com",
			},
		},
	}
	got := clashFieldsToValues(in)
	want := url.Values{
		"security": {"tls"},
		"sni":      {"sni.example.com"},
		"insecure": {"1"},
		"alpn":     {"h2,http/1.1"},
		"fp":       {"chrome"},
		"type":     {"ws"},
		"path":     {"/xyz"},
		"host":     {"host.example.com"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("clashFieldsToValues mismatch\n got=%v\nwant=%v", got, want)
	}
}

func TestClashFieldsToValues_RealityGRPC(t *testing.T) {
	in := map[string]any{
		"server":     "h",
		"port":       443,
		"tls":        true,
		"servername": "sni",
		"network":    "grpc",
		"grpc-opts": map[string]any{
			"grpc-service-name": "GunService",
		},
		"reality-opts": map[string]any{
			"public-key": "xxxxx",
			"short-id":   "abcd",
		},
		"client-fingerprint": "chrome",
	}
	got := clashFieldsToValues(in)
	if got.Get("security") != "reality" {
		t.Errorf("security want reality, got %q", got.Get("security"))
	}
	if got.Get("type") != "grpc" {
		t.Errorf("type want grpc, got %q", got.Get("type"))
	}
	if got.Get("serviceName") != "GunService" {
		t.Errorf("serviceName want GunService, got %q", got.Get("serviceName"))
	}
	if got.Get("pbk") != "xxxxx" {
		t.Errorf("pbk want xxxxx, got %q", got.Get("pbk"))
	}
	if got.Get("sid") != "abcd" {
		t.Errorf("sid want abcd, got %q", got.Get("sid"))
	}
	if got.Get("fp") != "chrome" {
		t.Errorf("fp want chrome, got %q", got.Get("fp"))
	}
}

func TestAsInt(t *testing.T) {
	cases := []struct {
		in   any
		want int
		ok   bool
	}{
		{443, 443, true},
		{int64(443), 443, true},
		{float64(443), 443, true},
		{"443", 443, true},
		{"  443 ", 443, true},
		{"abc", 0, false},
		{nil, 0, false},
		{true, 0, false},
	}
	for _, tc := range cases {
		got, ok := asInt(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("asInt(%v) = (%d,%v), want (%d,%v)", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestAsBool(t *testing.T) {
	cases := []struct {
		in   any
		want bool
	}{
		{true, true},
		{false, false},
		{1, true},
		{0, false},
		{"true", true},
		{"1", true},
		{"yes", true},
		{"false", false},
		{nil, false},
	}
	for _, tc := range cases {
		if got := asBool(tc.in); got != tc.want {
			t.Errorf("asBool(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestAsStringSlice(t *testing.T) {
	cases := []struct {
		in   any
		want []string
	}{
		{[]any{"a", "b"}, []string{"a", "b"}},
		{[]string{"a", "b"}, []string{"a", "b"}},
		{"single", []string{"single"}},
		{nil, nil},
		{42, nil},
	}
	for _, tc := range cases {
		got := asStringSlice(tc.in)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("asStringSlice(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
