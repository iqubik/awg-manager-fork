package hydraroute

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestNextScheduledGeoUpdate(t *testing.T) {
	loc := time.FixedZone("TEST", 3*60*60)
	cases := []struct {
		name     string
		interval string
		now      time.Time
		want     time.Time
		wantZero bool
	}{
		{
			name:     "hourly from middle of hour",
			interval: GeoUpdateHour,
			now:      time.Date(2026, 6, 18, 10, 15, 0, 0, loc),
			want:     time.Date(2026, 6, 18, 11, 0, 0, 0, loc),
		},
		{
			name:     "hourly at exact hour",
			interval: GeoUpdateHour,
			now:      time.Date(2026, 6, 18, 10, 0, 0, 0, loc),
			want:     time.Date(2026, 6, 18, 11, 0, 0, 0, loc),
		},
		{
			name:     "6h before boundary",
			interval: GeoUpdate6H,
			now:      time.Date(2026, 6, 18, 5, 59, 0, 0, loc),
			want:     time.Date(2026, 6, 18, 6, 0, 0, 0, loc),
		},
		{
			name:     "6h at boundary",
			interval: GeoUpdate6H,
			now:      time.Date(2026, 6, 18, 6, 0, 0, 0, loc),
			want:     time.Date(2026, 6, 18, 12, 0, 0, 0, loc),
		},
		{
			name:     "daily before 03",
			interval: GeoUpdateDay,
			now:      time.Date(2026, 6, 18, 2, 59, 0, 0, loc),
			want:     time.Date(2026, 6, 18, 3, 0, 0, 0, loc),
		},
		{
			name:     "daily at 03",
			interval: GeoUpdateDay,
			now:      time.Date(2026, 6, 18, 3, 0, 0, 0, loc),
			want:     time.Date(2026, 6, 19, 3, 0, 0, 0, loc),
		},
		{
			name:     "weekly monday before 04",
			interval: GeoUpdateWeek,
			now:      time.Date(2026, 6, 22, 3, 59, 0, 0, loc),
			want:     time.Date(2026, 6, 22, 4, 0, 0, 0, loc),
		},
		{
			name:     "weekly monday at 04",
			interval: GeoUpdateWeek,
			now:      time.Date(2026, 6, 22, 4, 0, 0, 0, loc),
			want:     time.Date(2026, 6, 29, 4, 0, 0, 0, loc),
		},
		{
			name:     "weekly sunday",
			interval: GeoUpdateWeek,
			now:      time.Date(2026, 6, 21, 12, 0, 0, 0, loc),
			want:     time.Date(2026, 6, 22, 4, 0, 0, 0, loc),
		},
		{
			name:     "off returns zero",
			interval: GeoUpdateOff,
			wantZero: true,
		},
		{
			name:     "invalid returns zero",
			interval: "every-minute",
			wantZero: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := nextScheduledGeoUpdate(tc.interval, tc.now)
			if tc.wantZero {
				if !got.IsZero() {
					t.Fatalf("got = %v, want zero", got)
				}
				return
			}
			if !got.Equal(tc.want) {
				t.Fatalf("got = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGeoDataScheduler_RunUpdateCallsUpdateAndSync(t *testing.T) {
	var updatedCalls atomic.Int32
	var syncCalls atomic.Int32

	scheduler := &GeoDataScheduler{
		updateAll: func(ctx context.Context) (int, error) {
			updatedCalls.Add(1)
			return 2, nil
		},
		syncConfig: func() error {
			syncCalls.Add(1)
			return nil
		},
		logInfo: func(action, key, msg string) {},
		logWarn: func(action, key, msg string) {},
	}

	scheduler.runUpdate(context.Background(), GeoUpdateHour)

	if got := updatedCalls.Load(); got != 1 {
		t.Fatalf("updateAll calls = %d, want 1", got)
	}
	if got := syncCalls.Load(); got != 1 {
		t.Fatalf("syncConfig calls = %d, want 1", got)
	}
}

func TestGeoDataScheduler_RunUpdateStillAttemptsSyncAfterUpdateError(t *testing.T) {
	var syncCalls atomic.Int32

	scheduler := &GeoDataScheduler{
		updateAll: func(ctx context.Context) (int, error) {
			return 0, errors.New("boom")
		},
		syncConfig: func() error {
			syncCalls.Add(1)
			return nil
		},
		logInfo: func(action, key, msg string) {},
		logWarn: func(action, key, msg string) {},
	}

	scheduler.runUpdate(context.Background(), GeoUpdateHour)

	if got := syncCalls.Load(); got != 1 {
		t.Fatalf("syncConfig calls = %d, want 1", got)
	}
}

func TestGeoDataScheduler_RunUpdateSkipsParallelRun(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var updateCalls atomic.Int32

	scheduler := &GeoDataScheduler{
		updateAll: func(ctx context.Context) (int, error) {
			updateCalls.Add(1)
			close(started)
			<-release
			return 1, nil
		},
		syncConfig: func() error { return nil },
		logInfo:    func(action, key, msg string) {},
		logWarn:    func(action, key, msg string) {},
	}

	done := make(chan struct{})
	go func() {
		scheduler.runUpdate(context.Background(), GeoUpdateHour)
		close(done)
	}()

	<-started
	scheduler.runUpdate(context.Background(), GeoUpdateHour)
	close(release)
	<-done

	if got := updateCalls.Load(); got != 1 {
		t.Fatalf("updateAll calls = %d, want 1", got)
	}
}
