package updater

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/downloader"
)

func TestServiceApplyUpgrade_ResetsUpgradingAfterDownloadError(t *testing.T) {
	svc := &Service{
		downloader: &fakeDownloader{
			downloadFileFn: func(context.Context, downloader.FileRequest) (downloader.FileResult, error) {
				return downloader.FileResult{}, errors.New("download failed")
			},
		},
		cached: &UpdateInfo{
			DownloadURL: "http://repo.local/awg-manager_2.12.0_aarch64-3.10-kn.ipk",
			CheckedAt:   time.Now(),
		},
	}

	err1 := svc.ApplyUpgrade(context.Background())
	if err1 == nil || !strings.Contains(err1.Error(), "download IPK") {
		t.Fatalf("first ApplyUpgrade error = %v, want download error", err1)
	}
	err2 := svc.ApplyUpgrade(context.Background())
	if err2 == nil || !strings.Contains(err2.Error(), "download IPK") {
		t.Fatalf("second ApplyUpgrade error = %v, want download error (not ErrUpgradeInProgress)", err2)
	}
}

func TestServiceApplyUpgrade_NoDownloadURLDoesNotSetUpgrading(t *testing.T) {
	svc := &Service{
		downloader: newDefaultDownloader(),
	}

	err1 := svc.ApplyUpgrade(context.Background())
	if err1 == nil || !strings.Contains(err1.Error(), "no download URL") {
		t.Fatalf("first ApplyUpgrade error = %v, want no download URL", err1)
	}
	err2 := svc.ApplyUpgrade(context.Background())
	if err2 == nil || !strings.Contains(err2.Error(), "no download URL") {
		t.Fatalf("second ApplyUpgrade error = %v, want no download URL (not ErrUpgradeInProgress)", err2)
	}
}

func TestServiceApplyUpgrade_ReleaseWithoutChecksumIsBlocked(t *testing.T) {
	svc := &Service{
		downloader: &fakeDownloader{
			downloadFileFn: func(context.Context, downloader.FileRequest) (downloader.FileResult, error) {
				t.Fatal("download should not start without a release checksum")
				return downloader.FileResult{}, nil
			},
		},
		cached: &UpdateInfo{
			Source:      "release",
			DownloadURL: "https://github.com/example/repo/releases/latest/download/awg-manager_2.12.0_aarch64-3.10-kn.ipk",
			CheckedAt:   time.Now(),
		},
	}

	err1 := svc.ApplyUpgrade(context.Background())
	if !errors.Is(err1, ErrReleaseChecksumUnavailable) {
		t.Fatalf("first ApplyUpgrade error = %v, want ErrReleaseChecksumUnavailable", err1)
	}
	err2 := svc.ApplyUpgrade(context.Background())
	if !errors.Is(err2, ErrReleaseChecksumUnavailable) {
		t.Fatalf("second ApplyUpgrade error = %v, want ErrReleaseChecksumUnavailable", err2)
	}
}

func TestServiceApplyUpgrade_EntwareWithoutChecksumStillAttemptsDownload(t *testing.T) {
	downloadCalls := 0
	svc := &Service{
		downloader: &fakeDownloader{
			downloadFileFn: func(context.Context, downloader.FileRequest) (downloader.FileResult, error) {
				downloadCalls++
				return downloader.FileResult{}, errors.New("download failed")
			},
		},
		cached: &UpdateInfo{
			Source:      "entware",
			DownloadURL: "http://repo.local/awg-manager_2.12.0_aarch64-3.10-kn.ipk",
			CheckedAt:   time.Now(),
		},
	}

	err := svc.ApplyUpgrade(context.Background())
	if err == nil || !strings.Contains(err.Error(), "download IPK") {
		t.Fatalf("ApplyUpgrade error = %v, want download error", err)
	}
	if downloadCalls != 1 {
		t.Fatalf("downloadCalls = %d, want 1", downloadCalls)
	}
}
