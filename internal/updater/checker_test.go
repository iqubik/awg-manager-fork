package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

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

func TestCheck_StableWithReleaseRepoURLUsesLatestReleaseVersionAsset(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
	}()

	releaseRepoURL = "https://example.com/releases"
	releaseBaseURL = ""
	arch := archSuffix()

	var seen downloader.Request
	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			seen = req
			return []byte("2.13.0.1\n"), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
		},
	}

	info := checkWithDownloader(context.Background(), "2.12.4", channelStable, dl)

	if seen.URL != "https://example.com/releases/latest/download/VERSION" {
		t.Fatalf("request URL = %q", seen.URL)
	}
	if !info.Available {
		t.Fatalf("expected update available, got %+v", info)
	}
	if info.LatestVersion != "2.13.0.1" {
		t.Fatalf("LatestVersion = %q, want 2.13.0.1", info.LatestVersion)
	}
	wantURL := "https://example.com/releases/latest/download/awg-manager_2.13.0.1_" + arch + "-kn.ipk"
	if info.DownloadURL != wantURL {
		t.Fatalf("DownloadURL = %q, want %q", info.DownloadURL, wantURL)
	}
}

func TestCheck_StableWithReleaseRepoURLRejectsInvalidVersion(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
	}()

	releaseRepoURL = "https://example.com/releases"
	releaseBaseURL = ""
	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			return []byte("2.11.6 bad\n"), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
		},
	}

	info := checkWithDownloader(context.Background(), "2.12.4", channelStable, dl)

	if info.Available {
		t.Fatalf("expected no update on invalid VERSION, got %+v", info)
	}
	if !strings.Contains(info.Error, "invalid VERSION") {
		t.Fatalf("error = %q, want invalid VERSION", info.Error)
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
		{channel: channelStable, baseURL: "https://example.com/releases/latest/download"},
		{channel: channelDevelop, baseURL: "https://example.com/releases/download/iq-latest"},
	} {
		var seen downloader.Request
		dl := &fakeDownloader{
			readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
				seen = req
				return []byte("2.12.3.4+r1\n"), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
			},
		}

		releaseRepoURL = "https://example.com/releases"
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

func TestCheck_StableWithReleaseRepoURLRejectsNonSemverVersion(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
	}()

	releaseRepoURL = "https://example.com/releases"
	releaseBaseURL = ""
	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			return []byte("abc\n"), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
		},
	}

	info := checkWithDownloader(context.Background(), "2.12.4", channelStable, dl)

	if info.Available {
		t.Fatalf("expected no update on non-semver VERSION, got %+v", info)
	}
	if !strings.Contains(info.Error, "invalid VERSION") {
		t.Fatalf("error = %q, want invalid VERSION", info.Error)
	}
}

func TestCheck_StableIgnoresLowerDevelopRevisionFromReleaseChannel(t *testing.T) {
	oldReleaseRepoURL := releaseRepoURL
	oldReleaseBaseURL := releaseBaseURL
	oldEntwareRepoURL := entwareRepoURL
	defer func() {
		releaseRepoURL = oldReleaseRepoURL
		releaseBaseURL = oldReleaseBaseURL
		entwareRepoURL = oldEntwareRepoURL
	}()

	releaseRepoURL = "https://example.com/releases"
	releaseBaseURL = ""

	dl := &fakeDownloader{
		readAllFn: func(_ context.Context, req downloader.Request) ([]byte, downloader.ResponseMeta, error) {
			if req.URL != "https://example.com/releases/latest/download/VERSION" {
				t.Fatalf("request URL = %q", req.URL)
			}
			return []byte("2.12.3.14+r1\n"), downloader.ResponseMeta{StatusCode: http.StatusOK}, nil
		},
	}

	info := checkWithDownloader(context.Background(), "2.12.4", channelStable, dl)

	if info.Available {
		t.Fatalf("expected no update, got %+v", info)
	}
	if info.LatestVersion != "" {
		t.Fatalf("LatestVersion = %q, want empty", info.LatestVersion)
	}
}
