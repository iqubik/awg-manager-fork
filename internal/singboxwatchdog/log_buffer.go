package singboxwatchdog

import "sync"

type LogBuffer struct {
	mu      sync.RWMutex
	entries []LogEntry
	max     int
}

func NewLogBuffer(max int) *LogBuffer {
	if max <= 0 {
		max = 500
	}
	return &LogBuffer{
		entries: make([]LogEntry, 0, max),
		max:     max,
	}
}

func (b *LogBuffer) Add(e LogEntry) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = append(b.entries, e)
	if len(b.entries) > b.max {
		b.entries = append([]LogEntry(nil), b.entries[len(b.entries)-b.max:]...)
	}
}

func (b *LogBuffer) GetAll() []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]LogEntry, len(b.entries))
	copy(out, b.entries)
	return out
}

func (b *LogBuffer) GetByTarget(id string) []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]LogEntry, 0, len(b.entries))
	for _, entry := range b.entries {
		if entry.TargetID == id {
			out = append(out, entry)
		}
	}
	return out
}

func (b *LogBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.entries = b.entries[:0]
}
