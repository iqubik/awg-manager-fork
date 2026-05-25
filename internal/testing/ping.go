package testing

import (
	"context"
	"fmt"

	"github.com/hoaxisr/awg-manager/internal/sys/httpclient"
)

// PingByIface measures TCP connect time (in milliseconds) to `host:port` through
// the specified kernel interface. Uses a Go-native HTTP client with SO_BINDTODEVICE
// instead of curl for zero external dependencies.
//
// Returns (-1, err) on execution failure, (0, nil) on timeout (configurable via ctx).
func (s *Service) PingByIface(ctx context.Context, ifaceName, host string, port int) (int, error) {
	target := fmt.Sprintf("http://%s:%d/", host, port)

	res, err := httpclient.DefaultClient.Do(ctx, httpclient.CallConfig{
		URL:            target,
		Interface:      ifaceName,
		ConnectTimeout: 5,
		MaxTime:        10,
		DiscardBody:    true,
	})
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return 0, nil
		}
		// Mimic curl exit-code-28 "timed out" → 0ms (unreachable).
		if isTimeoutError(err) {
			return 0, nil
		}
		return -1, fmt.Errorf("ping %s via %s: %w", host, ifaceName, err)
	}

	ms := httpclient.SecToMs(res.Metrics.TimeConnect)
	if ms < 1 {
		ms = 1
	}
	return ms, nil
}

// isTimeoutError reports whether an error from the HTTP client indicates a
// timeout / unreachable host (curl exit code 28 equivalent).
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return containsAny(msg, "timeout", "timed out", "i/o timeout", "no route to host", "connection refused")
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if contains(s, sub) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) > 0 && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
