package hydraroute

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
)

type GeoDataScheduler struct {
	store      *GeoDataStore
	service    *Service
	running    atomic.Bool
	updateAll  func(ctx context.Context) (int, error)
	syncConfig func() error
	logInfo    func(action, key, msg string)
	logWarn    func(action, key, msg string)
}

func NewGeoDataScheduler(store *GeoDataStore, service *Service) *GeoDataScheduler {
	if store == nil || service == nil {
		return nil
	}
	return &GeoDataScheduler{
		store:      store,
		service:    service,
		updateAll:  func(ctx context.Context) (int, error) { return store.UpdateAllWithClientVia(ctx, nil, "") },
		syncConfig: service.SyncGeoFilesToConfig,
		logInfo:    service.appLog.Info,
		logWarn:    service.appLog.Warn,
	}
}

func (s *GeoDataScheduler) Start(ctx context.Context) {
	if s == nil {
		return
	}
	go s.loop(ctx)
}

func (s *GeoDataScheduler) loop(ctx context.Context) {
	var lastKey string
	var nextRun time.Time

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		schedule := s.store.GetSchedule()
		key := schedule.Interval + "|" + schedule.UpdatedAt
		enabled := schedule.Interval != GeoUpdateOff
		now := time.Now()

		if !enabled {
			lastKey = key
			nextRun = time.Time{}
		} else if key != lastKey || nextRun.IsZero() {
			nextRun = nextScheduledGeoUpdate(schedule.Interval, now)
			lastKey = key
		}

		wait := time.Minute
		if enabled && !nextRun.IsZero() {
			until := time.Until(nextRun)
			if until < 0 {
				until = 0
			}
			if until < wait {
				wait = until
			}
		}

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		if !enabled || nextRun.IsZero() || time.Now().Before(nextRun) {
			continue
		}

		current := s.store.GetSchedule()
		currentKey := current.Interval + "|" + current.UpdatedAt
		if current.Interval == GeoUpdateOff || currentKey != lastKey {
			lastKey = currentKey
			nextRun = time.Time{}
			continue
		}

		s.runUpdate(ctx, current.Interval)
		nextRun = nextScheduledGeoUpdate(current.Interval, time.Now().Add(time.Second))
	}
}

func (s *GeoDataScheduler) runUpdate(ctx context.Context, interval string) {
	if !s.running.CompareAndSwap(false, true) {
		s.logWarn("geo-schedule-skip", interval, "previous auto-update is still running")
		return
	}
	defer s.running.Store(false)

	s.logInfo("geo-schedule-start", interval, fmt.Sprintf("auto-update triggered (%s)", interval))

	updated, err := s.updateAll(ctx)
	if err != nil {
		s.logWarn("geo-schedule-update", interval, err.Error())
	} else {
		s.logInfo("geo-schedule-update", interval, fmt.Sprintf("updated %d geo files", updated))
	}

	if syncErr := s.syncConfig(); syncErr != nil {
		s.logWarn("geo-schedule-sync", interval, syncErr.Error())
	} else {
		s.logInfo("geo-schedule-sync", interval, "geo files synced to hrneo.conf")
	}
}

func nextScheduledGeoUpdate(interval string, now time.Time) time.Time {
	loc := now.Location()

	switch interval {
	case GeoUpdateHour:
		return now.Truncate(time.Hour).Add(time.Hour)
	case GeoUpdate6H:
		base := time.Date(now.Year(), now.Month(), now.Day(), (now.Hour()/6)*6, 0, 0, 0, loc)
		next := base.Add(6 * time.Hour)
		if !next.After(now) {
			next = next.Add(6 * time.Hour)
		}
		return next
	case GeoUpdateDay:
		next := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, loc)
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		return next
	case GeoUpdateWeek:
		next := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, loc)
		for next.Weekday() != time.Monday || !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		return next
	default:
		return time.Time{}
	}
}
