package updater

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/downloader"
	"github.com/hoaxisr/awg-manager/internal/logging"
)

func TestChangelogSourcesForChannel_DefaultsToUpstream(t *testing.T) {
	oldEntwareRepoURL := entwareRepoURL
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	entwareRepoURL = "http://example.test"
	releaseRepoURL = ""
	releaseBaseURL = ""
	t.Cleanup(func() {
		entwareRepoURL = oldEntwareRepoURL
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
	})

	cases := []struct {
		channel          string
		wantPrimaryURL   string
		wantSecondaryURL string
	}{
		{channel: "stable", wantPrimaryURL: "http://example.test/CHANGELOG.md"},
		{channel: "develop", wantPrimaryURL: "http://example.test/develop/CHANGELOG.md"},
		{channel: "", wantPrimaryURL: "http://example.test/CHANGELOG.md"},
	}

	for _, tc := range cases {
		primary, secondary := changelogSourcesForChannel(tc.channel)
		if primary != tc.wantPrimaryURL || secondary != tc.wantSecondaryURL {
			t.Fatalf("changelogSourcesForChannel(%q) = (%q, %q), want (%q, %q)", tc.channel, primary, secondary, tc.wantPrimaryURL, tc.wantSecondaryURL)
		}
	}
}

func TestChangelogSourcesForChannel_DevelopUsesIqLatestAsset(t *testing.T) {
	oldEntwareRepoURL := entwareRepoURL
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	entwareRepoURL = "http://example.test"
	releaseRepoURL = "https://github.com/example/repo/releases"
	releaseBaseURL = ""
	t.Cleanup(func() {
		entwareRepoURL = oldEntwareRepoURL
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
	})

	primary, secondary := changelogSourcesForChannel("develop")
	if primary != "https://github.com/example/repo/releases/download/iq-latest/CHANGELOG.md" {
		t.Fatalf("develop primary = %q", primary)
	}
	if secondary != "" {
		t.Fatalf("develop secondary = %q, want empty", secondary)
	}
}

func TestChangelogSourcesForChannel_LegacyReleaseBaseStaysOnIqLatest(t *testing.T) {
	oldEntwareRepoURL := entwareRepoURL
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	entwareRepoURL = "http://example.test"
	releaseRepoURL = ""
	releaseBaseURL = "https://github.example/releases/download/iq-latest"
	t.Cleanup(func() {
		entwareRepoURL = oldEntwareRepoURL
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
	})

	primary, _ := changelogSourcesForChannel("stable")
	if primary != "http://example.test/CHANGELOG.md" {
		t.Fatalf("stable primary = %q", primary)
	}

	primary, _ = changelogSourcesForChannel("develop")
	if primary != "https://github.example/releases/download/iq-latest/CHANGELOG.md" {
		t.Fatalf("develop primary = %q", primary)
	}
}

func TestChangelogSources_StableUsesLatestReleaseAsset(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	releaseRepoURL = "https://github.com/example/repo/releases"
	releaseBaseURL = ""
	t.Cleanup(func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
	})

	primary, secondary, err := resolveChangelogSourcesForChannel(context.Background(), nil, channelStable)
	if err != nil {
		t.Fatalf("resolveChangelogSourcesForChannel: %v", err)
	}
	if primary != "https://github.com/example/repo/releases/latest/download/CHANGELOG.md" {
		t.Fatalf("stable primary = %q", primary)
	}
	if secondary != "" {
		t.Fatalf("stable secondary = %q, want empty", secondary)
	}
}

func TestGetChangelogMinor_StableSourceDoesNotDependOnRevisionSuffix(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	releaseRepoURL = "https://github.com/example/repo/releases"
	releaseBaseURL = ""
	t.Cleanup(func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
	})

	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			if req.URL != "https://github.com/example/repo/releases/latest/download/CHANGELOG.md" {
				t.Fatalf("unexpected URL %q", req.URL)
			}
			return []byte("## [2.12.10] - 2026-06-12\n\n### Fixed\n- item\n"), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
		},
	}

	svc := &Service{
		version:    "2.12.10+r1",
		appLog:     logging.NewScopedLogger(nil, "", ""),
		downloader: dl,
		changelog:  newChangelogFetcher("", "", time.Minute, dl),
	}

	if _, err := svc.GetChangelogMinor(context.Background(), "2.12.10+r1"); err != nil {
		t.Fatalf("GetChangelogMinor(+r1): %v", err)
	}
	if _, err := svc.GetChangelogMinor(context.Background(), "2.12.10"); err != nil {
		t.Fatalf("GetChangelogMinor(base): %v", err)
	}
	if primary := svc.changelog.primary(); primary != "https://github.com/example/repo/releases/latest/download/CHANGELOG.md" {
		t.Fatalf("primary = %q", primary)
	}
}
