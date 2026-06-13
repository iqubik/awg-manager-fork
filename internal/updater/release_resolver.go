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

type githubMatchingRef struct {
	Ref string `json:"ref"`
}

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

	apiBaseURL, err := githubRepoAPIBaseURL(repoURL)
	if err != nil {
		return stableReleaseInfo{}, err
	}

	tagsURL := apiBaseURL + "/git/matching-refs/tags/v"
	body, _, err := dl.ReadAll(ctx, downloader.Request{
		Purpose:      "awgm-update-check",
		URL:          tagsURL,
		Method:       http.MethodGet,
		Timeout:      repoTimeout,
		MaxBodyBytes: releasesMaxBytes,
	})
	if err != nil {
		return stableReleaseInfo{}, fmt.Errorf("fetch git tags api: %w", err)
	}

	var refs []githubMatchingRef
	if err := json.Unmarshal(body, &refs); err != nil {
		return stableReleaseInfo{}, fmt.Errorf("decode git tags api: %w", err)
	}

	selectedTag, selectedVersion := selectHighestStableTag(refs)
	if selectedVersion == "" {
		return stableReleaseInfo{}, fmt.Errorf("no stable tag found in %s", tagsURL)
	}

	releaseAPIURL := apiBaseURL + "/releases/tags/" + selectedTag
	body, meta, err := dl.ReadAll(ctx, downloader.Request{
		Purpose:       "awgm-update-check",
		URL:           releaseAPIURL,
		Method:        http.MethodGet,
		Timeout:       repoTimeout,
		MaxBodyBytes:  releasesMaxBytes,
		AllowedStatus: []int{http.StatusOK, http.StatusNotFound},
	})
	if err != nil {
		return stableReleaseInfo{}, fmt.Errorf("fetch stable release %s: %w", selectedTag, err)
	}
	if meta.StatusCode == http.StatusNotFound {
		return stableReleaseInfo{}, fmt.Errorf("stable release %s is not published", selectedTag)
	}
	if meta.StatusCode != http.StatusOK {
		return stableReleaseInfo{}, fmt.Errorf("fetch stable release %s: status %d", selectedTag, meta.StatusCode)
	}

	var release githubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return stableReleaseInfo{}, fmt.Errorf("decode stable release %s: %w", selectedTag, err)
	}
	if strings.TrimSpace(release.TagName) != selectedTag {
		return stableReleaseInfo{}, fmt.Errorf("stable release %s returned tag %q", selectedTag, strings.TrimSpace(release.TagName))
	}
	if release.Draft || release.Prerelease {
		return stableReleaseInfo{}, fmt.Errorf("stable release %s is not published", selectedTag)
	}

	selectedAssets := make(map[string]string, len(release.Assets))
	for _, asset := range release.Assets {
		if strings.TrimSpace(asset.Name) == "" || strings.TrimSpace(asset.BrowserDownloadURL) == "" {
			continue
		}
		selectedAssets[asset.Name] = asset.BrowserDownloadURL
	}

	return stableReleaseInfo{
		RepoURL: repoURL,
		APIURL:  releaseAPIURL,
		TagName: selectedTag,
		Version: selectedVersion,
		Assets:  selectedAssets,
	}, nil
}

func githubRepoAPIBaseURL(repoURL string) (string, error) {
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

	return fmt.Sprintf("https://api.github.com/repos/%s/%s", parts[0], parts[1]), nil
}

func selectHighestStableTag(refs []githubMatchingRef) (tag string, version string) {
	for _, ref := range refs {
		tagName := strings.TrimSpace(ref.Ref)
		tagName = strings.TrimPrefix(tagName, "refs/tags/")
		normalizedVersion, ok := normalizeStableReleaseTag(tagName)
		if !ok {
			continue
		}
		if version == "" || semver.Compare(normalizedVersion, version) > 0 {
			tag = tagName
			version = normalizedVersion
		}
	}
	return tag, version
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
