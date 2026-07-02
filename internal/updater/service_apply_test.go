package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
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

func TestServiceApplyUpgrade_ReleaseWithoutChecksumStillAttemptsDownload(t *testing.T) {
	downloadCalls := 0
	oldStart := startDetachedUpgrade
	startDetachedUpgrade = func(_ string) error { return nil }
	t.Cleanup(func() { startDetachedUpgrade = oldStart })

	requireDownloadDir(t)

	svc := &Service{
		downloader: &fakeDownloader{
			downloadFileFn: func(_ context.Context, req downloader.FileRequest) (downloader.FileResult, error) {
				downloadCalls++
				if err := os.WriteFile(req.DestPath, []byte("awg-manager-ipk"), 0o644); err != nil {
					return downloader.FileResult{}, err
				}
				return downloader.FileResult{Path: req.DestPath, Size: int64(len("awg-manager-ipk"))}, nil
			},
		},
		cached: &UpdateInfo{
			Source:      "release",
			DownloadURL: "https://github.com/iqubik/awg-manager-fork/releases/latest/download/awg-manager_2.12.0_aarch64-3.10-kn.ipk",
			CheckedAt:   time.Now(),
		},
	}

	err := svc.ApplyUpgrade(context.Background())
	if err != nil {
		t.Fatalf("ApplyUpgrade error = %v, want nil (release without checksum must proceed)", err)
	}
	if downloadCalls != 1 {
		t.Fatalf("downloadCalls = %d, want 1", downloadCalls)
	}
}

func TestServiceApplyUpgrade_ReleaseWithMatchingChecksumSucceeds(t *testing.T) {
	requireDownloadDir(t)

	payload := []byte("awg-manager-ipk")
	sum := sha256.Sum256(payload)
	wantChecksum := hex.EncodeToString(sum[:])
	downloadCalls := 0
	oldStart := startDetachedUpgrade
	startDetachedUpgrade = func(_ string) error { return nil }
	t.Cleanup(func() { startDetachedUpgrade = oldStart })

	svc := &Service{
		downloader: &fakeDownloader{
			downloadFileFn: func(_ context.Context, req downloader.FileRequest) (downloader.FileResult, error) {
				downloadCalls++
				if err := os.WriteFile(req.DestPath, payload, 0o644); err != nil {
					return downloader.FileResult{}, err
				}
				return downloader.FileResult{Path: req.DestPath, Size: int64(len(payload))}, nil
			},
		},
		cached: &UpdateInfo{
			Source:      "release",
			DownloadURL: "https://github.com/iqubik/awg-manager-fork/releases/latest/download/awg-manager_2.12.0_aarch64-3.10-kn.ipk",
			SHA256:      wantChecksum,
			CheckedAt:   time.Now(),
		},
	}

	if err := svc.ApplyUpgrade(context.Background()); err != nil {
		t.Fatalf("ApplyUpgrade error = %v, want nil (checksum matched)", err)
	}
	if downloadCalls != 1 {
		t.Fatalf("downloadCalls = %d, want 1", downloadCalls)
	}
}

func TestServiceApplyUpgrade_ReleaseWithMismatchingChecksumFails(t *testing.T) {
	requireDownloadDir(t)

	downloadCalls := 0
	svc := &Service{
		downloader: &fakeDownloader{
			downloadFileFn: func(_ context.Context, req downloader.FileRequest) (downloader.FileResult, error) {
				downloadCalls++
				if err := os.WriteFile(req.DestPath, []byte("awg-manager-ipk"), 0o644); err != nil {
					return downloader.FileResult{}, err
				}
				return downloader.FileResult{Path: req.DestPath, Size: 16}, nil
			},
		},
		cached: &UpdateInfo{
			Source:      "release",
			DownloadURL: "https://github.com/iqubik/awg-manager-fork/releases/latest/download/awg-manager_2.12.0_aarch64-3.10-kn.ipk",
			SHA256:      "deadbeef" + strings.Repeat("0", 56),
			CheckedAt:   time.Now(),
		},
	}

	err := svc.ApplyUpgrade(context.Background())
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("ApplyUpgrade error = %v, want checksum mismatch", err)
	}
	if downloadCalls != 1 {
		t.Fatalf("downloadCalls = %d, want 1", downloadCalls)
	}
}

func requireDownloadDir(t *testing.T) {
	t.Helper()
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", downloadDir, err)
	}
}

