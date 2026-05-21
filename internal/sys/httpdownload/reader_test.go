package httpdownload

import (
	"bytes"
	"io"
	"sync/atomic"
	"testing"
)

type fixedChunkReader struct {
	data []byte
	pos  int
	step int
}

func (r *fixedChunkReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := r.step
	if n > len(p) {
		n = len(p)
	}
	remain := len(r.data) - r.pos
	if n > remain {
		n = remain
	}
	copy(p[:n], r.data[r.pos:r.pos+n])
	r.pos += n
	return n, nil
}

func TestReader_PassthroughBytes(t *testing.T) {
	src := bytes.Repeat([]byte("x"), 1024)
	pr := NewReader(bytes.NewReader(src), int64(len(src)), nil)
	got, err := io.ReadAll(pr)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, src) {
		t.Errorf("payload mismatch: got %d bytes, want %d", len(got), len(src))
	}
	if pr.BytesRead() != int64(len(src)) {
		t.Errorf("BytesRead = %d, want %d", pr.BytesRead(), len(src))
	}
}

func TestReader_EmitsAtLeastOnceOnEOF(t *testing.T) {
	// 1 KB of data — well under emitBytes threshold, so we should still
	// see a final emit triggered by io.EOF.
	src := bytes.Repeat([]byte("x"), 1024)
	var calls atomic.Int32
	var lastDownloaded atomic.Int64
	pr := NewReader(bytes.NewReader(src), int64(len(src)), func(downloaded, total int64) {
		calls.Add(1)
		lastDownloaded.Store(downloaded)
	})
	if _, err := io.ReadAll(pr); err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if calls.Load() == 0 {
		t.Error("expected at least one progress emit (EOF flush)")
	}
	if got := lastDownloaded.Load(); got != int64(len(src)) {
		t.Errorf("last downloaded = %d, want %d (final frame)", got, len(src))
	}
}

func TestReader_EmitsAfterByteThreshold(t *testing.T) {
	// Use deterministic 32KB chunks so 256KB always crosses the 64KB
	// threshold exactly 4 times (EOF flush may or may not add another one
	// depending on whether the final emit landed exactly at EOF).
	src := bytes.Repeat([]byte("x"), 256*1024)
	var calls atomic.Int32
	r := &fixedChunkReader{data: src, step: 32 * 1024}
	pr := NewReader(r, int64(len(src)), func(downloaded, total int64) {
		calls.Add(1)
	})
	if _, err := io.ReadAll(pr); err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if got := calls.Load(); got < 4 {
		t.Errorf("expected ≥4 emits across 256KB stream, got %d", got)
	}
}

func TestReader_NilProgressIsSafe(t *testing.T) {
	src := bytes.Repeat([]byte("x"), 1024)
	pr := NewReader(bytes.NewReader(src), 0, nil)
	if _, err := io.ReadAll(pr); err != nil {
		t.Fatalf("ReadAll with nil progress: %v", err)
	}
}
