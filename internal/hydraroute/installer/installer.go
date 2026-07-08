package installer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hoaxisr/awg-manager/internal/logging"
	"github.com/hoaxisr/awg-manager/internal/sys/routerinfo"
)

const (
	DefaultBinaryPath    = "/opt/bin/hrneo"
	DefaultControlPath   = "/opt/bin/neo"
	DefaultFeedConfPath  = "/opt/etc/opkg/customfeeds.conf"
	DefaultOpkgDir       = "/opt/etc/opkg"
	DefaultOpkgBinary    = "/opt/bin/opkg"
	DefaultPackageName   = "hrneo"
	DefaultHelperPackage = "wget-ssl"
	OldFeedBase          = "https://ground-zerro.github.io/release/keenetic/"
	NewFeedBase          = "https://git.zerrolabs.org/Ground-Zerro/release/pages/keenetic/"
)

var (
	versionRe        = regexp.MustCompile(`(?i)\bv?(\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?)\b`)
	archFeedSuffixes = map[string]string{
		"aarch64-3.10": "aarch64-k3.10",
		"mipsel-3.4":   "mipselsf-k3.4",
		"mips-3.4":     "mipssf-k3.4",
	}
)

type PackageState struct {
	Installed        bool
	Version          string
	UpdateAvailable  bool
	CandidateVersion string
	CheckedAt        time.Time
}

const packageStateTTL = 60 * time.Second

type Installer struct {
	binaryPath    string
	controlPath   string
	feedConfPath  string
	opkgDir       string
	opkgBinary    string
	packageName   string
	helperPackage string
	arch          string
	feedURL       string
	appLog        *logging.ScopedLogger
	freeDisk      func(path string) (int64, bool)
	runCmd        func(ctx context.Context, name string, args ...string) (string, error)
	stat          func(name string) (os.FileInfo, error)
	readFile      func(name string) ([]byte, error)
	writeFile     func(name string, data []byte, perm os.FileMode) error
	mkdirAll      func(path string, perm os.FileMode) error
	removeFile    func(name string) error

	shaMu   sync.Mutex
	shaVal  string
	shaSize int64
	shaMod  time.Time

	psMu         sync.Mutex
	packageState PackageState
}

func New(binaryPath, controlPath, arch string, appLogger logging.AppLogger) *Installer {
	if strings.TrimSpace(binaryPath) == "" {
		binaryPath = DefaultBinaryPath
	}
	if strings.TrimSpace(controlPath) == "" {
		controlPath = DefaultControlPath
	}

	return &Installer{
		binaryPath:    binaryPath,
		controlPath:   controlPath,
		feedConfPath:  DefaultFeedConfPath,
		opkgDir:       DefaultOpkgDir,
		opkgBinary:    DefaultOpkgBinary,
		packageName:   DefaultPackageName,
		helperPackage: DefaultHelperPackage,
		arch:          strings.TrimSpace(arch),
		feedURL:       feedURLForArch(arch),
		appLog:        logging.NewScopedLogger(appLogger, logging.GroupRouting, logging.SubHrNeo),
		freeDisk:      routerinfo.FreeBytes,
		runCmd:        runCommand,
		stat:          os.Stat,
		readFile:      os.ReadFile,
		writeFile:     os.WriteFile,
		mkdirAll:      os.MkdirAll,
		removeFile:    os.Remove,
	}
}

func (i *Installer) SetOpkgBinary(path string) {
	i.opkgBinary = path
}

func (i *Installer) SetRemoveFileFn(fn func(string) error) {
	if fn != nil {
		i.removeFile = fn
	}
}

func (i *Installer) BinaryPath() string { return i.binaryPath }

func (i *Installer) ControlPath() string { return i.controlPath }

func (i *Installer) RequiredVersion() string { return "" }

func (i *Installer) RequiredSHA256() string { return "" }

func (i *Installer) RequiredSize() int64 { return 0 }

func (i *Installer) Supported() bool {
	if strings.TrimSpace(i.feedURL) == "" {
		return false
	}
	if !dirExists(i.stat, i.opkgDir) {
		return false
	}
	return isExecutable(i.stat, i.opkgBinary)
}

func (i *Installer) PackageInstalled(ctx context.Context) bool {
	if i == nil {
		return false
	}
	ps := i.PackageState(ctx, false)
	return ps.Installed
}

func (i *Installer) CurrentSHA256() (string, error) {
	return i.binarySHA256()
}

func (i *Installer) FreeBytes() (int64, bool) {
	if i == nil || i.freeDisk == nil {
		return 0, false
	}
	return i.freeDisk(i.opkgDir)
}

func (i *Installer) SetFreeDiskFn(fn func(string) (int64, bool)) {
	i.freeDisk = fn
}

func (i *Installer) SetTestOpkgEnvironment(opkgDir, feedConfPath string) {
	if strings.TrimSpace(opkgDir) != "" {
		i.opkgDir = opkgDir
	}
	if strings.TrimSpace(feedConfPath) != "" {
		i.feedConfPath = feedConfPath
	}
}

func (i *Installer) SetCommandRunner(fn func(context.Context, string, ...string) (string, error)) {
	if fn != nil {
		i.runCmd = fn
	}
}

func (i *Installer) CurrentVersion(ctx context.Context) string {
	if !isExecutable(i.stat, i.binaryPath) {
		return ""
	}

	cctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	for _, args := range [][]string{{"--version"}, {"version"}, {"-v"}} {
		out, err := i.runCmd(cctx, i.binaryPath, args...)
		if err != nil {
			continue
		}
		if v := parseVersionOutput(out); v != "" {
			return v
		}
	}

	if pkgVersion, ok := i.InstalledPackageVersion(ctx); ok {
		return pkgVersion
	}
	return ""
}

func (i *Installer) DetectInstallationMode(ctx context.Context) (managed bool, legacy bool) {
	i.InvalidatePackageState()
	ps := i.PackageState(ctx, false)
	if ps.Installed {
		return true, false
	}
	if isExecutable(i.stat, i.binaryPath) {
		return false, true
	}
	return false, false
}

func (i *Installer) InstalledPackageVersion(ctx context.Context) (string, bool) {
	ps := i.PackageState(ctx, false)
	if !ps.Installed {
		return "", false
	}
	return ps.Version, true
}

func (i *Installer) AvailableUpdateVersion(ctx context.Context) (string, bool) {
	ps := i.PackageState(ctx, false)
	if !ps.Installed {
		return "", false
	}
	if !ps.UpdateAvailable {
		return "", false
	}
	return ps.CandidateVersion, true
}

func (i *Installer) EvaluateInstallState() InstallState {
	ctx := context.Background()
	ps := i.PackageState(ctx, false)
	if !ps.Installed && !isExecutable(i.stat, i.binaryPath) {
		return InstallStateMissing
	}
	if !i.Supported() {
		return InstallStateMissing
	}
	return InstallStateInstalled
}

func (i *Installer) InvalidatePackageState() {
	i.psMu.Lock()
	i.packageState = PackageState{}
	i.psMu.Unlock()
}

func (i *Installer) PackageState(ctx context.Context, force bool) PackageState {
	i.psMu.Lock()
	ps := i.packageState
	i.psMu.Unlock()

	if !force && !ps.CheckedAt.IsZero() && time.Since(ps.CheckedAt) < packageStateTTL {
		return ps
	}

	checked := time.Now()
	next := PackageState{CheckedAt: checked}
	statusOut, err := i.packageStatus(ctx)
	if err == nil {
		next.Installed = true
		for _, line := range strings.Split(statusOut, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(strings.ToLower(line), "version:") {
				next.Version = strings.TrimSpace(strings.TrimPrefix(line, "Version:"))
				break
			}
		}
	}

	if next.Installed {
		upgradeOut, err := i.runOpkg(ctx, "list-upgradable")
		if err == nil {
			prefix := i.packageName + " - "
			for _, line := range strings.Split(upgradeOut, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || !strings.HasPrefix(line, prefix) {
					continue
				}
				parts := strings.Split(line, " - ")
				if len(parts) >= 3 {
					next.UpdateAvailable = true
					next.CandidateVersion = strings.TrimSpace(parts[len(parts)-1])
					break
				}
			}
		}
	}

	i.psMu.Lock()
	i.packageState = next
	i.psMu.Unlock()
	return next
}

func (i *Installer) packageStatus(ctx context.Context) (string, error) {
	if i.runCmd == nil {
		return "", fmt.Errorf("no command runner")
	}
	if !i.Supported() {
		return "", fmt.Errorf("unsupported arch %s", i.arch)
	}
	out, err := i.runCmd(ctx, i.opkgBinary, "status", i.packageName)
	if err != nil {
		return "", err
	}
	if !strings.Contains(strings.ToLower(out), "package:") {
		return "", fmt.Errorf("unexpected opkg status output")
	}
	return out, nil
}

func (i *Installer) replaceOldBaseIfNeeded() error {
	data, err := i.readFile(i.feedConfPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", i.feedConfPath, err)
	}
	if !strings.Contains(string(data), OldFeedBase) {
		return nil
	}
	updated := strings.ReplaceAll(string(data), OldFeedBase, NewFeedBase)
	if err := i.writeFile(i.feedConfPath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", i.feedConfPath, err)
	}
	return nil
}

func (i *Installer) ensureGroundZerroFeed() error {
	const prefix = "src/gz ground-zerro "

	lines := []string{}
	data, err := i.readFile(i.feedConfPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", i.feedConfPath, err)
	}
	if err == nil {
		for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, prefix) {
				continue
			}
			lines = append(lines, trimmed)
		}
	}

	lines = append(lines, prefix+i.feedURL)
	content := strings.Join(lines, "\n") + "\n"
	if err := i.writeFile(i.feedConfPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", i.feedConfPath, err)
	}
	return nil
}

func (i *Installer) runOpkg(ctx context.Context, args ...string) (string, error) {
	out, err := i.runCmd(ctx, i.opkgBinary, args...)
	if err != nil {
		trimmed := strings.TrimSpace(out)
		if trimmed == "" {
			return out, fmt.Errorf("opkg %s: %w", strings.Join(args, " "), err)
		}
		return out, fmt.Errorf("opkg %s: %s: %w", strings.Join(args, " "), trimmed, err)
	}
	return out, nil
}

func (i *Installer) runOpkgErr(ctx context.Context, args ...string) error {
	_, err := i.runOpkg(ctx, args...)
	return err
}

func (i *Installer) binarySHA256() (string, error) {
	i.shaMu.Lock()
	defer i.shaMu.Unlock()

	fi, err := i.stat(i.binaryPath)
	if err != nil {
		return "", err
	}
	if i.shaVal != "" && fi.Size() == i.shaSize && fi.ModTime().Equal(i.shaMod) {
		return i.shaVal, nil
	}
	sha, err := sha256File(i.binaryPath)
	if err != nil {
		return "", err
	}
	i.shaVal = sha
	i.shaSize = fi.Size()
	i.shaMod = fi.ModTime()
	return sha, nil
}

func (i *Installer) EnsureFeed(ctx context.Context) error {
	if !i.Supported() {
		return fmt.Errorf("HydraRoute install is not supported for arch %q", i.arch)
	}
	if err := i.mkdirAll(i.opkgDir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", i.opkgDir, err)
	}

	if err := i.replaceOldBaseIfNeeded(); err != nil {
		return err
	}
	if err := i.runOpkgErr(ctx, "update"); err != nil {
		return err
	}
	if err := i.runOpkgErr(ctx, "install", i.helperPackage); err != nil {
		return err
	}
	if err := i.ensureGroundZerroFeed(); err != nil {
		return err
	}
	if err := i.runOpkgErr(ctx, "update"); err != nil {
		return err
	}
	i.InvalidatePackageState()
	return nil
}

func (i *Installer) InstallPackage(ctx context.Context) error {
	if !i.Supported() {
		return fmt.Errorf("HydraRoute install is not supported for arch %q", i.arch)
	}
	err := i.installWithLegacyBackup(ctx, i.packageName)
	i.InvalidatePackageState()
	return err
}

func (i *Installer) UpgradePackage(ctx context.Context) error {
	if !i.Supported() {
		return fmt.Errorf("HydraRoute install is not supported for arch %q", i.arch)
	}
	_, err := i.runOpkg(ctx, "upgrade", i.packageName)
	i.InvalidatePackageState()
	return err
}

func (i *Installer) installWithLegacyBackup(ctx context.Context, pkg string) error {
	hasLegacy := isExecutable(i.stat, i.binaryPath) || isExecutable(i.stat, i.controlPath)
	if !hasLegacy {
		return i.runOpkgErr(ctx, "install", pkg)
	}

	backupDir, err := i.createLegacyBackup(ctx)
	if err != nil {
		return err
	}

	if installErr := i.runOpkgErr(ctx, "install", pkg); installErr != nil {
		restoreErr := i.restoreLegacyBackup(backupDir)
		if restoreErr != nil {
			return fmt.Errorf("restore legacy backup: %w (install error: %w)", restoreErr, installErr)
		}
		return installErr
	}
	return nil
}

func (i *Installer) createLegacyBackup(ctx context.Context) (string, error) {
	ts := time.Now().Format("20060102-150405")
	backupDir := filepath.Join("/opt/etc/awg-manager/backups/hydraroute", ts)
	if err := i.mkdirAll(backupDir, 0o755); err != nil {
		return "", fmt.Errorf("create legacy backup dir %s: %w", backupDir, err)
	}
	var lastErr error
	for _, target := range []string{i.binaryPath, i.controlPath} {
		if !isExecutable(i.stat, target) {
			continue
		}
		data, err := i.readFile(target)
		if err != nil {
			lastErr = errorsCombine(lastErr, fmt.Errorf("read legacy %s: %w", target, err))
			continue
		}
		base := filepath.Base(target)
		if err := i.writeFile(filepath.Join(backupDir, base), data, 0o755); err != nil {
			lastErr = errorsCombine(lastErr, fmt.Errorf("write legacy backup %s: %w", base, err))
			continue
		}
		if err := i.removeFile(target); err != nil {
			lastErr = errorsCombine(lastErr, fmt.Errorf("remove legacy %s for migration: %w", target, err))
		}
	}
	if lastErr != nil {
		_ = i.removeAllLegacyBackup(backupDir)
		return "", lastErr
	}
	return backupDir, nil
}

func (i *Installer) restoreLegacyBackup(backupDir string) error {
	if backupDir == "" {
		return nil
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return err
	}
	var lastErr error
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		src := filepath.Join(backupDir, entry.Name())
		target, perm := legacyTargetForBackupEntry(i.binaryPath, i.controlPath, entry.Name())
		if target == "" {
			continue
		}
		if err := copyFile(src, target, perm, i.mkdirAll, i.readFile, i.writeFile); err != nil {
			lastErr = errorsCombine(lastErr, err)
		}
	}
	return lastErr
}

func legacyTargetForBackupEntry(binaryPath, controlPath, name string) (string, os.FileMode) {
	switch name {
	case filepath.Base(binaryPath):
		return binaryPath, 0o755
	case filepath.Base(controlPath):
		return controlPath, 0o755
	default:
		return "", 0
	}
}

func (i *Installer) removeAllLegacyBackup(backupDir string) error {
	_ = i.restoreLegacyBackup(backupDir)
	return os.RemoveAll(backupDir)
}

func copyFile(src, dst string, perm os.FileMode, mkdirAll func(string, os.FileMode) error, readFile func(string) ([]byte, error), writeFile func(string, []byte, os.FileMode) error) error {
	if err := mkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	data, err := readFile(src)
	if err != nil {
		return err
	}
	if err := writeFile(dst, data, perm); err != nil {
		_ = os.Remove(dst)
		return err
	}
	return nil
}

func errorsCombine(errs ...error) error {
	var out error
	for _, err := range errs {
		if err == nil {
			continue
		}
		if out == nil {
			out = err
			continue
		}
		out = fmt.Errorf("%v; %w", out, err)
	}
	return out
}

func parseVersionOutput(out string) string {
	m := versionRe.FindStringSubmatch(strings.TrimSpace(out))
	if len(m) < 2 {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(m[1]), "v")
}

func feedURLForArch(arch string) string {
	if suffix, ok := archFeedSuffixes[strings.TrimSpace(arch)]; ok {
		return NewFeedBase + suffix
	}
	return ""
}

func dirExists(stat func(string) (os.FileInfo, error), path string) bool {
	info, err := stat(path)
	return err == nil && info.IsDir()
}

func isExecutable(stat func(string) (os.FileInfo, error), path string) bool {
	info, err := stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func runCommand(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return string(out), err
}
