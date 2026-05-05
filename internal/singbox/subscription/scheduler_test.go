package subscription

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestScheduler_TickerCallsRefreshOnDue(t *testing.T) {
	store, _ := NewStore(filepath.Join(t.TempDir(), "s.json"))
	store.Create(CreateInput{Label: "a", URL: "u", RefreshHours: 1, Enabled: true})

	var calls int32
	doRefresh := func(ctx context.Context, id string) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}

	sched := NewScheduler(store, doRefresh)
	ctx, cancel := context.WithCancel(context.Background())
	sched.tick(ctx, time.Now().Add(2*time.Hour))
	cancel()

	// goroutine — give it a moment
	time.Sleep(50 * time.Millisecond)
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("refresh calls=%d want 1", got)
	}
}

func TestScheduler_SkipsDisabled(t *testing.T) {
	store, _ := NewStore(filepath.Join(t.TempDir(), "s.json"))
	store.Create(CreateInput{Label: "a", URL: "u", RefreshHours: 1, Enabled: false})

	var calls int32
	doRefresh := func(ctx context.Context, id string) error {
		atomic.AddInt32(&calls, 1)
		return nil
	}
	sched := NewScheduler(store, doRefresh)
	sched.tick(context.Background(), time.Now().Add(2*time.Hour))
	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&calls) != 0 {
		t.Error("disabled subscriptions must not refresh")
	}
}
