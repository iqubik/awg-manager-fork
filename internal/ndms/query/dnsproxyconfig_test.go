package query

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDNSProxyConfigStoreHasEncryptedTransport(t *testing.T) {
	path := "/show/rc/dns-proxy"

	tests := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "dot", raw: "server dot.example", want: true},
		{name: "tls uppercase", raw: "SERVER TLS", want: true},
		{name: "doh", raw: "dns-over-doh configured", want: true},
		{name: "https", raw: "https://dns.example/dns-query", want: true},
		{name: "plain", raw: "plain udp dns", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fg := newFakeGetter()
			fg.SetRaw(path, []byte(tc.raw))
			s := NewDNSProxyConfigStore(fg, NopLogger())
			got, err := s.HasEncryptedTransport(context.Background())
			if err != nil {
				t.Fatalf("HasEncryptedTransport() error = %v", err)
			}
			if got != tc.want {
				t.Fatalf("HasEncryptedTransport() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDNSProxyConfigStoreGetterErrorWrapped(t *testing.T) {
	fg := newFakeGetter()
	fg.SetError("/show/rc/dns-proxy", errors.New("boom"))
	s := NewDNSProxyConfigStore(fg, NopLogger())

	_, err := s.HasEncryptedTransport(context.Background())
	if err == nil {
		t.Fatal("HasEncryptedTransport() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "fetch dns-proxy config") {
		t.Fatalf("error = %q, want wrapped fetch dns-proxy config", err)
	}
}

