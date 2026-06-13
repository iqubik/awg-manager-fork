package updater

import (
	"context"
	"net/http"
	"strings"
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

	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			if req.URL != "https://github.com/example/repo/releases/latest/download/CHANGELOG.md" {
				t.Fatalf("unexpected URL %q", req.URL)
			}
			return nil, downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
		},
	}

	primary, secondary, err := resolveChangelogSourcesForChannel(context.Background(), dl, channelStable)
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

func TestStableManualRelease_ChangelogUsesAllReleaseBodiesWhenNoChangelogAsset(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	releaseRepoURL = "https://github.com/example/repo/releases"
	releaseBaseURL = ""
	stableReleaseResolver.Clear()
	t.Cleanup(func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		stableReleaseResolver.Clear()
	})

	var seen []string
	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			seen = append(seen, req.URL)
			switch req.URL {
			case "https://github.com/example/repo/releases/latest/download/CHANGELOG.md":
				return nil, downloader.ResponseMeta{StatusCode: http.StatusNotFound}, nil
			case "https://api.github.com/repos/example/repo/releases/latest":
				return []byte(`{
					"tag_name":"v2.13.0.1",
					"html_url":"https://github.com/example/repo/releases/tag/v2.13.0.1",
					"body":"AWG Manager 2.13.0.1\n\n- Fixed stable detection\n- Added manual release fallback",
					"published_at":"2026-06-12T10:00:00Z",
					"assets":[
						{"name":"awg-manager_2.13.0.1_aarch64-3.10-kn.ipk","browser_download_url":"https://github.com/example/repo/releases/download/v2.13.0.1/awg-manager_2.13.0.1_aarch64-3.10-kn.ipk"}
					]
				}`), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
			case "https://api.github.com/repos/example/repo/releases?per_page=100":
				return []byte(`[
					{
						"tag_name":"v2.13.0.1",
						"html_url":"https://github.com/example/repo/releases/tag/v2.13.0.1",
						"body":"AWG Manager 2.13.0.1\n\n- Fixed stable detection\n- Added manual release fallback",
						"published_at":"2026-06-12T10:00:00Z",
						"assets":[
							{"name":"awg-manager_2.13.0.1_aarch64-3.10-kn.ipk","browser_download_url":"https://github.com/example/repo/releases/download/v2.13.0.1/awg-manager_2.13.0.1_aarch64-3.10-kn.ipk"}
						]
					},
					{
						"tag_name":"2.12.9",
						"html_url":"https://github.com/example/repo/releases/tag/2.12.9",
						"body":"AWG Manager 2.12.9\n\n- Previous stable fixes",
						"published_at":"2026-05-30T10:00:00Z",
						"assets":[
							{"name":"awg-manager_2.12.9_aarch64-3.10-kn.ipk","browser_download_url":"https://github.com/example/repo/releases/download/2.12.9/awg-manager_2.12.9_aarch64-3.10-kn.ipk"}
						]
					}
				]`), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
			default:
				t.Fatalf("unexpected URL %q", req.URL)
				return nil, downloader.ResponseMeta{}, nil
			}
		},
	}

	svc := &Service{
		version:    "2.13.0.1",
		appLog:     logging.NewScopedLogger(nil, "", ""),
		downloader: dl,
		changelog:  newChangelogFetcher("", "", time.Minute, dl),
	}

	entry, err := svc.GetChangelogSingle(context.Background(), "2.13.0.1")
	if err != nil {
		t.Fatalf("GetChangelogSingle: %v", err)
	}
	if entry == nil {
		t.Fatal("expected changelog entry")
	}
	if entry.Version != "2.13.0.1" {
		t.Fatalf("Version = %q", entry.Version)
	}
	if entry.Date != "2026-06-12" {
		t.Fatalf("Date = %q", entry.Date)
	}
	if len(entry.Groups) != 1 || entry.Groups[0].Heading != "" {
		t.Fatalf("Groups = %+v", entry.Groups)
	}
	if len(entry.Groups[0].Items) != 2 {
		t.Fatalf("Items = %+v", entry.Groups[0].Items)
	}
	if entry.Groups[0].Items[0] != "AWG Manager 2.13.0.1" {
		t.Fatalf("first item = %q", entry.Groups[0].Items[0])
	}
	if entry.Groups[0].Items[1] != "- Fixed stable detection\n- Added manual release fallback" {
		t.Fatalf("second item = %q", entry.Groups[0].Items[1])
	}

	entries, err := svc.changelog.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %+v", entries)
	}
	if _, ok := entries["2.12.9"]; !ok {
		t.Fatalf("expected 2.12.9 entry, got %+v", entries)
	}
	if primary := svc.changelog.primary(); primary != "https://github.com/example/repo/releases/tag/v2.13.0.1" {
		t.Fatalf("primary = %q", primary)
	}
	for _, want := range []string{
		"https://github.com/example/repo/releases/latest/download/CHANGELOG.md",
		"https://api.github.com/repos/example/repo/releases/latest",
		"https://api.github.com/repos/example/repo/releases?per_page=100",
	} {
		found := false
		for _, url := range seen {
			if url == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected request %q in %v", want, seen)
		}
	}
	for _, url := range seen {
		if strings.Contains(url, "/git/matching-refs/tags/v") {
			t.Fatalf("unexpected tag scan URL %q", url)
		}
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
