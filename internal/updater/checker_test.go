package updater

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/downloader"
)

// --- archSuffix sanity check (the function lives in repo.go now) ---

func TestArchSuffix(t *testing.T) {
	got := archSuffix()
	switch runtime.GOARCH {
	case "mipsle":
		if got != "mipsel-3.4" {
			t.Errorf("archSuffix() = %q, want mipsel-3.4", got)
		}
	case "mips":
		if got != "mips-3.4" {
			t.Errorf("archSuffix() = %q, want mips-3.4", got)
		}
	case "arm64":
		if got != "aarch64-3.10" {
			t.Errorf("archSuffix() = %q, want aarch64-3.10", got)
		}
	default:
		if got != runtime.GOARCH {
			t.Errorf("archSuffix() = %q, want %q (fallback)", got, runtime.GOARCH)
		}
	}
}

// --- Check with mock HTTP server returning gzipped Packages ---

// newMockEntwareServer returns an httptest server that serves gzipped Packages
// content for any /<arch>/Packages.gz path. statusCode is the response code
// (use 200 for success cases). packagesContent is the plain (un-gzipped) text
// of the index — gzipBytes is applied here to match the real server.
func newMockEntwareServer(t *testing.T, packagesContent string, statusCode int) *httptest.Server {
	gzData := gzipBytes(t, packagesContent)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(statusCode)
		if statusCode == http.StatusOK {
			w.Write(gzData)
		}
	}))
}

// withMockRepo points entwareRepoURL at srv.URL for the duration of the test.
func withMockRepo(t *testing.T, srv *httptest.Server) {
	t.Helper()
	old := entwareRepoURL
	entwareRepoURL = srv.URL
	t.Cleanup(func() { entwareRepoURL = old })
}

func TestCheck_UpdateAvailable(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
	}()

	releaseRepoURL = ""
	arch := archSuffix()
	ipkName := "awg-manager_9.9.9_" + arch + "-kn.ipk"
	body := "Package: awg-manager\nVersion: 9.9.9\nFilename: " + ipkName + "\n"

	srv := newMockEntwareServer(t, body, http.StatusOK)
	defer srv.Close()
	withMockRepo(t, srv)

	info := Check(context.Background(), "2.0.0")

	if !info.Available {
		t.Fatal("expected Available=true")
	}
	if info.LatestVersion != "9.9.9" {
		t.Errorf("LatestVersion = %q, want 9.9.9", info.LatestVersion)
	}
	wantURL := srv.URL + "/" + archSuffixToRepoDir(arch) + "/" + ipkName
	if info.DownloadURL != wantURL {
		t.Errorf("DownloadURL = %q, want %q", info.DownloadURL, wantURL)
	}
	if info.Error != "" {
		t.Errorf("unexpected error: %s", info.Error)
	}
}

func TestCheck_PicksHighestOfMultipleBlocks(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
	}()

	releaseRepoURL = ""
	arch := archSuffix()
	body := `Package: awg-manager
Version: 2.6.5
Filename: awg-manager_2.6.5_` + arch + `-kn.ipk

Package: awg-manager
Version: 2.7.10
Filename: awg-manager_2.7.10_` + arch + `-kn.ipk

Package: awg-manager
Version: 2.7.3
Filename: awg-manager_2.7.3_` + arch + `-kn.ipk
`
	srv := newMockEntwareServer(t, body, http.StatusOK)
	defer srv.Close()
	withMockRepo(t, srv)

	info := Check(context.Background(), "2.0.0")
	if !info.Available {
		t.Fatal("expected Available=true")
	}
	if info.LatestVersion != "2.7.10" {
		t.Errorf("LatestVersion = %q, want 2.7.10", info.LatestVersion)
	}
}

func TestCheck_AlreadyUpToDate(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
	}()

	releaseRepoURL = ""
	arch := archSuffix()
	body := "Package: awg-manager\nVersion: 2.3.11\nFilename: awg-manager_2.3.11_" + arch + "-kn.ipk\n"

	srv := newMockEntwareServer(t, body, http.StatusOK)
	defer srv.Close()
	withMockRepo(t, srv)

	info := Check(context.Background(), "2.3.11")
	if info.Available {
		t.Fatal("expected Available=false (same version)")
	}
	if info.Error != "" {
		t.Errorf("unexpected error: %s", info.Error)
	}
}

func TestCheck_BuildRevisionSameAsRepoRelease(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
	}()

	releaseRepoURL = ""
	arch := archSuffix()
	body := "Package: awg-manager\nVersion: 2.11.2\nFilename: awg-manager_2.11.2_" + arch + "-kn.ipk\n"

	srv := newMockEntwareServer(t, body, http.StatusOK)
	defer srv.Close()
	withMockRepo(t, srv)

	info := Check(context.Background(), "2.11.2+r70")
	if info.Available {
		t.Fatal("expected Available=false when repo release matches base of build revision")
	}
	if info.LatestVersion != "" {
		t.Errorf("LatestVersion = %q, want empty", info.LatestVersion)
	}
}

func TestCheck_NewerThanRepo(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
	}()

	releaseRepoURL = ""
	arch := archSuffix()
	body := "Package: awg-manager\nVersion: 2.3.10\nFilename: awg-manager_2.3.10_" + arch + "-kn.ipk\n"

	srv := newMockEntwareServer(t, body, http.StatusOK)
	defer srv.Close()
	withMockRepo(t, srv)

	info := Check(context.Background(), "2.3.11")
	if info.Available {
		t.Fatal("expected Available=false (current is newer)")
	}
}

func TestCheck_PackageMissing(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
	}()

	releaseRepoURL = ""
	body := "Package: curl\nVersion: 8.0.1\nFilename: curl_8.0.1.ipk\n"

	srv := newMockEntwareServer(t, body, http.StatusOK)
	defer srv.Close()
	withMockRepo(t, srv)

	info := Check(context.Background(), "2.0.0")
	if info.Available {
		t.Fatal("expected Available=false when package not found in index")
	}
	if info.Error == "" {
		t.Fatal("expected error mentioning missing package")
	}
}

func TestCheck_HTTPError(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
	}()

	releaseRepoURL = ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()
	withMockRepo(t, srv)

	info := Check(context.Background(), "2.0.0")
	if info.Available {
		t.Fatal("expected Available=false on HTTP 500")
	}
	if info.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestCheck_DevelopDetectsNewerRevision(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
	}()

	releaseRepoURL = ""
	releaseBaseURL = ""
	arch := archSuffix()
	archDir := archSuffixToRepoDir(arch)
	ipk := "awg-manager_2.11.2+r71_" + arch + "-kn.ipk"
	packages := "Package: awg-manager\nVersion: 2.11.2+r71\nFilename: " + ipk + "\n"

	var seen downloader.Request
	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			seen = req
			return gzipBytes(t, packages), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
		},
	}

	info := checkWithDownloader(context.Background(), "2.11.2+r70", "develop", dl)

	if !strings.Contains(seen.URL, "/develop/") {
		t.Errorf("request URL %q does not contain /develop/", seen.URL)
	}
	wantSuffix := archDir + "/Packages.gz"
	if !strings.HasSuffix(seen.URL, wantSuffix) {
		t.Errorf("request URL %q does not end with %q", seen.URL, wantSuffix)
	}
	if !info.Available {
		t.Fatal("expected Available=true: r71 > r70 on develop")
	}
	if info.LatestVersion != "2.11.2+r71" {
		t.Errorf("LatestVersion = %q, want 2.11.2+r71", info.LatestVersion)
	}
	wantURL := entwareRepoURL + "/develop/" + archDir + "/" + ipk
	if info.DownloadURL != wantURL {
		t.Errorf("DownloadURL = %q, want %q", info.DownloadURL, wantURL)
	}
}

func TestCheck_DevelopSameRevisionUpToDate(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
	}()

	releaseRepoURL = ""
	releaseBaseURL = ""
	arch := archSuffix()
	archDir := archSuffixToRepoDir(arch)
	ipk := "awg-manager_2.11.2+r70_" + arch + "-kn.ipk"
	packages := "Package: awg-manager\nVersion: 2.11.2+r70\nFilename: " + ipk + "\n"

	var seen downloader.Request
	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			seen = req
			return gzipBytes(t, packages), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
		},
	}

	info := checkWithDownloader(context.Background(), "2.11.2+r70", "develop", dl)

	if !strings.Contains(seen.URL, "/develop/") {
		t.Errorf("request URL %q does not contain /develop/", seen.URL)
	}
	wantSuffix := archDir + "/Packages.gz"
	if !strings.HasSuffix(seen.URL, wantSuffix) {
		t.Errorf("request URL %q does not end with %q", seen.URL, wantSuffix)
	}
	if info.Available {
		t.Fatal("expected Available=false: same revision")
	}
	if info.DownloadURL != "" {
		t.Errorf("DownloadURL = %q, want empty", info.DownloadURL)
	}
}

func TestCheck_StableWithoutReleaseBaseURLUsesPackagesIndex(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
	}()

	releaseRepoURL = ""
	releaseBaseURL = ""

	arch := archSuffix()
	archDir := archSuffixToRepoDir(arch)
	ipkName := "awg-manager_9.9.9_" + arch + "-kn.ipk"
	packages := "Package: awg-manager\nVersion: 9.9.9\nFilename: " + ipkName + "\n"

	var seen downloader.Request
	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			seen = req
			return gzipBytes(t, packages), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
		},
	}

	info := checkWithDownloader(context.Background(), "2.0.0", channelStable, dl)

	if !strings.HasSuffix(seen.URL, archDir+"/Packages.gz") {
		t.Fatalf("request URL = %q, want Packages.gz path", seen.URL)
	}
	if !info.Available {
		t.Fatalf("expected update available, got %+v", info)
	}
}

func TestCheck_StableUsesHighestSemverTagNotLatestDownload(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	oldFetcher := stableReleaseResolver.fetch
	oldTTL := stableReleaseResolver.ttl
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
		stableReleaseResolver.fetch = oldFetcher
		stableReleaseResolver.ttl = oldTTL
		stableReleaseResolver.Clear()
	}()

	releaseRepoURL = "https://github.com/example/repo/releases"
	releaseBaseURL = ""
	arch := archSuffix()

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
				"VERSION":      "https://github.com/example/repo/releases/download/v2.13.0.1/VERSION",
				"CHANGELOG.md": "https://github.com/example/repo/releases/download/v2.13.0.1/CHANGELOG.md",
				"awg-manager_2.13.0.1_" + arch + "-kn.ipk": "https://github.com/example/repo/releases/download/v2.13.0.1/awg-manager_2.13.0.1_" + arch + "-kn.ipk",
			},
		}, nil
	}

	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			if req.URL == "https://github.com/example/repo/releases/download/v2.13.0.1/VERSION" {
				return []byte("2.13.0.1\n"), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
			}
			t.Fatalf("unexpected URL %q", req.URL)
			return nil, downloader.ResponseMeta{}, nil
		},
	}

	info := checkWithDownloader(context.Background(), "2.12.4", channelStable, dl)

	if !info.Available {
		t.Fatalf("expected update available, got %+v", info)
	}
	if info.LatestVersion != "2.13.0.1" {
		t.Fatalf("LatestVersion = %q, want 2.13.0.1", info.LatestVersion)
	}
	if info.SourceURL != "https://api.github.com/repos/example/repo/releases/tags/v2.13.0.1" {
		t.Fatalf("SourceURL = %q", info.SourceURL)
	}
	wantURL := "https://github.com/example/repo/releases/download/v2.13.0.1/awg-manager_2.13.0.1_" + arch + "-kn.ipk"
	if info.DownloadURL != wantURL {
		t.Fatalf("DownloadURL = %q, want %q", info.DownloadURL, wantURL)
	}
}

func TestStableResolver_SelectsHighestGitTagNotReleaseList(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
	}()

	var seen []string
	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			seen = append(seen, req.URL)
			switch req.URL {
			case "https://api.github.com/repos/example/repo/git/matching-refs/tags/v":
				return []byte(`[
					{"ref":"refs/tags/v2.12.4"},
					{"ref":"refs/tags/v2.13.0.1"},
					{"ref":"refs/tags/v2.14.0-beta"},
					{"ref":"refs/tags/iq-latest"}
				]`), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
			case "https://api.github.com/repos/example/repo/releases/tags/v2.13.0.1":
				return []byte(`{
					"tag_name":"v2.13.0.1",
					"draft":false,
					"prerelease":false,
					"assets":[{"name":"CHANGELOG.md","browser_download_url":"https://github.com/example/repo/releases/download/v2.13.0.1/CHANGELOG.md"}]
				}`), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
			default:
				t.Fatalf("unexpected URL %q", req.URL)
				return nil, downloader.ResponseMeta{}, nil
			}
		},
	}

	info, err := fetchHighestStableReleaseWithDownloader(context.Background(), dl, "https://github.com/example/repo/releases")
	if err != nil {
		t.Fatalf("fetchHighestStableReleaseWithDownloader: %v", err)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %v, want 2 calls", seen)
	}
	if seen[0] != "https://api.github.com/repos/example/repo/git/matching-refs/tags/v" {
		t.Fatalf("first request URL = %q", seen[0])
	}
	if seen[1] != "https://api.github.com/repos/example/repo/releases/tags/v2.13.0.1" {
		t.Fatalf("second request URL = %q", seen[1])
	}
	if info.TagName != "v2.13.0.1" {
		t.Fatalf("TagName = %q", info.TagName)
	}
	if info.Version != "2.13.0.1" {
		t.Fatalf("Version = %q", info.Version)
	}
}

func TestStableResolver_IgnoresBareNumericTags(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
	}()

	var seen []string
	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			seen = append(seen, req.URL)
			switch req.URL {
			case "https://api.github.com/repos/example/repo/git/matching-refs/tags/v":
				return []byte(`[
					{"ref":"refs/tags/2.13.0.1"},
					{"ref":"refs/tags/v2.12.9"},
					{"ref":"refs/tags/v2.13.0.1"}
				]`), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
			case "https://api.github.com/repos/example/repo/releases/tags/v2.13.0.1":
				return []byte(`{
					"tag_name":"v2.13.0.1",
					"draft":false,
					"prerelease":false,
					"assets":[
						{"name":"VERSION","browser_download_url":"https://github.com/example/repo/releases/download/v2.13.0.1/VERSION"},
						{"name":"CHANGELOG.md","browser_download_url":"https://github.com/example/repo/releases/download/v2.13.0.1/CHANGELOG.md"},
						{"name":"awg-manager_2.13.0.1_aarch64-3.10-kn.ipk","browser_download_url":"https://github.com/example/repo/releases/download/v2.13.0.1/awg-manager_2.13.0.1_aarch64-3.10-kn.ipk"}
					]
				}`), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
			default:
				t.Fatalf("unexpected URL %q", req.URL)
				return nil, downloader.ResponseMeta{}, nil
			}
		},
	}

	info, err := fetchHighestStableReleaseWithDownloader(context.Background(), dl, "https://github.com/example/repo/releases")
	if err != nil {
		t.Fatalf("fetchHighestStableReleaseWithDownloader: %v", err)
	}
	if len(seen) != 2 {
		t.Fatalf("requests = %v, want 2 calls", seen)
	}
	if seen[1] != "https://api.github.com/repos/example/repo/releases/tags/v2.13.0.1" {
		t.Fatalf("release request URL = %q", seen[1])
	}
	if info.TagName != "v2.13.0.1" {
		t.Fatalf("TagName = %q, want v2.13.0.1", info.TagName)
	}
	if info.Version != "2.13.0.1" {
		t.Fatalf("Version = %q, want 2.13.0.1", info.Version)
	}
}

func TestCheck_DevelopWithReleaseBaseURLUsesVersionAsset(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
	}()

	releaseRepoURL = ""
	releaseBaseURL = "https://example.com/releases/download/iq-latest"
	arch := archSuffix()

	var seen downloader.Request
	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			seen = req
			return []byte("2.11.2+r71\n"), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
		},
	}

	info := checkWithDownloader(context.Background(), "2.11.2+r70", channelDevelop, dl)

	if seen.URL != releaseBaseURL+"/VERSION" {
		t.Fatalf("request URL = %q, want %q", seen.URL, releaseBaseURL+"/VERSION")
	}
	if !info.Available {
		t.Fatalf("expected update available, got %+v", info)
	}
	if info.LatestVersion != "2.11.2+r71" {
		t.Fatalf("LatestVersion = %q, want 2.11.2+r71", info.LatestVersion)
	}
	wantURL := releaseBaseURL + "/awg-manager_2.11.2+r71_" + arch + "-kn.ipk"
	if info.DownloadURL != wantURL {
		t.Fatalf("DownloadURL = %q, want %q", info.DownloadURL, wantURL)
	}
}

func TestCheck_DevelopUsesIqLatestVersionAsset(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
	}()

	releaseRepoURL = "https://github.com/example/repo/releases"
	releaseBaseURL = ""
	var seen downloader.Request
	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			seen = req
			return []byte("2.12.3.14+r1\n"), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
		},
	}

	info := checkWithDownloader(context.Background(), "2.12.4", channelDevelop, dl)

	if seen.URL != "https://github.com/example/repo/releases/download/iq-latest/VERSION" {
		t.Fatalf("request URL = %q", seen.URL)
	}
	if info.Available {
		t.Fatalf("expected no update, got %+v", info)
	}
}

func TestCheck_StableMissingChangelogDoesNotOfferUpdate(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldFetcher := stableReleaseResolver.fetch
	oldTTL := stableReleaseResolver.ttl
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		stableReleaseResolver.fetch = oldFetcher
		stableReleaseResolver.ttl = oldTTL
		stableReleaseResolver.Clear()
	}()

	releaseRepoURL = "https://github.com/example/repo/releases"
	stableReleaseResolver.ttl = time.Hour
	stableReleaseResolver.fetch = func(_ context.Context, _ Downloader, repoURL string) (stableReleaseInfo, error) {
		return stableReleaseInfo{
			RepoURL: repoURL,
			APIURL:  "https://api.github.com/repos/example/repo/releases/tags/v2.13.0.1",
			TagName: "v2.13.0.1",
			Version: "2.13.0.1",
			Assets: map[string]string{
				"VERSION": "https://github.com/example/repo/releases/download/v2.13.0.1/VERSION",
				"awg-manager_2.13.0.1_" + archSuffix() + "-kn.ipk": "https://github.com/example/repo/releases/download/v2.13.0.1/awg-manager_2.13.0.1_" + archSuffix() + "-kn.ipk",
			},
		}, nil
	}

	info := checkWithDownloader(context.Background(), "2.12.4", channelStable, &fakeDownloader{})

	if info.Available {
		t.Fatalf("expected no update, got %+v", info)
	}
	if !strings.Contains(info.Error, "stable release v2.13.0.1 is incomplete: missing CHANGELOG.md") {
		t.Fatalf("error = %q", info.Error)
	}
}

func TestStableResolver_DoesNotFallbackWhenHighestTagReleaseMissing(t *testing.T) {
	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			switch req.URL {
			case "https://api.github.com/repos/example/repo/git/matching-refs/tags/v":
				return []byte(`[
					{"ref":"refs/tags/v2.12.9"},
					{"ref":"refs/tags/v2.13.0.1"}
				]`), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
			case "https://api.github.com/repos/example/repo/releases/tags/v2.13.0.1":
				return nil, downloader.ResponseMeta{StatusCode: http.StatusNotFound}, nil
			default:
				t.Fatalf("unexpected URL %q", req.URL)
				return nil, downloader.ResponseMeta{}, nil
			}
		},
	}

	_, err := fetchHighestStableReleaseWithDownloader(context.Background(), dl, "https://github.com/example/repo/releases")
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got != "stable release v2.13.0.1 is not published" {
		t.Fatalf("error = %q", got)
	}
}

func TestCheck_StableMissingVersionDoesNotOfferUpdate(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldFetcher := stableReleaseResolver.fetch
	oldTTL := stableReleaseResolver.ttl
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		stableReleaseResolver.fetch = oldFetcher
		stableReleaseResolver.ttl = oldTTL
		stableReleaseResolver.Clear()
	}()

	releaseRepoURL = "https://github.com/example/repo/releases"
	stableReleaseResolver.ttl = time.Hour
	stableReleaseResolver.fetch = func(_ context.Context, _ Downloader, repoURL string) (stableReleaseInfo, error) {
		return stableReleaseInfo{
			RepoURL: repoURL,
			APIURL:  "https://api.github.com/repos/example/repo/releases/tags/v2.13.0.1",
			TagName: "v2.13.0.1",
			Version: "2.13.0.1",
			Assets: map[string]string{
				"CHANGELOG.md": "https://github.com/example/repo/releases/download/v2.13.0.1/CHANGELOG.md",
				"awg-manager_2.13.0.1_" + archSuffix() + "-kn.ipk": "https://github.com/example/repo/releases/download/v2.13.0.1/awg-manager_2.13.0.1_" + archSuffix() + "-kn.ipk",
			},
		}, nil
	}

	info := checkWithDownloader(context.Background(), "2.12.4", channelStable, &fakeDownloader{})

	if info.Available {
		t.Fatalf("expected no update, got %+v", info)
	}
	if !strings.Contains(info.Error, "stable release v2.13.0.1 is incomplete: missing VERSION") {
		t.Fatalf("error = %q", info.Error)
	}
}

func TestCheck_StableVersionAssetMismatchReturnsError(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldFetcher := stableReleaseResolver.fetch
	oldTTL := stableReleaseResolver.ttl
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		stableReleaseResolver.fetch = oldFetcher
		stableReleaseResolver.ttl = oldTTL
		stableReleaseResolver.Clear()
	}()

	releaseRepoURL = "https://github.com/example/repo/releases"
	stableReleaseResolver.ttl = time.Hour
	stableReleaseResolver.fetch = func(_ context.Context, _ Downloader, repoURL string) (stableReleaseInfo, error) {
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
	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			if strings.HasSuffix(req.URL, "/VERSION") {
				return []byte("2.13.0.9\n"), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
			}
			t.Fatalf("unexpected URL %q", req.URL)
			return nil, downloader.ResponseMeta{}, nil
		},
	}

	info := checkWithDownloader(context.Background(), "2.12.4", channelStable, dl)

	if info.Available {
		t.Fatalf("expected no update, got %+v", info)
	}
	if !strings.Contains(info.Error, `VERSION asset mismatch in v2.13.0.1: got "2.13.0.9" want "2.13.0.1"`) {
		t.Fatalf("error = %q", info.Error)
	}
}

func TestCheck_DevelopMissingVersionDoesNotLeakHTML(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
	}()

	releaseRepoURL = "https://github.com/example/repo/releases"
	releaseBaseURL = ""
	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			return nil, downloader.ResponseMeta{}, fmt.Errorf("download via direct: status 404: <!DOCTYPE html><html><head>githubassets</head></html>")
		},
	}

	info := checkWithDownloader(context.Background(), "2.12.4", channelDevelop, dl)

	if info.Available {
		t.Fatalf("expected no update, got %+v", info)
	}
	if !strings.Contains(info.Error, "release asset VERSION not found in iq-latest") {
		t.Fatalf("error = %q", info.Error)
	}
	if strings.Contains(strings.ToLower(info.Error), "<!doctype html") || strings.Contains(strings.ToLower(info.Error), "githubassets") {
		t.Fatalf("error leaks html: %q", info.Error)
	}
}

func TestCheck_StableCurrentDevelopBuildSeesHigherStableTag(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldFetcher := stableReleaseResolver.fetch
	oldTTL := stableReleaseResolver.ttl
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		stableReleaseResolver.fetch = oldFetcher
		stableReleaseResolver.ttl = oldTTL
		stableReleaseResolver.Clear()
	}()

	releaseRepoURL = "https://github.com/example/repo/releases"
	stableReleaseResolver.ttl = time.Hour
	stableReleaseResolver.fetch = func(_ context.Context, _ Downloader, repoURL string) (stableReleaseInfo, error) {
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
	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			if strings.HasSuffix(req.URL, "/VERSION") {
				return []byte("2.13.0.1\n"), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
			}
			t.Fatalf("unexpected URL %q", req.URL)
			return nil, downloader.ResponseMeta{}, nil
		},
	}

	info := checkWithDownloader(context.Background(), "2.12.9+r1", channelStable, dl)

	if !info.Available {
		t.Fatalf("expected update available, got %+v", info)
	}
	if info.LatestVersion != "2.13.0.1" {
		t.Fatalf("LatestVersion = %q", info.LatestVersion)
	}
}

func TestCheck_DevelopWithReleaseBaseURLSameRevisionUpToDate(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
	}()

	releaseRepoURL = ""
	releaseBaseURL = "https://example.com/releases/download/iq-latest"

	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			return []byte("2.11.2+r70\n"), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
		},
	}

	info := checkWithDownloader(context.Background(), "2.11.2+r70", channelDevelop, dl)

	if info.Available {
		t.Fatal("expected Available=false: same revision")
	}
	if info.DownloadURL != "" {
		t.Errorf("DownloadURL = %q, want empty", info.DownloadURL)
	}
}

func TestCheck_ReleaseChannelsAcceptForkFourSegmentVersion(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
	}()

	arch := archSuffix()

	for _, tc := range []struct {
		channel string
		baseURL string
	}{
		{channel: channelDevelop, baseURL: "https://github.com/example/repo/releases/download/iq-latest"},
	} {
		var seen downloader.Request
		dl := &fakeDownloader{
			readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
				seen = req
				return []byte("2.12.3.4+r1\n"), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
			},
		}

		releaseRepoURL = "https://github.com/example/repo/releases"
		releaseBaseURL = ""
		info := checkWithDownloader(context.Background(), "2.12.3.3.1", tc.channel, dl)

		if seen.URL != tc.baseURL+"/VERSION" {
			t.Fatalf("request URL = %q, want %q", seen.URL, tc.baseURL+"/VERSION")
		}
		if !info.Available {
			t.Fatalf("expected update available, got %+v", info)
		}
		if info.LatestVersion != "2.12.3.4+r1" {
			t.Fatalf("LatestVersion = %q, want 2.12.3.4+r1", info.LatestVersion)
		}
		wantURL := tc.baseURL + "/awg-manager_2.12.3.4+r1_" + arch + "-kn.ipk"
		if info.DownloadURL != wantURL {
			t.Fatalf("DownloadURL = %q, want %q", info.DownloadURL, wantURL)
		}
	}
}

func TestFetchHighestStableReleaseWithDownloader_RejectsWhenNoStableReleaseExists(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
	}()

	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			return []byte(`[
				{"ref":"refs/tags/iq-latest"},
				{"ref":"refs/tags/v2.14.0-beta"}
			]`), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
		},
	}

	_, err := fetchHighestStableReleaseWithDownloader(context.Background(), dl, "https://github.com/example/repo/releases")
	if err == nil {
		t.Fatal("expected error when no stable releases exist")
	}
	if !strings.Contains(err.Error(), "no stable tag found") {
		t.Fatalf("error = %q", err)
	}
}

func TestCheck_StableIgnoresLowerDevelopRevisionFromReleaseChannel(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	oldFetcher := stableReleaseResolver.fetch
	oldTTL := stableReleaseResolver.ttl
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
		stableReleaseResolver.fetch = oldFetcher
		stableReleaseResolver.ttl = oldTTL
		stableReleaseResolver.Clear()
	}()

	releaseRepoURL = "https://github.com/example/repo/releases"
	releaseBaseURL = ""
	stableReleaseResolver.ttl = time.Hour
	stableReleaseResolver.fetch = func(_ context.Context, _ Downloader, repoURL string) (stableReleaseInfo, error) {
		return stableReleaseInfo{
			RepoURL: repoURL,
			APIURL:  "https://api.github.com/repos/example/repo/releases/tags/v2.12.3.14",
			TagName: "v2.12.3.14",
			Version: "2.12.3.14",
			Assets: map[string]string{
				"VERSION":      "https://github.com/example/repo/releases/download/v2.12.3.14/VERSION",
				"CHANGELOG.md": "https://github.com/example/repo/releases/download/v2.12.3.14/CHANGELOG.md",
			},
		}, nil
	}

	info := checkWithDownloader(context.Background(), "2.12.4", channelStable, nil)

	if info.Available {
		t.Fatalf("expected no update, got %+v", info)
	}
	if info.LatestVersion != "" {
		t.Fatalf("LatestVersion = %q, want empty", info.LatestVersion)
	}
}
