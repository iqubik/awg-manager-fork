//go:build linux

package httpclient

import (
	"net"
	"strings"
	"syscall"
	"time"
)

// bindDialer returns a net.Dialer that binds outgoing sockets to the given
// interface using SO_BINDTODEVICE, and clamps the advertised TCP MSS to the
// interface MTU so large inbound segments fit inside tunnel devices.
func bindDialer(iface string, connectTimeout time.Duration) *net.Dialer {
	d := &net.Dialer{
		Timeout: connectTimeout,
	}
	iface = strings.TrimSpace(iface)
	if iface == "" {
		return d
	}

	// Resolve the MSS clamp once per dialer. 0 means "leave the kernel
	// default" (unknown MTU or a full-size 1500-byte link).
	mss := tunnelMSS(iface)

	// SO_BINDTODEVICE is privileged; calling code must run as root or have
	// CAP_NET_RAW on the Keenetic router. Dialer.Control covers both the TCP
	// and UDP (tunnel-DNS) socket flavours.
	d.Control = func(network, _ string, c syscall.RawConn) error {
		var setErr error
		ctrlErr := c.Control(func(fd uintptr) {
			if err := syscall.SetsockoptString(
				int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, iface,
			); err != nil {
				setErr = err
				return
			}
			// Clamp the advertised MSS for TCP only. The route the kernel
			// picks for MSS is the WAN (1500), but SO_BINDTODEVICE forces
			// egress through a smaller-MTU tunnel — without this the remote
			// would send oversized segments that the tunnel black-holes,
			// stalling TLS and surfacing as "EOF". Best-effort: the kernel
			// enforces its own floor and ignores invalid values.
			if mss > 0 && strings.HasPrefix(network, "tcp") {
				_ = syscall.SetsockoptInt(
					int(fd), syscall.IPPROTO_TCP, syscall.TCP_MAXSEG, mss,
				)
			}
		})
		if ctrlErr != nil {
			return ctrlErr
		}
		return setErr
	}
	return d
}
