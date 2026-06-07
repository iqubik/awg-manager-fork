package httpclient

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ipv4TCPHeaderOverhead is the combined IPv4 (20) + TCP (20) header size that
// is subtracted from a link MTU to derive the largest TCP payload (MSS).
const ipv4TCPHeaderOverhead = 40

// sysClassNet is the kernel directory exposing per-interface attributes.
// Declared as a var so tests can point it at a fixture directory.
var sysClassNet = "/sys/class/net" //nolint:gochecknoglobals

// tunnelMSS returns the TCP MSS to advertise for a socket bound to iface, or
// 0 when no clamping is needed.
//
// SO_BINDTODEVICE forces egress through a tunnel device, but the kernel still
// derives the advertised MSS from the matching *route* — typically the
// 1500-byte WAN — not from the bound device. A WireGuard / AmneziaWG interface
// has a smaller MTU (e.g. 1280), so without clamping the remote peer sends
// segments too large for the tunnel. Those oversized inbound packets are
// silently dropped, stalling the TLS handshake and surfacing to callers as a
// bare "EOF". Clamping the MSS to the tunnel MTU keeps every inbound segment
// inside the tunnel.
func tunnelMSS(iface string) int {
	return mssFromMTU(readIfaceMTU(iface))
}

// mssFromMTU converts a link MTU into the MSS to advertise, or 0 when the MTU
// is unknown or already at/above a standard 1500-byte link (where the
// route-derived MSS already fits and no clamp is required).
func mssFromMTU(mtu int) int {
	if mtu <= ipv4TCPHeaderOverhead || mtu >= 1500 {
		return 0
	}
	return mtu - ipv4TCPHeaderOverhead
}

// readIfaceMTU reads /sys/class/net/<iface>/mtu. Returns 0 on any error so
// callers fall back to the kernel default (no clamp).
func readIfaceMTU(iface string) int {
	iface = strings.TrimSpace(iface)
	if iface == "" {
		return 0
	}
	data, err := os.ReadFile(filepath.Join(sysClassNet, iface, "mtu"))
	if err != nil {
		return 0
	}
	mtu, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return mtu
}
