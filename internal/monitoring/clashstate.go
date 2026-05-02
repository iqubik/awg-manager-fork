package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ClashState caches per-outbound latencies pulled from sing-box's
// Clash API GET /proxies. Used by Scheduler to augment sing-box
// tunnel rows in the matrix snapshot with the latest urltest delay
// without doing one HTTP call per tunnel.
//
// Cache TTL defaults to 30s, matching the Scheduler tick — at most
// one Clash fetch per matrix snapshot. Failure to reach Clash leaves
// the cache untouched and returns (0, false), so sing-box tunnels
// still appear in the snapshot, just without the badge.
type ClashState struct {
	clashBaseURL func() string
	httpClient   *http.Client
	cacheTTL     time.Duration

	mu        sync.RWMutex
	latencies map[string]int // outbound tag → last known delay (ms)
	lastFetch time.Time
}

// NewClashState constructs the provider. httpClient may be nil; in that
// case a default 5s-timeout client is used.
func NewClashState(clashBaseURL func() string, httpClient *http.Client) *ClashState {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &ClashState{
		clashBaseURL: clashBaseURL,
		httpClient:   httpClient,
		cacheTTL:     30 * time.Second,
		latencies:    map[string]int{},
	}
}

// Invalidate marks the cache as stale so the next LatencyForOutbound
// call re-fetches /proxies. Used by force-refresh paths (e.g. the
// /monitoring/matrix?force=1 button).
func (c *ClashState) Invalidate() {
	c.mu.Lock()
	c.lastFetch = time.Time{}
	c.mu.Unlock()
}

// LatencyForOutbound returns the last-known delay for the named
// outbound. ok=false means: tag unknown, OR last delay was 0 (timeout
// / never tested), OR Clash is unreachable.
func (c *ClashState) LatencyForOutbound(ctx context.Context, tag string) (int, bool) {
	c.mu.RLock()
	fresh := !c.lastFetch.IsZero() && time.Since(c.lastFetch) < c.cacheTTL
	c.mu.RUnlock()

	if !fresh {
		_ = c.refresh(ctx) // ignore error — fall through to cached values
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	d, ok := c.latencies[tag]
	if !ok || d <= 0 {
		return 0, false
	}
	return d, true
}

func (c *ClashState) refresh(ctx context.Context) error {
	base := c.clashBaseURL()
	if base == "" {
		return errors.New("clash base URL not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/proxies", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("clash %d: %s", resp.StatusCode, string(body))
	}
	var parsed struct {
		Proxies map[string]struct {
			History []struct {
				Delay int `json:"delay"`
			} `json:"history"`
		} `json:"proxies"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.latencies = make(map[string]int, len(parsed.Proxies))
	for tag, p := range parsed.Proxies {
		if len(p.History) == 0 {
			continue
		}
		c.latencies[tag] = p.History[len(p.History)-1].Delay
	}
	c.lastFetch = time.Now()
	return nil
}
