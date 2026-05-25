//go:build !linux

package httpclient

import (
	"net"
	"time"
)

// bindDialer is a stub for non-Linux builds (dev/test on macOS, Windows).
// SO_BINDTODEVICE is Linux-only; interface binding is silently skipped.
func bindDialer(_ string, connectTimeout time.Duration) *net.Dialer {
	return &net.Dialer{Timeout: connectTimeout}
}
