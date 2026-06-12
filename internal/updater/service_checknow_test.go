package updater

import (
	"context"
	"testing"
	"time"
)

func TestServiceCheckNow_ClearsStableReleaseResolverCache(t *testing.T) {
	oldFetcher := stableReleaseResolver.fetch
	oldTTL := stableReleaseResolver.ttl
	oldRepoURL := releaseRepoURL
	oldBaseURL := releaseBaseURL
	stableReleaseResolver.Clear()
	releaseRepoURL = "https://github.com/example/repo/releases"
	releaseBaseURL = ""

	fetchCalls := 0
	stableReleaseResolver.ttl = time.Hour
	stableReleaseResolver.fetch = func(_ context.Context, _ Downloader, repoURL string) (stableReleaseInfo, error) {
		fetchCalls++
		return stableReleaseInfo{
			RepoURL: repoURL,
			APIURL:  "https://api.github.com/repos/example/repo/releases?per_page=100",
			TagName: "v2.13.0.1",
			Version: "2.13.0.1",
			Assets: map[string]string{
				"awg-manager_2.13.0.1_" + archSuffix() + "-kn.ipk": "https://github.com/example/repo/releases/download/v2.13.0.1/awg-manager_2.13.0.1_" + archSuffix() + "-kn.ipk",
			},
		}, nil
	}

	t.Cleanup(func() {
		stableReleaseResolver.fetch = oldFetcher
		stableReleaseResolver.ttl = oldTTL
		stableReleaseResolver.Clear()
		releaseRepoURL = oldRepoURL
		releaseBaseURL = oldBaseURL
	})

	// Seed cache with stale-but-valid data; CheckNow must still force resolver refresh.
	stableReleaseResolver.store(stableReleaseInfo{
		RepoURL: "https://github.com/example/repo/releases",
		APIURL:  "https://api.github.com/repos/example/repo/releases?per_page=100",
		TagName: "v2.12.4",
		Version: "2.12.4",
		Assets: map[string]string{
			"awg-manager_2.12.4_" + archSuffix() + "-kn.ipk": "https://github.com/example/repo/releases/download/v2.12.4/awg-manager_2.12.4_" + archSuffix() + "-kn.ipk",
		},
	})

	svc := &Service{
		version:    "2.12.4",
		downloader: &fakeDownloader{},
		changelog:  newChangelogFetcher("http://example.test/CHANGELOG.md", "", time.Minute, &fakeDownloader{}),
	}

	info := svc.CheckNow(context.Background())

	if fetchCalls != 1 {
		t.Fatalf("fetchCalls = %d, want 1", fetchCalls)
	}
	if !info.Available {
		t.Fatalf("expected update available, got %+v", info)
	}
	if info.LatestVersion != "2.13.0.1" {
		t.Fatalf("LatestVersion = %q, want 2.13.0.1", info.LatestVersion)
	}
}
