package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/hoaxisr/awg-manager/internal/downloader"
	"github.com/hoaxisr/awg-manager/internal/sys/semver"
)

const releasesMaxBytes int64 = 1 << 20

type githubRelease struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

type stableReleaseInfo struct {
	RepoURL string
	APIURL  string
	TagName string
	Version string
	Assets  map[string]string
}

type releaseResolver struct {
	ttl    time.Duration
	fetch  func(context.Context, Downloader, string) (stableReleaseInfo, error)
	mu     sync.RWMutex
	cached stableReleaseInfo
	at     time.Time
}

func newReleaseResolver(ttl time.Duration) *releaseResolver {
	return &releaseResolver{
		ttl:   ttl,
		fetch: fetchHighestStableReleaseWithDownloader,
	}
}

func (r *releaseResolver) ResolveStable(ctx context.Context, dl Downloader, repoURL string) (stableReleaseInfo, error) {
	trimmedRepoURL := strings.TrimRight(strings.TrimSpace(repoURL), "/")
	if trimmedRepoURL == "" {
		return stableReleaseInfo{}, fmt.Errorf("release repo URL is empty")
	}

	if info, ok := r.peek(trimmedRepoURL); ok {
		return info, nil
	}

	info, err := r.fetch(ctx, dl, trimmedRepoURL)
	if err != nil {
		return stableReleaseInfo{}, err
	}
	r.store(info)
	return info, nil
}

func (r *releaseResolver) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cached = stableReleaseInfo{}
	r.at = time.Time{}
}

func (r *releaseResolver) peek(repoURL string) (stableReleaseInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.cached.RepoURL != repoURL || r.at.IsZero() || time.Since(r.at) > r.ttl {
		return stableReleaseInfo{}, false
	}
	return r.cached, true
}

func (r *releaseResolver) store(info stableReleaseInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cached = info
	r.at = time.Now()
}

var stableReleaseResolver = newReleaseResolver(10 * time.Minute)

func fetchHighestStableReleaseWithDownloader(
	ctx context.Context,
	dl Downloader,
	repoURL string,
) (stableReleaseInfo, error) {
	if dl == nil {
		dl = newDefaultDownloader()
	}

	apiURL, err := githubReleasesAPIURL(repoURL)
	if err != nil {
		return stableReleaseInfo{}, err
	}

	body, _, err := dl.ReadAll(ctx, downloader.Request{
		Purpose:      "awgm-update-check",
		URL:          apiURL,
		Method:       http.MethodGet,
		Timeout:      repoTimeout,
		MaxBodyBytes: releasesMaxBytes,
	})
	if err != nil {
		return stableReleaseInfo{}, fmt.Errorf("fetch releases api: %w", err)
	}

	var releases []githubRelease
	if err := json.Unmarshal(body, &releases); err != nil {
		return stableReleaseInfo{}, fmt.Errorf("decode releases api: %w", err)
	}

	var (
		selectedTag     string
		selectedVersion string
		selectedAssets  map[string]string
	)

	for _, release := range releases {
		if release.Draft || release.Prerelease {
			continue
		}

		version, ok := normalizeStableReleaseTag(release.TagName)
		if !ok {
			continue
		}

		if selectedVersion == "" || semver.Compare(version, selectedVersion) > 0 {
			selectedTag = release.TagName
			selectedVersion = version
			selectedAssets = make(map[string]string, len(release.Assets))
			for _, asset := range release.Assets {
				if strings.TrimSpace(asset.Name) == "" || strings.TrimSpace(asset.BrowserDownloadURL) == "" {
					continue
				}
				selectedAssets[asset.Name] = asset.BrowserDownloadURL
			}
		}
	}

	if selectedVersion == "" {
		return stableReleaseInfo{}, fmt.Errorf("no stable release found in %s", apiURL)
	}

	return stableReleaseInfo{
		RepoURL: repoURL,
		APIURL:  apiURL,
		TagName: selectedTag,
		Version: selectedVersion,
		Assets:  selectedAssets,
	}, nil
}

func githubReleasesAPIURL(repoURL string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(repoURL))
	if err != nil {
		return "", fmt.Errorf("invalid release repo URL: %w", err)
	}
	if !strings.EqualFold(u.Host, "github.com") {
		return "", fmt.Errorf("unsupported release repo host %q", u.Host)
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 3 || parts[2] != "releases" {
		return "", fmt.Errorf("invalid release repo path %q", u.Path)
	}

	return fmt.Sprintf("https://api.github.com/repos/%s/%s/releases?per_page=100", parts[0], parts[1]), nil
}

func normalizeStableReleaseTag(tag string) (string, bool) {
	trimmed := strings.TrimSpace(tag)
	if trimmed == "" || strings.EqualFold(trimmed, "iq-latest") {
		return "", false
	}

	if !strings.HasPrefix(trimmed, "v") {
		return "", false
	}
	if !releaseStableTagPattern.MatchString(trimmed) {
		return "", false
	}

	return strings.TrimPrefix(trimmed, "v"), true
}
