package diagnostics

import (
	"time"

	"github.com/hoaxisr/awg-manager/internal/logging"
)

// journalWarningsLimitPerBucket caps diagnostics report excerpts from each log bucket.
// Keep it bounded: reports are meant to be safe to download/share, while Total/Truncated
// still show whether more WARN/ERROR entries exist in the in-memory journal.
const journalWarningsLimitPerBucket = 300

// collectJournalWarnings collects WARN/ERROR entries from app and sing-box log buckets.
// Level "warn" is intentional: logging.IsVisible() treats ERROR and WARN as always visible,
// so requesting "warn" returns only error+warn and excludes info/full/debug.
func (r *Runner) collectJournalWarnings() *JournalWarningsInfo {
	if r.deps.LogService == nil {
		return nil
	}

	limit := journalWarningsLimitPerBucket

	return &JournalWarningsInfo{
		Levels:         []string{string(logging.LevelError), string(logging.LevelWarn)},
		LimitPerBucket: limit,
		AWGM:           r.collectJournalWarningBucket(logging.BucketApp, limit),
		Singbox:        r.collectJournalWarningBucket(logging.BucketSingbox, limit),
	}
}

func (r *Runner) collectJournalWarningBucket(bucket logging.Bucket, limit int) JournalWarningBucket {
	if limit <= 0 {
		limit = journalWarningsLimitPerBucket
	}

	logs, total := r.deps.LogService.GetBucketLogs(
		bucket,
		"",
		"",
		string(logging.LevelWarn),
		limit,
		0,
	)
	if logs == nil {
		logs = []logging.LogEntry{}
	}

	stats := r.deps.LogService.GetBucketStats(bucket)

	out := JournalWarningBucket{
		Bucket:         string(bucket),
		Total:          total,
		Included:       len(logs),
		Truncated:      total > len(logs),
		BufferSize:     stats.Size,
		BufferCapacity: stats.Capacity,
		Entries:        logs,
	}

	if !stats.Oldest.IsZero() {
		out.BufferOldestTimestamp = stats.Oldest.UTC().Format(time.RFC3339)
	}

	return out
}
