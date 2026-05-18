package diagnostics

import (
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/logging"
)

type journalWarningsLogCall struct {
	bucket   logging.Bucket
	group    string
	subgroup string
	level    string
	limit    int
	offset   int
}

type fakeJournalWarningsLogService struct {
	calls  []journalWarningsLogCall
	logs   map[logging.Bucket][]logging.LogEntry
	totals map[logging.Bucket]int
	stats  map[logging.Bucket]logging.BufferStats
}

func (f *fakeJournalWarningsLogService) GetLogs(category, level string) []logging.LogEntry {
	return nil
}

func (f *fakeJournalWarningsLogService) GetBucketLogs(bucket logging.Bucket, group, subgroup, level string, limit, offset int) ([]logging.LogEntry, int) {
	f.calls = append(f.calls, journalWarningsLogCall{
		bucket:   bucket,
		group:    group,
		subgroup: subgroup,
		level:    level,
		limit:    limit,
		offset:   offset,
	})
	return f.logs[bucket], f.totals[bucket]
}

func (f *fakeJournalWarningsLogService) GetBucketStats(bucket logging.Bucket) logging.BufferStats {
	return f.stats[bucket]
}

func TestCollectJournalWarningsCollectsAppAndSingboxWarnLevel(t *testing.T) {
	oldest := time.Date(2026, 5, 18, 12, 0, 0, 0, time.UTC)

	fake := &fakeJournalWarningsLogService{
		logs: map[logging.Bucket][]logging.LogEntry{
			logging.BucketApp: {
				{Level: "warn", Group: "system", Message: "app warning"},
				{Level: "error", Group: "tunnel", Message: "app error"},
			},
			logging.BucketSingbox: {
				{Level: "error", Group: "singbox", Subgroup: "outbound", Message: "singbox error"},
			},
		},
		totals: map[logging.Bucket]int{
			logging.BucketApp:     5,
			logging.BucketSingbox: 1,
		},
		stats: map[logging.Bucket]logging.BufferStats{
			logging.BucketApp: {
				Bucket:   logging.BucketApp,
				Size:     10,
				Capacity: 5000,
				Oldest:   oldest,
			},
			logging.BucketSingbox: {
				Bucket:   logging.BucketSingbox,
				Size:     3,
				Capacity: 7000,
				Oldest:   oldest.Add(time.Hour),
			},
		},
	}

	runner := NewRunner(Deps{LogService: fake})
	got := runner.collectJournalWarnings()

	if got == nil {
		t.Fatal("collectJournalWarnings returned nil")
	}
	if got.LimitPerBucket != journalWarningsLimitPerBucket {
		t.Fatalf("LimitPerBucket = %d, want %d", got.LimitPerBucket, journalWarningsLimitPerBucket)
	}
	if len(got.Levels) != 2 || got.Levels[0] != "error" || got.Levels[1] != "warn" {
		t.Fatalf("Levels = %#v, want [error warn]", got.Levels)
	}

	if len(fake.calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(fake.calls))
	}

	wantBuckets := []logging.Bucket{logging.BucketApp, logging.BucketSingbox}
	for i, call := range fake.calls {
		if call.bucket != wantBuckets[i] {
			t.Fatalf("call[%d].bucket = %s, want %s", i, call.bucket, wantBuckets[i])
		}
		if call.group != "" || call.subgroup != "" {
			t.Fatalf("call[%d] group/subgroup = %q/%q, want empty", i, call.group, call.subgroup)
		}
		if call.level != string(logging.LevelWarn) {
			t.Fatalf("call[%d].level = %q, want warn", i, call.level)
		}
		if call.limit != journalWarningsLimitPerBucket || call.offset != 0 {
			t.Fatalf("call[%d] limit/offset = %d/%d, want %d/0", i, call.limit, call.offset, journalWarningsLimitPerBucket)
		}
	}

	if got.AWGM.Bucket != string(logging.BucketApp) {
		t.Fatalf("AWGM bucket = %q, want app", got.AWGM.Bucket)
	}
	if got.AWGM.Total != 5 || got.AWGM.Included != 2 || !got.AWGM.Truncated {
		t.Fatalf("AWGM total/included/truncated = %d/%d/%v, want 5/2/true",
			got.AWGM.Total, got.AWGM.Included, got.AWGM.Truncated)
	}
	if got.AWGM.BufferSize != 10 || got.AWGM.BufferCapacity != 5000 {
		t.Fatalf("AWGM buffer size/capacity = %d/%d, want 10/5000",
			got.AWGM.BufferSize, got.AWGM.BufferCapacity)
	}
	if got.AWGM.BufferOldestTimestamp != oldest.Format(time.RFC3339) {
		t.Fatalf("AWGM buffer oldest = %q, want %q",
			got.AWGM.BufferOldestTimestamp, oldest.Format(time.RFC3339))
	}

	if got.Singbox.Bucket != string(logging.BucketSingbox) {
		t.Fatalf("Singbox bucket = %q, want singbox", got.Singbox.Bucket)
	}
	if got.Singbox.Total != 1 || got.Singbox.Included != 1 || got.Singbox.Truncated {
		t.Fatalf("Singbox total/included/truncated = %d/%d/%v, want 1/1/false",
			got.Singbox.Total, got.Singbox.Included, got.Singbox.Truncated)
	}
	if got.Singbox.BufferSize != 3 || got.Singbox.BufferCapacity != 7000 {
		t.Fatalf("Singbox buffer size/capacity = %d/%d, want 3/7000",
			got.Singbox.BufferSize, got.Singbox.BufferCapacity)
	}
}

func TestCollectJournalWarningsWithoutLogServiceReturnsNil(t *testing.T) {
	runner := NewRunner(Deps{})
	if got := runner.collectJournalWarnings(); got != nil {
		t.Fatalf("collectJournalWarnings without LogService = %#v, want nil", got)
	}
}

func TestCollectJournalWarningBucketNormalizesNilEntriesAndLimit(t *testing.T) {
	fake := &fakeJournalWarningsLogService{
		logs: map[logging.Bucket][]logging.LogEntry{
			logging.BucketApp: nil,
		},
		totals: map[logging.Bucket]int{
			logging.BucketApp: 0,
		},
		stats: map[logging.Bucket]logging.BufferStats{
			logging.BucketApp: {
				Bucket:   logging.BucketApp,
				Size:     0,
				Capacity: 5000,
			},
		},
	}

	runner := NewRunner(Deps{LogService: fake})
	got := runner.collectJournalWarningBucket(logging.BucketApp, 0)

	if len(fake.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(fake.calls))
	}
	if fake.calls[0].limit != journalWarningsLimitPerBucket {
		t.Fatalf("limit = %d, want default %d", fake.calls[0].limit, journalWarningsLimitPerBucket)
	}
	if got.Entries == nil {
		t.Fatal("Entries is nil, want empty slice so JSON encodes [] not null")
	}
	if len(got.Entries) != 0 {
		t.Fatalf("Entries len = %d, want 0", len(got.Entries))
	}
	if got.Truncated {
		t.Fatalf("Truncated = true, want false for empty bucket")
	}
}
