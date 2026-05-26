package testing

import (
	"context"
	"fmt"
	"strings"
	"time"

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
		ConnectTimeout: 5 * time.Second,
		MaxTime:        10 * time.Second,
		DiscardBody:    true,
	})
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return 0, nil
		}
		if isTimeoutError(err) {
			return 0, nil
		}
		return -1, fmt.Errorf("ping %s via %s: %w", host, ifaceName, err)
	}

	// TimeConnect and TimeNameLookup are cumulative from request start
	// (matching curl semantics), so subtract to get pure TCP RTT.
	rtt := res.Metrics.TimeConnect - res.Metrics.TimeNameLookup
	if rtt <= 0 {
		rtt = res.Metrics.TimeConnect
	}
	ms := httpclient.SecToMs(rtt)
	if ms < 1 {
		ms = 1
	}
	return ms, nil
}

// isTimeoutError reports whether an error from the HTTP client indicates a
// timeout / unreachable host.
func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	for _, sub := range []string{"timeout", "timed out", "i/o timeout", "no route to host", "connection refused"} {
		if strings.Contains(msg, sub) {
			return true
		}
	}
	return false
}
