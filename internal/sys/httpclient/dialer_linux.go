//go:build linux

package httpclient

import (
	"net"
	"syscall"
	"time"
)

// bindDialer returns a net.Dialer that binds to the given interface
// using SO_BINDTODEVICE.
func bindDialer(iface string, connectTimeout time.Duration) *net.Dialer {
	d := &net.Dialer{
		Timeout: connectTimeout,
	}
	if iface == "" {
		return d
	}
	// Keep the *net.Dialer default but add control hook.
	// SO_BINDTODEVICE is privileged; calling code must run as root
	// or have CAP_NET_RAW on the Keenetic router.
	ctrl := func(_, _ string, c syscall.RawConn) error {
		var setErr error
		err := c.Control(func(fd uintptr) {
			setErr = syscall.SetsockoptString(
				int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, iface,
			)
		})
		if err != nil {
			return err
		}
		return setErr
	}
	// Need to control both network flavours.
	// On modern Go Dialer.Control covers both.
	d.Control = ctrl
	return d
}
