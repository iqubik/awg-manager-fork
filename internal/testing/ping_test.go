package testing

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/sys/httpclient"
)

// TestPingByIface_Success verifies latency computation from a real httptest server.
func TestPingByIface_Success(t *testing.T) {
	s := NewService(nil, nil)

	tsrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer tsrv.Close()

	// Extract host:port from URL like http://127.0.0.1:12345/
	var host string
	var port int
	fmt.Sscanf(tsrv.URL, "http://%s:%d", &host, &port)
	if host == "" {
		// Fallback: strip http:// prefix and trailing /
		hostPort := tsrv.URL[len("http://"):]
		if idx := len(hostPort) - 1; idx >= 0 && hostPort[idx] == '/' {
			hostPort = hostPort[:idx]
		}
		// For IPv4 loopback the host isn't quoted, so Sscanf may have succeeded.
		if host == "" {
			// Use net.SplitHostPort
			h, p, _ := net.SplitHostPort(hostPort)
			host = h
			fmt.Sscanf(p, "%d", &port)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// We can't bind to an interface in tests on non-Linux, but the request
	// should succeed via loopback even without SO_BINDTODEVICE.
	ms, err := s.PingByIface(ctx, "", host, port)
	if err != nil {
		t.Fatalf("PingByIface error: %v", err)
	}
	if ms < 0 {
		t.Errorf("latency = %d, want >= 0", ms)
	}
}

// stubDoer implements HTTPDoer for call-site tests.
type stubDoer struct {
	result *httpclient.Result
	err    error
}

func (s stubDoer) Do(_ context.Context, _ httpclient.CallConfig) (*httpclient.Result, error) {
	return s.result, s.err
}
