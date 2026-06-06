package pingcheck

import (
	"context"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/tunnel/wg"
)

// slowWGClient simulates a WG client that never returns a recent handshake.
type slowWGClient struct{}

func (c *slowWGClient) Show(ctx context.Context, iface string) (*wg.ShowResult, error) {
	return &wg.ShowResult{HasPeer: true}, nil // no recent handshake
}

// TestWaitHandshake_InterruptedByStopCh verifies that waitHandshake exits
// immediately when stopCh is closed, rather than blocking for the full
// 30-second handshake timeout. This prevents HTTP handlers from hanging
// when StopMonitoring is called during an active link toggle.
func TestWaitHandshake_InterruptedByStopCh(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := &Service{
		wg:  &slowWGClient{},
		ctx: ctx,
	}

	stopCh := make(chan struct{})

	// Close stopCh after a short delay to simulate StopMonitoring.
	go func() {
		time.Sleep(50 * time.Millisecond)
		close(stopCh)
	}()

	start := time.Now()
	result := s.waitHandshake("fake0", stopCh)
	elapsed := time.Since(start)

	if result {
		t.Error("waitHandshake should return false when interrupted by stopCh")
	}
	if elapsed > 2*time.Second {
		t.Errorf("waitHandshake took %v — should have exited quickly via stopCh, not waited for 30s deadline", elapsed)
	}
}

// TestWaitHandshake_DeadlineWithoutStop verifies that waitHandshake respects
// the configured handshake deadline when stopCh is NOT closed (normal path).
func TestWaitHandshake_DeadlineWithoutStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const fastTimeout = 40 * time.Millisecond
	s := &Service{
		wg:               &slowWGClient{},
		ctx:              ctx,
		handshakeTimeout: fastTimeout,
	}

	stopCh := make(chan struct{}) // never closed

	start := time.Now()
	result := s.waitHandshake("fake0", stopCh)
	elapsed := time.Since(start)

	if result {
		t.Error("waitHandshake should return false when deadline expires")
	}
	if elapsed < fastTimeout/2 {
		t.Errorf("waitHandshake returned too quickly (%v) — deadline should be about %v", elapsed, fastTimeout)
	}
	if elapsed > time.Second {
		t.Errorf("waitHandshake took too long (%v) for configured timeout %v", elapsed, fastTimeout)
	}
}
