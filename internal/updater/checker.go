package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	osexec "os/exec"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/hoaxisr/awg-manager/internal/downloader"
	"github.com/hoaxisr/awg-manager/internal/sys/semver"
)

const (
	defaultEntwareRepoURL  = ""
	repoTimeout            = 30 * time.Second
	downloadTimeout        = 5 * time.Minute
	downloadDir            = "/opt/tmp"
	pkgName                = "awg-manager"
	changelogProbeMaxBytes = 64 << 10
)

const (
	channelStable  = "stable"
	channelDevelop = "develop"
)

const releaseChecksumWarning = "Пакет будет загружен из GitHub Release этого форка. SHA256 не опубликован, поэтому дополнительная проверка контрольной суммы пропущена."

var (
	releaseVersionPattern   = regexp.MustCompile(`^\d+\.\d+\.\d+(?:\.\d+)*(?:\+r\d+)?$`)
	releaseStableTagPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+(?:\.\d+)*$`)

	// entwareRepoURL is optional in this fork. We do not fall back to the
	// upstream author's Entware mirror for fork updates unless a mirror is
	// explicitly configured for the current build or test.
	entwareRepoURL = defaultEntwareRepoURL

	// releaseRepoURL defaults to this fork's GitHub releases and may still be
	// overridden at build time for custom release channels.
	releaseRepoURL = "https://github.com/iqubik/awg-manager-fork/releases"

	// releaseBaseURL is a legacy exact asset base override kept for backward
	// compatibility with localized/dev builds that explicitly pin iq-latest.
	releaseBaseURL = ""
)

// channelBaseURL возвращает базовый URL репозитория для канала. develop
// отдаётся из подкаталога /develop того же сервера.
func channelBaseURL(channel string) string {
	if channel == channelDevelop {
		return entwareRepoURL + "/develop"
	}
	return entwareRepoURL
}

func normalizedReleaseRepoURL() string {
	if trimmed := strings.TrimRight(strings.TrimSpace(releaseRepoURL), "/"); trimmed != "" {
		return trimmed
	}

	trimmedBase := strings.TrimRight(strings.TrimSpace(releaseBaseURL), "/")
	switch {
	case strings.HasSuffix(trimmedBase, "/download/iq-latest"):
		return strings.TrimSuffix(trimmedBase, "/download/iq-latest")
	case strings.HasSuffix(trimmedBase, "/latest/download"):
		return strings.TrimSuffix(trimmedBase, "/latest/download")
	default:
		return ""
	}
}

func releaseBaseURLForChannel(channel string) string {
	if channel == channelStable {
		if repoURL := strings.TrimRight(strings.TrimSpace(releaseRepoURL), "/"); repoURL != "" {
			return repoURL + "/latest/download"
		}
		return ""
	}

	if repoURL := normalizedReleaseRepoURL(); repoURL != "" {
		return repoURL + "/download/iq-latest"
	}

	return strings.TrimRight(strings.TrimSpace(releaseBaseURL), "/")
}

// versionComparator выбирает сравнялку версий по каналу: develop учитывает
// build-revision (+rN), stable — нет (как было).
func versionComparator(channel string) func(a, b string) int {
	if channel == channelDevelop {
		return semver.CompareWithRevision
	}
	return semver.Compare
}

// Check queries the entware repo's Packages.gz for the latest awg-manager
// version and returns update info including the .ipk download URL if a newer
// version is available. Uses the stable channel.
func Check(ctx context.Context, currentVersion string) *UpdateInfo {
	return checkWithDownloader(ctx, currentVersion, channelStable, newDefaultDownloader())
}

func checkWithDownloader(ctx context.Context, currentVersion, channel string, dl Downloader) *UpdateInfo {
	info := &UpdateInfo{
		CurrentVersion: currentVersion,
		CheckedAt:      time.Now(),
	}
	info.Channel = channel

	cmp := versionComparator(channel)

	if baseURL := releaseBaseURLForChannel(channel); baseURL != "" {
		info.Source = "release"
		info.SourceURL = releaseAssetURL(baseURL, "VERSION")
		if channel == channelStable {
			return checkStableLatestReleaseWithDownloader(ctx, info, currentVersion, dl, baseURL)
		}
		return checkReleaseWithDownloader(ctx, info, currentVersion, dl, cmp, baseURL)
	}

	base := channelBaseURL(channel)
	if strings.TrimSpace(base) == "" {
		info.Error = "update source is not configured for this build"
		return info
	}
	archDir := archSuffixToRepoDir(archSuffix())
	pkgsURL := fmt.Sprintf("%s/%s/Packages.gz", base, archDir)
	info.Source = "entware"
	info.SourceURL = pkgsURL

	pkg, err := fetchLatestPackageWithDownloader(ctx, dl, pkgsURL, pkgName, cmp)
	if err != nil {
		info.Error = fmt.Sprintf("entware repo: %s", err)
		return info
	}

	if cmp(currentVersion, pkg.Version) >= 0 {
		return info
	}

	info.Available = true
	info.LatestVersion = pkg.Version
	info.DownloadURL = fmt.Sprintf("%s/%s/%s", base, archDir, pkg.Filename)
	info.SHA256 = pkg.SHA256
	return info
}

// upgradeLogPath captures the detached opkg output. The old daemon is
// stopped by the package prerm mid-install, so this file is the ONLY
// diagnostic left if opkg fails and the router ends up without awg-manager.
const upgradeLogPath = "/opt/tmp/awg-manager-upgrade.log"

func checkStableLatestReleaseWithDownloader(
	ctx context.Context,
	info *UpdateInfo,
	currentVersion string,
	dl Downloader,
	baseURL string,
) *UpdateInfo {
	if dl == nil {
		dl = newDefaultDownloader()
	}

	latest, err := fetchReleaseVersionAssetWithDownloader(ctx, dl, releaseAssetURL(baseURL, "VERSION"), releaseAssetRef{
		channel: channelStable,
		tag:     "latest",
		name:    "VERSION",
	})
	if err != nil {
		if isMissingReleaseAssetError(err, "VERSION") {
			return checkStableManualLatestReleaseWithDownloader(ctx, info, currentVersion, dl)
		}
		info.Error = fmt.Sprintf("release channel: %s", err)
		return info
	}

	if semver.Compare(currentVersion, latest) >= 0 {
		return info
	}

	ipkURL := releaseAssetURL(baseURL, fmt.Sprintf("%s_%s_%s-kn.ipk", pkgName, latest, archSuffix()))
	_, meta, err := dl.ReadAll(ctx, downloader.Request{
		Purpose:       "awgm-update-check",
		URL:           ipkURL,
		Method:        http.MethodHead,
		Timeout:       repoTimeout,
		MaxBodyBytes:  1,
		AllowedStatus: []int{http.StatusOK, http.StatusNotFound, http.StatusMethodNotAllowed},
	})
	if err != nil {
		info.Error = fmt.Sprintf("release channel: %s", err)
		return info
	}
	if meta.StatusCode == http.StatusNotFound {
		info.Error = fmt.Sprintf("release channel: release asset %s not found in latest", path.Base(ipkURL))
		return info
	}
	if meta.StatusCode != http.StatusOK && meta.StatusCode != http.StatusMethodNotAllowed {
		info.Error = fmt.Sprintf("release channel: fetch %s: status %d", path.Base(ipkURL), meta.StatusCode)
		return info
	}

	info.Available = true
	info.LatestVersion = latest
	info.DownloadURL = ipkURL
	info.Warning = releaseChecksumWarning
	return info
}

func checkStableManualLatestReleaseWithDownloader(
	ctx context.Context,
	info *UpdateInfo,
	currentVersion string,
	dl Downloader,
) *UpdateInfo {
	repoURL := normalizedReleaseRepoURL()
	releaseInfo, err := stableReleaseResolver.ResolveStable(ctx, dl, repoURL)
	if err != nil {
		info.Error = fmt.Sprintf("release channel: %s", err)
		return info
	}

	if releaseInfo.HTMLURL != "" {
		info.SourceURL = releaseInfo.HTMLURL
	}

	if semver.Compare(currentVersion, releaseInfo.Version) >= 0 {
		return info
	}

	assetName := fmt.Sprintf("%s_%s_%s-kn.ipk", pkgName, releaseInfo.Version, archSuffix())
	downloadURL := strings.TrimSpace(releaseInfo.Assets[assetName])
	if downloadURL == "" {
		info.Error = fmt.Sprintf("release channel: stable release %s is missing asset %s", releaseInfo.TagName, assetName)
		return info
	}

	info.Available = true
	info.LatestVersion = releaseInfo.Version
	info.DownloadURL = downloadURL
	info.Warning = releaseChecksumWarning
	return info
}

func ensureStableLatestAssetExists(ctx context.Context, dl Downloader, assetURL, assetName string) error {
	if dl == nil {
		dl = newDefaultDownloader()
	}
	_, meta, err := dl.ReadAll(ctx, downloader.Request{
		Purpose:       "awgm-update-check",
		URL:           assetURL,
		Method:        http.MethodHead,
		Timeout:       repoTimeout,
		MaxBodyBytes:  1,
		AllowedStatus: []int{http.StatusOK, http.StatusNotFound, http.StatusMethodNotAllowed},
	})
	if err != nil {
		return err
	}
	switch meta.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusNotFound:
		return fmt.Errorf("release asset %s not found in latest", assetName)
	case http.StatusMethodNotAllowed:
		_, meta, err = dl.ReadAll(ctx, downloader.Request{
			Purpose:       "awgm-update-check",
			URL:           assetURL,
			Method:        http.MethodGet,
			Timeout:       repoTimeout,
			MaxBodyBytes:  changelogProbeMaxBytes,
			AllowedStatus: []int{http.StatusOK, http.StatusNotFound},
		})
		if err != nil {
			return sanitizeReleaseAssetError(releaseAssetRef{
				channel: channelStable,
				tag:     "latest",
				name:    assetName,
			}, err)
		}
		if meta.StatusCode == http.StatusNotFound {
			return fmt.Errorf("release asset %s not found in latest", assetName)
		}
		if meta.StatusCode != http.StatusOK {
			return fmt.Errorf("fetch %s: status %d", assetName, meta.StatusCode)
		}
		return nil
	default:
		return fmt.Errorf("fetch %s: status %d", assetName, meta.StatusCode)
	}
}

func checkReleaseWithDownloader(
	ctx context.Context,
	info *UpdateInfo,
	currentVersion string,
	dl Downloader,
	cmp func(a, b string) int,
	baseURL string,
) *UpdateInfo {
	if dl == nil {
		dl = newDefaultDownloader()
	}

	latest, err := fetchLatestReleaseVersionWithDownloader(ctx, dl, baseURL)
	if err != nil {
		info.Error = fmt.Sprintf("release channel: %s", err)
		return info
	}

	if cmp(currentVersion, latest) >= 0 {
		return info
	}

	info.Available = true
	info.LatestVersion = latest
	info.DownloadURL = releaseAssetURL(baseURL, fmt.Sprintf(
		"%s_%s_%s-kn.ipk",
		pkgName,
		latest,
		archSuffix(),
	))
	info.Warning = releaseChecksumWarning
	return info
}

func fetchLatestReleaseVersionWithDownloader(ctx context.Context, dl Downloader, baseURL string) (string, error) {
	return fetchReleaseVersionAssetWithDownloader(ctx, dl, releaseAssetURL(baseURL, "VERSION"), releaseAssetRef{
		channel: channelDevelop,
		tag:     "iq-latest",
		name:    "VERSION",
	})
}

type releaseAssetRef struct {
	channel string
	tag     string
	name    string
}

func fetchReleaseVersionAssetWithDownloader(ctx context.Context, dl Downloader, assetURL string, ref releaseAssetRef) (string, error) {
	body, _, err := dl.ReadAll(ctx, downloader.Request{
		Purpose:      "awgm-update-check",
		URL:          assetURL,
		Method:       http.MethodGet,
		Timeout:      repoTimeout,
		MaxBodyBytes: releaseVersionMaxBytes,
	})
	if err != nil {
		return "", sanitizeReleaseAssetError(ref, err)
	}

	version := strings.TrimSpace(string(body))
	if version == "" {
		return "", fmt.Errorf("release asset %s is empty in %s", ref.name, releaseAssetDisplayTarget(ref))
	}
	if strings.ContainsAny(version, "\r\n\t /\\") {
		return "", fmt.Errorf("release asset %s is invalid in %s: %q", ref.name, releaseAssetDisplayTarget(ref), version)
	}
	if !releaseVersionPattern.MatchString(version) {
		return "", fmt.Errorf("release asset %s is invalid in %s: %q", ref.name, releaseAssetDisplayTarget(ref), version)
	}

	return version, nil
}

func sanitizeReleaseAssetError(ref releaseAssetRef, err error) error {
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return fmt.Errorf("release asset %s not found in %s", ref.name, releaseAssetDisplayTarget(ref))
	}

	lower := strings.ToLower(msg)
	if strings.Contains(lower, "<!doctype html") || strings.Contains(lower, "<html") ||
		strings.Contains(lower, "status 404") || strings.Contains(lower, "not found") {
		return fmt.Errorf("release asset %s not found in %s", ref.name, releaseAssetDisplayTarget(ref))
	}

	return fmt.Errorf("fetch %s: %s", ref.name, msg)
}

func isMissingReleaseAssetError(err error, assetName string) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "release asset "+assetName+" not found in ")
}

func releaseAssetDisplayTarget(ref releaseAssetRef) string {
	if ref.channel == channelDevelop || strings.EqualFold(ref.tag, "iq-latest") {
		return "iq-latest"
	}
	if strings.TrimSpace(ref.tag) != "" {
		return ref.tag
	}
	return "release"
}

func releaseAssetURL(baseURL, filename string) string {
	return strings.TrimRight(baseURL, "/") + "/" + filename
}

// Upgrade downloads the IPK from downloadURL and launches opkg install in a
// detached process.
func Upgrade(ctx context.Context, downloadURL string) error {
	return upgradeWithDownloader(ctx, downloadURL, "", newDefaultDownloader())
}

var startDetachedUpgrade = func(ipkPath string) error {
	script := fmt.Sprintf("sleep 2 && opkg install %s && rm -f %s", ipkPath, ipkPath)
	cmd := osexec.Command("sh", "-c", script)
	if logf, err := os.OpenFile(upgradeLogPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644); err == nil {
		cmd.Stdout = logf
		cmd.Stderr = logf
		defer logf.Close() // child keeps its own fd after Start
	}
	setUpgradeDetachedProcess(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

func upgradeWithDownloader(ctx context.Context, downloadURL, wantSHA256 string, dl Downloader) error {
	if dl == nil {
		dl = newDefaultDownloader()
	}
	filename, err := ipkFilenameFromURL(downloadURL)
	if err != nil {
		return err
	}
	ipkPath := downloadDir + "/" + filename

	_, err = dl.DownloadFile(ctx, downloader.FileRequest{
		Request: downloader.Request{
			Purpose:      "awgm-update-ipk",
			URL:          downloadURL,
			Method:       http.MethodGet,
			Timeout:      downloadTimeout,
			MaxBodyBytes: ipkMaxBytes,
		},
		DestPath:     ipkPath,
		MaxFileBytes: ipkMaxBytes,
		Mode:         0o644,
		Atomic:       true,
	})
	if err != nil {
		return fmt.Errorf("download IPK: %w", err)
	}
	if err := verifyFileSHA256(ipkPath, wantSHA256); err != nil {
		os.Remove(ipkPath)
		return err
	}
	if err := startDetachedUpgrade(ipkPath); err != nil {
		os.Remove(ipkPath)
		return err
	}
	return nil
}

// verifyFileSHA256 checks path against the hex digest want. Empty want is
// accepted (older repo indexes without SHA256sum) — but when the index does
// provide a digest, a mismatch aborts the root opkg install of the file.
func verifyFileSHA256(path, want string) error {
	if want == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("verify IPK: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("verify IPK: %w", err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("IPK checksum mismatch: got %s, want %s", got, want)
	}
	return nil
}

func ipkFilenameFromURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid download URL: %w", err)
	}
	if strings.Contains(u.Path, "..") || strings.Contains(u.EscapedPath(), "..") || strings.Contains(strings.ToLower(u.EscapedPath()), "%2e") {
		return "", fmt.Errorf("invalid download URL path: %q", raw)
	}
	name := path.Base(u.Path)
	if name == "" || name == "." || name == "/" {
		return "", fmt.Errorf("invalid download URL path: %q", raw)
	}
	if !isSafeIPKFilename(name) {
		return "", fmt.Errorf("invalid package filename %q", name)
	}
	return name, nil
}

var safeIPKFilenameRe = regexp.MustCompile(`^[A-Za-z0-9._+-]+$`)

func isSafeIPKFilename(name string) bool {
	if name == "" || name == "." || name == "/" {
		return false
	}
	if !strings.HasPrefix(name, pkgName+"_") {
		return false
	}
	if !strings.HasSuffix(strings.ToLower(name), ".ipk") {
		return false
	}
	return safeIPKFilenameRe.MatchString(name)
}
