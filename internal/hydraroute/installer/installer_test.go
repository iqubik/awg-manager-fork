package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFeedURLForArch(t *testing.T) {
	cases := map[string]string{
		"aarch64-3.10": NewFeedBase + "aarch64-k3.10",
		"mipsel-3.4":   NewFeedBase + "mipselsf-k3.4",
		"mips-3.4":     NewFeedBase + "mipssf-k3.4",
		"unknown":      "",
	}
	for arch, want := range cases {
		if got := feedURLForArch(arch); got != want {
			t.Fatalf("feedURLForArch(%q)=%q want %q", arch, got, want)
		}
	}
}

func TestInstaller_SupportedRequiresArchAndOpkgDir(t *testing.T) {
	tmp := t.TempDir()
	inst := newTestInstaller(t, tmp, "aarch64-3.10", nil)
	if !inst.Supported() {
		t.Fatal("Supported=false want true")
	}

	inst = newTestInstaller(t, tmp, "unknown", nil)
	if inst.Supported() {
		t.Fatal("Supported=true want false for unknown arch")
	}
}

func TestInstaller_SupportedRequiresOpkgBinary(t *testing.T) {
	tmp := t.TempDir()
	inst := newTestInstaller(t, tmp, "aarch64-3.10", nil)
	if !inst.Supported() {
		t.Fatal("Supported=false want true")
	}

	os.Remove(filepath.Join(inst.opkgDir, "opkg"))
	if inst.Supported() {
		t.Fatal("Supported=true want false when opkg binary is missing")
	}
}

func TestInstaller_EnsureFeed_RewritesFeedAndRunsExpectedOpkgFlow(t *testing.T) {
	tmp := t.TempDir()
	var calls []string
	inst := newTestInstaller(t, tmp, "mipsel-3.4", func(_ context.Context, name string, args ...string) (string, error) {
		calls = append(calls, filepath.Base(name)+" "+strings.Join(args, " "))
		return "", nil
	})

	feedConf := filepath.Join(tmp, "customfeeds.conf")
	inst.feedConfPath = feedConf
	if err := os.WriteFile(feedConf, []byte("src/gz ground-zerro "+OldFeedBase+"mipsel-legacy\nsrc/gz other https://example.invalid/feed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := inst.EnsureFeed(context.Background()); err != nil {
		t.Fatalf("EnsureFeed() err: %v", err)
	}

	data, err := os.ReadFile(feedConf)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Contains(text, OldFeedBase) {
		t.Fatalf("old base still present:\n%s", text)
	}
	wantLine := "src/gz ground-zerro " + NewFeedBase + "mipselsf-k3.4"
	if !strings.Contains(text, wantLine) {
		t.Fatalf("feed line missing:\n%s", text)
	}
	if strings.Count(text, "src/gz ground-zerro ") != 1 {
		t.Fatalf("unexpected ground-zerro entries:\n%s", text)
	}

	wantCalls := []string{
		"opkg update",
		"opkg install wget-ssl",
		"opkg update",
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("opkg calls=%v want %v", calls, wantCalls)
	}
}

func TestInstaller_DetectInstallationMode_ManagedAndLegacy(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()
	inst := newTestInstaller(t, tmp, "aarch64-3.10", nil)

	if managed, legacy := inst.DetectInstallationMode(ctx); managed || legacy {
		t.Fatalf("DetectInstallationMode()=%v/%v want false/false", managed, legacy)
	}

	mustWriteExecutable(t, inst.binaryPath, "#!/bin/sh\necho 2.4.1\n")
	inst.runCmd = func(_ context.Context, name string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "status" && args[1] == inst.packageName {
			return "", fmt.Errorf("not installed")
		}
		return "", nil
	}
	if managed, legacy := inst.DetectInstallationMode(ctx); managed || !legacy {
		t.Fatalf("DetectInstallationMode()=%v/%v want false/true", managed, legacy)
	}

	inst.runCmd = func(_ context.Context, name string, args ...string) (string, error) {
		if len(args) >= 2 && args[0] == "status" && args[1] == inst.packageName {
			return "Package: hrneo\nVersion: 2.4.1\n", nil
		}
		return "", nil
	}
	if managed, legacy := inst.DetectInstallationMode(ctx); !managed || legacy {
		t.Fatalf("DetectInstallationMode()=%v/%v want true/false", managed, legacy)
	}
}

func TestInstaller_EvaluateInstallState_UsesPackageStatus(t *testing.T) {
	tmp := t.TempDir()
	inst := newTestInstaller(t, tmp, "aarch64-3.10", func(_ context.Context, name string, args ...string) (string, error) {
		if filepath.Base(name) == "opkg" && len(args) > 0 && args[0] == "status" {
			return "Package: hrneo\nVersion: 2.4.1\n", nil
		}
		if filepath.Base(name) == DefaultPackageName && len(args) > 0 && args[0] == "status" {
			return "Package: hrneo\nVersion: 2.4.1\n", nil
		}
		return "", nil
	})

	if got := inst.EvaluateInstallState(); got != InstallStateInstalled {
		t.Fatalf("EvaluateInstallState()=%q want %q", got, InstallStateInstalled)
	}
}

func TestInstaller_AvailableUpdateVersion_ParsesListUpgradable(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()
	inst := newTestInstaller(t, tmp, "aarch64-3.10", func(_ context.Context, name string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "status hrneo":
			return "Package: hrneo\nVersion: 2.4.1\n", nil
		case "list-upgradable":
			return "hrneo - 2.4.1 - 2.4.2\nother - 1 - 2\n", nil
		default:
			return "", nil
		}
	})

	got, ok := inst.AvailableUpdateVersion(ctx)
	if !ok || got != "2.4.2" {
		t.Fatalf("AvailableUpdateVersion()=%q/%v want 2.4.2/true", got, ok)
	}
}

func TestInstaller_CurrentVersion_FallsBackToPackageStatus(t *testing.T) {
	tmp := t.TempDir()
	inst := newTestInstaller(t, tmp, "aarch64-3.10", func(_ context.Context, name string, args ...string) (string, error) {
		if filepath.Base(name) != "opkg" {
			return "", fmt.Errorf("bad flag")
		}
		if strings.Join(args, " ") == "status hrneo" {
			return "Package: hrneo\nVersion: 2.4.9\n", nil
		}
		return "", nil
	})
	mustWriteExecutable(t, inst.binaryPath, "#!/bin/sh\nexit 1\n")

	if got := inst.CurrentVersion(context.Background()); got != "2.4.9" {
		t.Fatalf("CurrentVersion()=%q want 2.4.9", got)
	}
}

func TestInstaller_InstallAndUpgradePackage_RunExpectedCommands(t *testing.T) {
	tmp := t.TempDir()
	var calls []string
	inst := newTestInstaller(t, tmp, "mipsel-3.4", func(_ context.Context, name string, args ...string) (string, error) {
		calls = append(calls, filepath.Base(name)+" "+strings.Join(args, " "))
		return "", nil
	})

	if err := inst.InstallPackage(context.Background()); err != nil {
		t.Fatalf("InstallPackage() err: %v", err)
	}
	if err := inst.UpgradePackage(context.Background()); err != nil {
		t.Fatalf("UpgradePackage() err: %v", err)
	}

	want := []string{
		"opkg install hrneo",
		"opkg upgrade hrneo",
	}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%v want %v", calls, want)
	}
}

func TestInstaller_PackageState_ReusesCacheWithinTTL(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()
	var calls []string
	inst := newTestInstaller(t, tmp, "aarch64-3.10", func(_ context.Context, name string, args ...string) (string, error) {
		calls = append(calls, filepath.Base(name)+" "+strings.Join(args, " "))
		switch strings.Join(args, " ") {
		case "status hrneo":
			return "Package: hrneo\nVersion: 2.4.1\n", nil
		case "list-upgradable":
			return "", nil
		default:
			return "", nil
		}
	})

	first := inst.PackageState(ctx, false)
	if !first.Installed || first.Version != "2.4.1" {
		t.Fatalf("first=%+v want installed 2.4.1", first)
	}
	second := inst.PackageState(ctx, false)
	if !second.Installed || second.Version != "2.4.1" {
		t.Fatalf("second=%+v want installed 2.4.1", second)
	}
	if len(calls) != 2 {
		t.Fatalf("calls=%v want 2 opkg invocations: status, list-upgradable", calls)
	}
}

func TestInstaller_PackageState_CacheForceRefresh(t *testing.T) {
	tmp := t.TempDir()
	ctx := context.Background()
	var calls []string
	inst := newTestInstaller(t, tmp, "aarch64-3.10", func(_ context.Context, name string, args ...string) (string, error) {
		calls = append(calls, filepath.Base(name)+" "+strings.Join(args, " "))
		switch strings.Join(args, " ") {
		case "status hrneo":
			return "Package: hrneo\nVersion: 2.4.1\n", nil
		case "list-upgradable":
			return "", nil
		default:
			return "", nil
		}
	})

	_ = inst.PackageState(ctx, false)
	inst.InvalidatePackageState()
	_ = inst.PackageState(ctx, false)
	if len(calls) != 4 {
		t.Fatalf("calls=%v want 4 opkg invocations after invalidation", calls)
	}
}

func newTestInstaller(
	t *testing.T,
	tmp string,
	arch string,
	run func(context.Context, string, ...string) (string, error),
) *Installer {
	t.Helper()

	opkgDir := filepath.Join(tmp, "opkg")
	opkgPath := filepath.Join(opkgDir, "opkg")
	if err := os.MkdirAll(opkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(opkgPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	inst := New(filepath.Join(tmp, "hrneo"), filepath.Join(tmp, "neo"), arch, nil)
	inst.opkgDir = opkgDir
	inst.opkgBinary = opkgPath
	inst.feedConfPath = filepath.Join(tmp, "customfeeds.conf")
	if run != nil {
		inst.runCmd = run
	}
	return inst
}

func mustWriteExecutable(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}
