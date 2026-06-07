package query

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestDNSProxyStatusStoreListFetchesRaw(t *testing.T) {
	fg := newFakeGetter()
	fg.SetRaw("/show/dns-proxy", []byte("status bytes"))
	s := NewDNSProxyStatusStore(fg, NopLogger())

	got, err := s.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if string(got) != "status bytes" {
		t.Fatalf("List() = %q, want %q", string(got), "status bytes")
	}
	if fg.Calls("/show/dns-proxy") != 1 {
		t.Fatalf("Calls(/show/dns-proxy) = %d, want 1", fg.Calls("/show/dns-proxy"))
	}
}

func TestDNSProxyStatusStoreGetterErrorWrapped(t *testing.T) {
	fg := newFakeGetter()
	fg.SetError("/show/dns-proxy", errors.New("boom"))
	s := NewDNSProxyStatusStore(fg, NopLogger())

	_, err := s.List(context.Background())
	if err == nil {
		t.Fatal("List() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "fetch dns-proxy status") {
		t.Fatalf("error = %q, want wrapped fetch dns-proxy status", err)
	}
}

