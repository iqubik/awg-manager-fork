package subscription

import (
	"context"
	"time"
)

// RefreshFunc is the callback the scheduler invokes for due subscriptions.
// In production it's Service.Refresh; tests inject a stub.
type RefreshFunc func(ctx context.Context, id string) error

// Scheduler walks the Store every tickInterval and refreshes any due item.
// Per-subscription serialization is enforced by the Service mutex; the
// scheduler runs each due item in its own goroutine.
type Scheduler struct {
	store        *Store
	doRefresh    RefreshFunc
	tickInterval time.Duration
	stop         chan struct{}
}

func NewScheduler(store *Store, doRefresh RefreshFunc) *Scheduler {
	return &Scheduler{
		store:        store,
		doRefresh:    doRefresh,
		tickInterval: time.Minute,
		stop:         make(chan struct{}),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	go s.loop(ctx)
}

func (s *Scheduler) Stop() {
	close(s.stop)
}

func (s *Scheduler) loop(ctx context.Context) {
	t := time.NewTicker(s.tickInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case now := <-t.C:
			s.tick(ctx, now)
		}
	}
}

// tick is the internal step exercised by tests.
func (s *Scheduler) tick(ctx context.Context, now time.Time) {
	for _, sub := range s.store.MaybeRefresh(now) {
		go s.doRefresh(ctx, sub.ID)
	}
}
