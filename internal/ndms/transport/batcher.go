package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/hoaxisr/awg-manager/internal/logging"
)

// readReq — один pending read submission в Batcher.
type readReq struct {
	ctx   context.Context
	path  string
	reply chan readResult // capacity 1, single recipient
}

// readResult — результат одного read, доставляется на reply channel.
type readResult struct {
	body []byte
	err  error
}

// Batcher собирает RCI GET-запросы в один POST с массивом команд
// (DataLoader pattern). Между Client.GetRaw и Client.postJSON.
type Batcher struct {
	cli      *Client
	submit   chan readReq
	window   time.Duration
	maxBatch int
	log      *logging.ScopedLogger

	// Lifecycle
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	done           chan struct{}

	// Counters
	submittedReads atomic.Uint64
	coalescedReads atomic.Uint64
	cancelledDrops atomic.Uint64
}

// newBatcher constructs a Batcher. Call Start() to begin processing.
func newBatcher(cli *Client, window time.Duration, maxBatch, submitBuf int) *Batcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &Batcher{
		cli:            cli,
		submit:         make(chan readReq, submitBuf),
		window:         window,
		maxBatch:       maxBatch,
		shutdownCtx:    ctx,
		shutdownCancel: cancel,
		done:           make(chan struct{}),
	}
}

// SetAppLogger wires the UI-visible logger. Optional, nil-safe.
func (b *Batcher) SetAppLogger(log *logging.ScopedLogger) {
	b.log = log
}

// Start launches the flusher goroutine.
func (b *Batcher) Start() {
	go b.flusherLoop()
}

// Submit enqueues a read. Blocks until result is available, ctx is
// cancelled, or batcher is closed.
func (b *Batcher) Submit(ctx context.Context, path string) ([]byte, error) {
	req := readReq{
		ctx:   ctx,
		path:  path,
		reply: make(chan readResult, 1),
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-b.shutdownCtx.Done():
		return nil, ErrBatcherClosed
	case b.submit <- req:
	}

	select {
	case res := <-req.reply:
		return res.body, res.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Close shuts down the flusher. Blocks until in-flight flush completes.
func (b *Batcher) Close() {
	b.shutdownCancel()
	<-b.done
}

// flusherLoop is the core scheduler — runs as goroutine.
func (b *Batcher) flusherLoop() {
	defer close(b.done)
	for {
		select {
		case <-b.shutdownCtx.Done():
			// Drain any already-enqueued submits so callers aren't orphaned.
			// Use a fresh background context: shutdownCtx is already cancelled.
			var pending []readReq
			for {
				select {
				case req := <-b.submit:
					pending = append(pending, req)
				default:
					goto drained
				}
			}
		drained:
			if len(pending) > 0 {
				b.flush(context.Background(), pending)
			}
			return
		case req := <-b.submit:
			pending := []readReq{req}
			b.collectAndFlush(pending)
		}
	}
}

// collectAndFlush собирает дополнительные submits в течение window,
// затем flushes batch.
func (b *Batcher) collectAndFlush(pending []readReq) {
	timer := time.NewTimer(b.window)
	defer timer.Stop()

collect:
	for {
		select {
		case <-b.shutdownCtx.Done():
			b.flush(context.Background(), pending)
			return
		case <-timer.C:
			break collect
		case more := <-b.submit:
			pending = append(pending, more)
			if len(pending) >= b.maxBatch {
				if !timer.Stop() {
					<-timer.C
				}
				break collect
			}
		}
	}
	b.flush(b.shutdownCtx, pending)
}

// flush обрабатывает один batch: coalesce → POST → distribute.
func (b *Batcher) flush(ctx context.Context, pending []readReq) {
	if len(pending) == 0 {
		return
	}

	// Pre-flush filter: drop submits whose ctx is already cancelled.
	// Caller уже получил ctx.Err() из Submit'a — мы только cleanup'им
	// orphan reply channels и считаем counter.
	alive := pending[:0]
	for _, r := range pending {
		if r.ctx.Err() == nil {
			alive = append(alive, r)
		} else {
			close(r.reply)
			b.cancelledDrops.Add(1)
		}
	}
	if len(alive) == 0 {
		return
	}
	pending = alive

	byPath := map[string][]readReq{}
	uniqueOrder := []string{}
	for _, r := range pending {
		if _, seen := byPath[r.path]; !seen {
			uniqueOrder = append(uniqueOrder, r.path)
		}
		byPath[r.path] = append(byPath[r.path], r)
	}

	batch := make([]any, 0, len(uniqueOrder))
	validPaths := make([]string, 0, len(uniqueOrder))
	for _, path := range uniqueOrder {
		cmd, err := pathToCommand(path)
		if err != nil {
			for _, r := range byPath[path] {
				r.reply <- readResult{nil, err}
				close(r.reply)
			}
			delete(byPath, path)
			continue
		}
		batch = append(batch, cmd)
		validPaths = append(validPaths, path)
	}

	if len(batch) == 0 {
		return
	}

	b.submittedReads.Add(uint64(len(pending)))
	b.coalescedReads.Add(uint64(len(batch)))

	raw, err := b.cli.postJSON(ctx, batch)
	if err != nil {
		b.distributeAll(byPath, validPaths, nil, fmt.Errorf("rci batch: %w", err))
		return
	}

	var responses []json.RawMessage
	if err := json.Unmarshal(raw, &responses); err != nil {
		b.distributeAll(byPath, validPaths, nil, ErrBatchResponseShape)
		return
	}
	if len(responses) != len(validPaths) {
		b.distributeAll(byPath, validPaths, nil, ErrBatchLengthMismatch)
		return
	}

	for i, path := range validPaths {
		body := []byte(responses[i])
		var itemErr error
		if msg := ExtractError(body); msg != "" {
			itemErr = &NDMSAppError{Method: "POST-BATCH", Path: path, Message: msg, Body: body}
		}
		for _, r := range byPath[path] {
			r.reply <- readResult{body, itemErr}
			close(r.reply)
		}
	}
}

// distributeAll отдаёт одинаковый result/error всем receivers в batch'е.
func (b *Batcher) distributeAll(byPath map[string][]readReq, paths []string, body []byte, err error) {
	for _, path := range paths {
		for _, r := range byPath[path] {
			r.reply <- readResult{body, err}
			close(r.reply)
		}
	}
}
