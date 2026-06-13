package updater

import (
	"context"
	"testing"
	"time"
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

func TestChangelogSources_StableUsesHighestGitTagReleaseAsset(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldFetcher := stableReleaseResolver.fetch
	oldTTL := stableReleaseResolver.ttl
	stableReleaseResolver.Clear()
	releaseRepoURL = "https://github.com/example/repo/releases"
	releaseBaseURL = ""
	stableReleaseResolver.ttl = time.Hour
	stableReleaseResolver.fetch = func(_ context.Context, _ Downloader, repoURL string) (stableReleaseInfo, error) {
		if repoURL != "https://github.com/example/repo/releases" {
			t.Fatalf("repoURL = %q", repoURL)
		}
		return stableReleaseInfo{
			RepoURL: repoURL,
			APIURL:  "https://api.github.com/repos/example/repo/releases/tags/v2.13.0.1",
			TagName: "v2.13.0.1",
			Version: "2.13.0.1",
			Assets: map[string]string{
				"CHANGELOG.md": "https://github.com/example/repo/releases/download/v2.13.0.1/CHANGELOG.md",
			},
		}, nil
	}
	t.Cleanup(func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		stableReleaseResolver.fetch = oldFetcher
		stableReleaseResolver.ttl = oldTTL
		stableReleaseResolver.Clear()
	})

	primary, secondary, err := resolveChangelogSourcesForChannel(context.Background(), nil, channelStable)
	if err != nil {
		t.Fatalf("resolveChangelogSourcesForChannel: %v", err)
	}
	if primary != "https://github.com/example/repo/releases/download/v2.13.0.1/CHANGELOG.md" {
		t.Fatalf("stable primary = %q", primary)
	}
	if secondary != "" {
		t.Fatalf("stable secondary = %q, want empty", secondary)
	}
}

func TestResolveChangelogSources_StableMissingAssetReturnsShortError(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldFetcher := stableReleaseResolver.fetch
	oldTTL := stableReleaseResolver.ttl
	stableReleaseResolver.Clear()
	releaseRepoURL = "https://github.com/example/repo/releases"
	releaseBaseURL = ""
	stableReleaseResolver.ttl = time.Hour
	stableReleaseResolver.fetch = func(_ context.Context, _ Downloader, repoURL string) (stableReleaseInfo, error) {
		return stableReleaseInfo{
			RepoURL: repoURL,
			APIURL:  "https://api.github.com/repos/example/repo/releases/tags/v2.13.0.1",
			TagName: "v2.13.0.1",
			Version: "2.13.0.1",
			Assets:  map[string]string{},
		}, nil
	}
	t.Cleanup(func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		stableReleaseResolver.fetch = oldFetcher
		stableReleaseResolver.ttl = oldTTL
		stableReleaseResolver.Clear()
	})

	_, _, err := resolveChangelogSourcesForChannel(context.Background(), nil, channelStable)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got != "missing asset CHANGELOG.md in v2.13.0.1" {
		t.Fatalf("error = %q", got)
	}
}
