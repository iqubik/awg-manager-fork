package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
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
	TagName     string    `json:"tag_name"`
	HTMLURL     string    `json:"html_url"`
	Body        string    `json:"body"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	CreatedAt   time.Time `json:"created_at"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

type stableTagCandidate struct {
	TagName string
	Version string
}

type stableReleaseInfo struct {
	RepoURL     string
	APIURL      string
	HTMLURL     string
	TagName     string
	Version     string
	Body        string
	PublishedAt time.Time
	CreatedAt   time.Time
	Assets      map[string]string
}

func stableReleaseInfoFromGitHubRelease(repoURL, apiURL string, release githubRelease) (stableReleaseInfo, error) {
	tagName := strings.TrimSpace(release.TagName)
	version, ok := normalizeStableLatestReleaseTag(tagName)
	if !ok {
		return stableReleaseInfo{}, fmt.Errorf("stable latest release has non-canonical tag %q", tagName)
	}

	selectedAssets := make(map[string]string, len(release.Assets))
	for _, asset := range release.Assets {
		if strings.TrimSpace(asset.Name) == "" || strings.TrimSpace(asset.BrowserDownloadURL) == "" {
			continue
		}
		selectedAssets[asset.Name] = asset.BrowserDownloadURL
	}

	htmlURL := strings.TrimSpace(release.HTMLURL)
	if htmlURL == "" {
		htmlURL = strings.TrimRight(repoURL, "/") + "/tag/" + tagName
	}

	return stableReleaseInfo{
		RepoURL:     repoURL,
		APIURL:      apiURL,
		HTMLURL:     htmlURL,
		TagName:     tagName,
		Version:     version,
		Body:        strings.TrimSpace(release.Body),
		PublishedAt: release.PublishedAt,
		CreatedAt:   release.CreatedAt,
		Assets:      selectedAssets,
	}, nil
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
		fetch: fetchLatestStableReleaseMetadataWithDownloader,
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
var resolveStableReleaseForAssetsFunc = resolveStableReleaseForAssets
var resolveStableReleaseForUpdateFunc = resolveStableReleaseForUpdate

func fetchLatestStableReleaseMetadataWithDownloader(
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

	releaseAPIURL := apiBaseURL + "/releases/latest"
	body, meta, err := dl.ReadAll(ctx, downloader.Request{
		Purpose:       "awgm-update-check",
		URL:           releaseAPIURL,
		Method:        http.MethodGet,
		Timeout:       repoTimeout,
		MaxBodyBytes:  releasesMaxBytes,
		AllowedStatus: []int{http.StatusOK, http.StatusNotFound},
	})
	if err != nil {
		return stableReleaseInfo{}, fmt.Errorf("fetch stable latest release: %w", err)
	}
	if meta.StatusCode == http.StatusNotFound {
		return stableReleaseInfo{}, fmt.Errorf("stable latest release is not published")
	}
	if meta.StatusCode != http.StatusOK {
		return stableReleaseInfo{}, fmt.Errorf("fetch stable latest release: status %d", meta.StatusCode)
	}

	var release githubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return stableReleaseInfo{}, fmt.Errorf("decode stable latest release: %w", err)
	}
	if release.Draft {
		return stableReleaseInfo{}, fmt.Errorf("stable latest release is draft")
	}
	if release.Prerelease {
		return stableReleaseInfo{}, fmt.Errorf("stable latest release is prerelease")
	}

	return stableReleaseInfoFromGitHubRelease(repoURL, releaseAPIURL, release)
}

func (info stableReleaseInfo) publishedDate() string {
	ts := info.PublishedAt
	if ts.IsZero() {
		ts = info.CreatedAt
	}
	if ts.IsZero() {
		return ""
	}
	return ts.UTC().Format("2006-01-02")
}

func fetchHighestStableReleaseWithDownloader(
	ctx context.Context,
	dl Downloader,
	repoURL string,
) (stableReleaseInfo, error) {
	return resolveStableReleaseWithRequiredAssets(ctx, dl, repoURL, func(candidate stableTagCandidate) []string {
		return nil
	})
}

func resolveStableReleaseForAssets(
	ctx context.Context,
	dl Downloader,
	repoURL string,
	requiredAssets []string,
) (stableReleaseInfo, error) {
	return resolveStableReleaseWithRequiredAssets(ctx, dl, repoURL, func(candidate stableTagCandidate) []string {
		return requiredAssets
	})
}

func resolveStableReleaseForUpdate(
	ctx context.Context,
	dl Downloader,
	repoURL string,
	arch string,
) (stableReleaseInfo, string, error) {
	info, err := resolveStableReleaseWithRequiredAssets(ctx, dl, repoURL, func(candidate stableTagCandidate) []string {
		return []string{
			"VERSION",
			"CHANGELOG.md",
			fmt.Sprintf("%s_%s_%s-kn.ipk", pkgName, candidate.Version, arch),
		}
	})
	if err != nil {
		return stableReleaseInfo{}, "", err
	}
	return info, fmt.Sprintf("%s_%s_%s-kn.ipk", pkgName, info.Version, arch), nil
}

func resolveStableReleaseWithRequiredAssets(
	ctx context.Context,
	dl Downloader,
	repoURL string,
	requiredAssetsFor func(candidate stableTagCandidate) []string,
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

	candidates := collectStableTagCandidates(refs)
	if len(candidates) == 0 {
		return stableReleaseInfo{}, fmt.Errorf("no stable tag found in %s", tagsURL)
	}

	var reasons []string
	for _, candidate := range candidates {
		requiredAssets := requiredAssetsFor(candidate)
		info, reason, err := fetchStableReleaseCandidate(ctx, dl, repoURL, apiBaseURL, candidate, requiredAssets)
		if err != nil {
			return stableReleaseInfo{}, err
		}
		if reason != "" {
			reasons = append(reasons, reason)
			continue
		}
		return info, nil
	}

	if len(reasons) == 0 {
		return stableReleaseInfo{}, fmt.Errorf("no published stable release found for %s", repoURL)
	}

	topRequiredAssets := requiredAssetsFor(candidates[0])
	if len(topRequiredAssets) == 0 {
		return stableReleaseInfo{}, fmt.Errorf("no published stable release found for %s: %s", repoURL, strings.Join(reasons, "; "))
	}

	return stableReleaseInfo{}, fmt.Errorf(
		"no stable release with assets [%s] found for %s: %s",
		strings.Join(topRequiredAssets, ", "),
		repoURL,
		strings.Join(reasons, "; "),
	)
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
	candidates := collectStableTagCandidates(refs)
	if len(candidates) == 0 {
		return "", ""
	}
	return candidates[0].TagName, candidates[0].Version
}

func collectStableTagCandidates(refs []githubMatchingRef) []stableTagCandidate {
	candidates := make([]stableTagCandidate, 0, len(refs))
	for _, ref := range refs {
		tagName := strings.TrimSpace(ref.Ref)
		tagName = strings.TrimPrefix(tagName, "refs/tags/")
		normalizedVersion, ok := normalizeStableReleaseTag(tagName)
		if !ok {
			continue
		}
		candidates = append(candidates, stableTagCandidate{
			TagName: tagName,
			Version: normalizedVersion,
		})
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return semver.Compare(candidates[i].Version, candidates[j].Version) > 0
	})

	return candidates
}

func fetchStableReleaseCandidate(
	ctx context.Context,
	dl Downloader,
	repoURL string,
	apiBaseURL string,
	candidate stableTagCandidate,
	requiredAssets []string,
) (stableReleaseInfo, string, error) {
	releaseAPIURL := apiBaseURL + "/releases/tags/" + candidate.TagName
	body, meta, err := dl.ReadAll(ctx, downloader.Request{
		Purpose:       "awgm-update-check",
		URL:           releaseAPIURL,
		Method:        http.MethodGet,
		Timeout:       repoTimeout,
		MaxBodyBytes:  releasesMaxBytes,
		AllowedStatus: []int{http.StatusOK, http.StatusNotFound},
	})
	if err != nil {
		return stableReleaseInfo{}, "", fmt.Errorf("fetch stable release %s: %w", candidate.TagName, err)
	}
	if meta.StatusCode == http.StatusNotFound {
		return stableReleaseInfo{}, fmt.Sprintf("%s: release not published", candidate.TagName), nil
	}
	if meta.StatusCode != http.StatusOK {
		return stableReleaseInfo{}, "", fmt.Errorf("fetch stable release %s: status %d", candidate.TagName, meta.StatusCode)
	}

	var release githubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return stableReleaseInfo{}, "", fmt.Errorf("decode stable release %s: %w", candidate.TagName, err)
	}
	if strings.TrimSpace(release.TagName) != candidate.TagName {
		return stableReleaseInfo{}, fmt.Sprintf("%s: tag mismatch %q", candidate.TagName, strings.TrimSpace(release.TagName)), nil
	}
	if release.Draft {
		return stableReleaseInfo{}, fmt.Sprintf("%s: draft release", candidate.TagName), nil
	}
	if release.Prerelease {
		return stableReleaseInfo{}, fmt.Sprintf("%s: prerelease", candidate.TagName), nil
	}

	selectedAssets := make(map[string]string, len(release.Assets))
	for _, asset := range release.Assets {
		if strings.TrimSpace(asset.Name) == "" || strings.TrimSpace(asset.BrowserDownloadURL) == "" {
			continue
		}
		selectedAssets[asset.Name] = asset.BrowserDownloadURL
	}
	if len(selectedAssets) == 0 {
		return stableReleaseInfo{}, fmt.Sprintf("%s: no assets", candidate.TagName), nil
	}
	if len(requiredAssets) > 0 {
		missingAssets := make([]string, 0, len(requiredAssets))
		for _, assetName := range requiredAssets {
			if strings.TrimSpace(selectedAssets[assetName]) == "" {
				missingAssets = append(missingAssets, assetName)
			}
		}
		if len(missingAssets) > 0 {
			return stableReleaseInfo{}, fmt.Sprintf("%s: missing assets [%s]", candidate.TagName, strings.Join(missingAssets, ", ")), nil
		}
	}

	return stableReleaseInfo{
		RepoURL: repoURL,
		APIURL:  releaseAPIURL,
		TagName: candidate.TagName,
		Version: candidate.Version,
		Assets:  selectedAssets,
	}, "", nil
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

func normalizeStableLatestReleaseTag(tag string) (string, bool) {
	trimmed := strings.TrimSpace(tag)
	if trimmed == "" || strings.EqualFold(trimmed, "iq-latest") {
		return "", false
	}

	lower := strings.ToLower(trimmed)
	for _, marker := range []string{"alpha", "beta", "rc", "dev"} {
		if strings.Contains(lower, marker) {
			return "", false
		}
	}

	if strings.HasPrefix(trimmed, "v") {
		trimmed = strings.TrimPrefix(trimmed, "v")
	}

	if !releaseVersionPattern.MatchString(trimmed) {
		return "", false
	}

	return trimmed, true
}
