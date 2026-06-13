package updater

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/downloader"
)

func TestServiceCheckNow_StableUsesDirectLatestWithoutResolver(t *testing.T) {
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
			APIURL:  "https://api.github.com/repos/example/repo/releases/tags/v2.13.0.1",
			TagName: "v2.13.0.1",
			Version: "2.13.0.1",
			Assets: map[string]string{
				"VERSION":      "https://github.com/example/repo/releases/download/v2.13.0.1/VERSION",
				"CHANGELOG.md": "https://github.com/example/repo/releases/download/v2.13.0.1/CHANGELOG.md",
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

	// Seed cache with stale-but-valid data; direct latest path should ignore it.
	stableReleaseResolver.store(stableReleaseInfo{
		RepoURL: "https://github.com/example/repo/releases",
		APIURL:  "https://api.github.com/repos/example/repo/releases/tags/v2.13.0.1",
		TagName: "v2.12.4",
		Version: "2.12.4",
		Assets: map[string]string{
			"VERSION":      "https://github.com/example/repo/releases/download/v2.12.4/VERSION",
			"CHANGELOG.md": "https://github.com/example/repo/releases/download/v2.12.4/CHANGELOG.md",
			"awg-manager_2.12.4_" + archSuffix() + "-kn.ipk": "https://github.com/example/repo/releases/download/v2.12.4/awg-manager_2.12.4_" + archSuffix() + "-kn.ipk",
		},
	})

	svc := &Service{
		version: "2.12.4",
		downloader: &fakeDownloader{
			readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
				switch req.URL {
				case "https://github.com/example/repo/releases/latest/download/VERSION":
					return []byte("2.13.0.1\n"), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
				case "https://github.com/example/repo/releases/latest/download/CHANGELOG.md":
					return []byte("## [2.13.0.1] - 2026-06-12\n\n### Fixed\n- item\n"), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
				case "https://github.com/example/repo/releases/latest/download/awg-manager_2.13.0.1_" + archSuffix() + "-kn.ipk":
					return nil, downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
				default:
					t.Fatalf("unexpected URL %q", req.URL)
					return nil, downloader.ResponseMeta{}, nil
				}
			},
		},
		changelog: newChangelogFetcher("http://example.test/CHANGELOG.md", "", time.Minute, &fakeDownloader{}),
	}

	info := svc.CheckNow(context.Background())

	if fetchCalls != 0 {
		t.Fatalf("fetchCalls = %d, want 0", fetchCalls)
	}
	if !info.Available {
		t.Fatalf("expected update available, got %+v", info)
	}
	if info.LatestVersion != "2.13.0.1" {
		t.Fatalf("LatestVersion = %q, want 2.13.0.1", info.LatestVersion)
	}
}
