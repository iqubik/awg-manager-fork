package updater

import (
	"context"
	"fmt"
	"sync"
	"time"
	"net/http"
	"strings"

	"github.com/hoaxisr/awg-manager/internal/downloader"
)

func changelogSourcesForChannel(channel string) (primary, secondary string) {
	upstream := entwareRepoURL + "/CHANGELOG.md"
	if channel == channelDevelop {
		upstream = entwareRepoURL + "/develop/CHANGELOG.md"
	}
	if strings.TrimSpace(releaseBaseURL) == "" {
		return upstream, ""
	}
	return releaseAssetURL("CHANGELOG.md"), upstream
}

// changelogFetcher pulls the monolithic CHANGELOG.md, parses it, and
// serves cached results. Single-flight via fetchMu so a slow HTTP call
// converges to one real request under concurrent load.
type changelogFetcher struct {
	primaryURL   string
	secondaryURL string
	ttl        time.Duration
	downloader Downloader

	fetchMu sync.Mutex
	mu      sync.RWMutex
	cached  map[string]Entry
	fetched time.Time
}

func newChangelogFetcher(primaryURL, secondaryURL string, ttl time.Duration, dl Downloader) *changelogFetcher {
	return &changelogFetcher{primaryURL: primaryURL, secondaryURL: secondaryURL, ttl: ttl, downloader: dl}
}

// Fetch returns the parsed changelog map. Fresh cache hits skip HTTP;
// errors do not populate the cache so the next call retries.
func (c *changelogFetcher) Fetch(ctx context.Context) (map[string]Entry, error) {
	if entries, ok := c.peek(); ok {
		return entries, nil
	}

	c.fetchMu.Lock()
	defer c.fetchMu.Unlock()

	// Re-check after acquiring the mutex — another goroutine may have
	// populated the cache while we waited.
	if entries, ok := c.peek(); ok {
		return entries, nil
	}

	primaryEntries, primaryErr := c.fetchURL(ctx, c.primary())
	secondaryURL := c.secondary()
	if secondaryURL == "" {
		if primaryErr != nil {
			return nil, primaryErr
		}
		c.store(primaryEntries)
		return primaryEntries, nil
	}

	secondaryEntries, secondaryErr := c.fetchURL(ctx, secondaryURL)
	switch {
	case primaryErr == nil && secondaryErr == nil:
		merged := mergeChangelogEntries(primaryEntries, secondaryEntries)
		c.store(merged)
		return merged, nil
	case primaryErr == nil:
		c.store(primaryEntries)
		return primaryEntries, nil
	case secondaryErr == nil:
		c.store(secondaryEntries)
		return secondaryEntries, nil
	default:
		return nil, fmt.Errorf("all changelog sources failed: primary=%v secondary=%v", primaryErr, secondaryErr)
	}
}

// Invalidate forces the next Fetch to hit the network.
func (c *changelogFetcher) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cached = nil
}

// SetSources переключает источники changelog (например при смене канала).
// Сброс кэша гарантирует, что следующий Fetch ударит по новым URL.
func (c *changelogFetcher) SetSources(primaryURL, secondaryURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.primaryURL != primaryURL || c.secondaryURL != secondaryURL {
		c.primaryURL = primaryURL
		c.secondaryURL = secondaryURL
		c.cached = nil
	}
}

func (c *changelogFetcher) primary() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.primaryURL
}

func (c *changelogFetcher) secondary() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.secondaryURL
}

func (c *changelogFetcher) peek() (map[string]Entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.cached == nil || time.Since(c.fetched) > c.ttl {
		return nil, false
	}
	return c.cached, true
}

func (c *changelogFetcher) store(entries map[string]Entry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cached = entries
	c.fetched = time.Now()
}

func (c *changelogFetcher) fetchURL(ctx context.Context, url string) (map[string]Entry, error) {
	body, err := c.download(ctx, url)
	if err != nil {
		return nil, err
	}
	parsed, err := ParseChangelog(body)
	if err != nil {
		return nil, fmt.Errorf("parse changelog: %w", err)
	}
	return parsed, nil
}

func (c *changelogFetcher) download(ctx context.Context, url string) (string, error) {
	dl := c.downloader
	if dl == nil {
		dl = newDefaultDownloader()
	}
	body, meta, err := dl.ReadAll(ctx, downloader.Request{
		Purpose:       "awgm-changelog",
		URL:           url,
		Method:        http.MethodGet,
		Timeout:       repoTimeout,
		MaxBodyBytes:  changelogMaxBytes,
		AllowedStatus: []int{http.StatusOK, http.StatusNotFound},
	})
	if err != nil {
		return "", err
	}
	if meta.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("changelog not published yet")
	}
	if meta.StatusCode != http.StatusOK {
		return "", fmt.Errorf("changelog status %d", meta.StatusCode)
	}
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "<") {
		return "", fmt.Errorf("changelog source returned html instead of markdown")
	}
	return string(body), nil
}
