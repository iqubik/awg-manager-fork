package httpclient

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMSSFromMTU(t *testing.T) {
	cases := []struct {
		mtu  int
		want int
	}{
		{mtu: 1280, want: 1240}, // AmneziaWG / WireGuard tunnel
		{mtu: 1420, want: 1380}, // common WG default
		{mtu: 1412, want: 1372},
		{mtu: 1500, want: 0}, // full Ethernet link — no clamp needed
		{mtu: 1600, want: 0}, // jumbo / above standard — no clamp
		{mtu: 40, want: 0},   // degenerate
		{mtu: 0, want: 0},    // unknown
		{mtu: -1, want: 0},   // invalid
	}
	for _, tc := range cases {
		if got := mssFromMTU(tc.mtu); got != tc.want {
			t.Errorf("mssFromMTU(%d) = %d, want %d", tc.mtu, got, tc.want)
		}
	}
}

func TestReadIfaceMTU(t *testing.T) {
	dir := t.TempDir()
	ifaceDir := filepath.Join(dir, "nwg2")
	if err := os.MkdirAll(ifaceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ifaceDir, "mtu"), []byte("1280\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	orig := sysClassNet
	sysClassNet = dir
	t.Cleanup(func() { sysClassNet = orig })

	if got := readIfaceMTU("nwg2"); got != 1280 {
		t.Fatalf("readIfaceMTU(nwg2) = %d, want 1280", got)
	}
	if got := readIfaceMTU("does-not-exist"); got != 0 {
		t.Fatalf("readIfaceMTU(missing) = %d, want 0", got)
	}
	if got := readIfaceMTU(""); got != 0 {
		t.Fatalf("readIfaceMTU(empty) = %d, want 0", got)
	}
	if got := tunnelMSS("nwg2"); got != 1240 {
		t.Fatalf("tunnelMSS(nwg2) = %d, want 1240", got)
	}
}
