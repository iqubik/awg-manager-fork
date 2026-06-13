package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hoaxisr/awg-manager/internal/downloader"
	"github.com/hoaxisr/awg-manager/internal/sys/semver"
)

func changelogSourcesForChannel(channel string) (primary, secondary string) {
	if baseURL := releaseBaseURLForChannel(channel); baseURL != "" {
		return releaseAssetURL(baseURL, "CHANGELOG.md"), ""
	}

	if channel == channelDevelop {
		return entwareRepoURL + "/develop/CHANGELOG.md", ""
	}
	return entwareRepoURL + "/CHANGELOG.md", ""
}

func resolveChangelogSourcesForChannel(ctx context.Context, dl Downloader, channel string) (primary, secondary string, err error) {
	if channel == channelStable {
		if source, err := resolveStableChangelogSource(ctx, dl); err != nil {
			return "", "", err
		} else if source != nil {
			return source.primaryURL, source.secondaryURL, nil
		}
	}

	primary, secondary = changelogSourcesForChannel(channel)
	return primary, secondary, nil
}

type resolvedChangelogSource struct {
	primaryURL   string
	secondaryURL string
	resolve      func(context.Context) (map[string]Entry, error)
}

func resolveStableChangelogSource(ctx context.Context, dl Downloader) (*resolvedChangelogSource, error) {
	baseURL := releaseBaseURLForChannel(channelStable)
	if baseURL == "" {
		return nil, nil
	}

	changelogURL := releaseAssetURL(baseURL, "CHANGELOG.md")
	if err := ensureStableLatestAssetExists(ctx, dl, changelogURL, "CHANGELOG.md"); err == nil {
		return &resolvedChangelogSource{primaryURL: changelogURL}, nil
	}

	repoURL := normalizedReleaseRepoURL()
	info, err := stableReleaseResolver.ResolveStable(ctx, dl, repoURL)
	if err != nil {
		return nil, err
	}
	if assetURL := strings.TrimSpace(info.Assets["CHANGELOG.md"]); assetURL != "" {
		return &resolvedChangelogSource{primaryURL: assetURL}, nil
	}

	return &resolvedChangelogSource{
		primaryURL: info.HTMLURL,
		resolve: func(ctx context.Context) (map[string]Entry, error) {
			return fetchStableReleaseEntriesWithDownloader(ctx, dl, repoURL)
		},
	}, nil
}

// changelogFetcher pulls the monolithic CHANGELOG.md, parses it, and
// serves cached results. Single-flight via fetchMu so a slow HTTP call
// converges to one real request under concurrent load.
type changelogFetcher struct {
	primaryURL   string
	secondaryURL string
	resolve      func(context.Context) (map[string]Entry, error)
	ttl          time.Duration
	downloader   Downloader

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

	if entries, err, ok := c.fetchResolved(ctx); ok {
		if err != nil {
			return nil, err
		}
		c.store(entries)
		return entries, nil
	}

	primaryURL, secondaryURL := c.sources()

	primaryEntries, primaryErr := c.fetchURL(ctx, primaryURL)
	switch {
	case primaryErr == nil && secondaryURL == "":
		c.store(primaryEntries)
		return primaryEntries, nil
	case primaryErr != nil && secondaryURL == "":
		return nil, primaryErr
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
	if c.primaryURL != primaryURL || c.secondaryURL != secondaryURL || c.resolve != nil {
		c.primaryURL = primaryURL
		c.secondaryURL = secondaryURL
		c.resolve = nil
		c.cached = nil
	}
}

func (c *changelogFetcher) SetResolvedSources(primaryURL, secondaryURL string, resolve func(context.Context) (map[string]Entry, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.primaryURL = primaryURL
	c.secondaryURL = secondaryURL
	c.resolve = resolve
	c.cached = nil
}

func (c *changelogFetcher) sources() (string, string) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.primaryURL, c.secondaryURL
}

func (c *changelogFetcher) fetchResolved(ctx context.Context) (map[string]Entry, error, bool) {
	c.mu.RLock()
	resolve := c.resolve
	c.mu.RUnlock()
	if resolve == nil {
		return nil, nil, false
	}

	entries, err := resolve(ctx)
	if err != nil {
		return nil, err, true
	}
	return entries, nil, true
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
		return "", sanitizeReleaseAssetError(releaseAssetRef{
			channel: detectReleaseAssetChannel(url),
			tag:     detectReleaseAssetTag(url),
			name:    "CHANGELOG.md",
		}, err)
	}
	if meta.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("release asset CHANGELOG.md not found in %s", detectReleaseAssetTarget(url))
	}
	if meta.StatusCode != http.StatusOK {
		return "", fmt.Errorf("changelog status %d", meta.StatusCode)
	}
	trimmed := strings.TrimSpace(string(body))
	if strings.HasPrefix(trimmed, "<") {
		return "", fmt.Errorf("release asset CHANGELOG.md not found in %s", detectReleaseAssetTarget(url))
	}
	return string(body), nil
}

func detectReleaseAssetChannel(url string) string {
	if strings.Contains(url, "/download/iq-latest/") {
		return channelDevelop
	}
	return channelStable
}

func detectReleaseAssetTag(url string) string {
	if strings.Contains(url, "/download/iq-latest/") {
		return "iq-latest"
	}
	marker := "/download/"
	idx := strings.Index(url, marker)
	if idx < 0 {
		return ""
	}
	rest := url[idx+len(marker):]
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return ""
	}
	return rest[:slash]
}

func detectReleaseAssetTarget(url string) string {
	return releaseAssetDisplayTarget(releaseAssetRef{
		channel: detectReleaseAssetChannel(url),
		tag:     detectReleaseAssetTag(url),
	})
}

var htmlTagPattern = regexp.MustCompile(`(?s)<[^>]*>`)
var releaseBodyParagraphSplitPattern = regexp.MustCompile(`\n\s*\n+`)

func fetchStableReleaseEntriesWithDownloader(ctx context.Context, dl Downloader, repoURL string) (map[string]Entry, error) {
	if dl == nil {
		dl = newDefaultDownloader()
	}

	apiBaseURL, err := githubRepoAPIBaseURL(repoURL)
	if err != nil {
		return nil, err
	}

	releasesAPIURL := apiBaseURL + "/releases?per_page=100"
	body, meta, err := dl.ReadAll(ctx, downloader.Request{
		Purpose:       "awgm-changelog",
		URL:           releasesAPIURL,
		Method:        http.MethodGet,
		Timeout:       repoTimeout,
		MaxBodyBytes:  releasesMaxBytes,
		AllowedStatus: []int{http.StatusOK, http.StatusNotFound},
	})
	if err != nil {
		return nil, fmt.Errorf("fetch stable releases: %w", err)
	}
	if meta.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("stable releases list is not published")
	}
	if meta.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch stable releases: status %d", meta.StatusCode)
	}

	var releases []githubRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return nil, fmt.Errorf("decode stable releases: %w", err)
	}

	candidates := make([]stableReleaseInfo, 0, len(releases))
	for idx, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}
		if _, ok := normalizeStableReleaseTag(strings.TrimSpace(release.TagName)); !ok {
			continue
		}
		info, err := stableReleaseInfoFromGitHubRelease(repoURL, fmt.Sprintf("%s/releases/%d", apiBaseURL, idx), release)
		if err != nil {
			continue
		}
		candidates = append(candidates, info)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return semver.Compare(candidates[i].Version, candidates[j].Version) > 0
	})

	entries := make(map[string]Entry, len(candidates))
	for _, candidate := range candidates {
		if candidate.Version == "" {
			continue
		}
		if _, exists := entries[candidate.Version]; exists {
			continue
		}
		entries[candidate.Version] = stableReleaseBodyEntry(candidate)
	}
	return entries, nil
}

func stableReleaseBodyEntry(info stableReleaseInfo) Entry {
	body := strings.TrimSpace(info.Body)
	body = htmlTagPattern.ReplaceAllString(body, "")
	paragraphs := splitReleaseBodyParagraphs(body)

	entry := Entry{
		Version: info.Version,
		Date:    info.publishedDate(),
	}
	if len(paragraphs) > 0 {
		entry.Groups = []Group{{
			Heading: "",
			Items:   paragraphs,
		}}
	}
	return entry
}

func splitReleaseBodyParagraphs(body string) []string {
	if strings.TrimSpace(body) == "" {
		return nil
	}

	chunks := releaseBodyParagraphSplitPattern.Split(body, -1)
	paragraphs := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		chunk = strings.TrimSpace(chunk)
		if chunk == "" {
			continue
		}
		paragraphs = append(paragraphs, chunk)
	}
	return paragraphs
}
