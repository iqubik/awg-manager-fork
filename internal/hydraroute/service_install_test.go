package hydraroute

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	hydrainstaller "github.com/hoaxisr/awg-manager/internal/hydraroute/installer"
)

func TestService_Install_OfficialPackageUpdatesStatus(t *testing.T) {
	tmp := t.TempDir()
	restoreHydraPaths(t, tmp)

	var calls []string
	inst := newHydraTestInstaller(t, tmp, func(_ context.Context, name string, args ...string) (string, error) {
		call := filepath.Base(name) + " " + strings.Join(args, " ")
		calls = append(calls, call)
		switch call {
		case "opkg status hrneo":
			if fileExists(legacyHrneoBinary) {
				return "Package: hrneo\nVersion: 2.4.1\n", nil
			}
			return "", fmt.Errorf("not installed")
		case "opkg list-upgradable":
			return "", nil
		case "opkg install hrneo":
			mustWriteExecutableFile(t, legacyHrneoBinary, "#!/bin/sh\necho 'HydraRoute Neo 2.4.1'\n")
			mustWriteExecutableFile(t, legacyNeoCommand, "#!/bin/sh\nexit 0\n")
			return "", nil
		default:
			return "", nil
		}
	})

	svc := NewService(nil, nil)
	svc.SetInstaller(inst)

	if err := svc.Install(context.Background()); err != nil {
		t.Fatalf("Install() err: %v", err)
	}

	st := svc.RefreshStatus()
	if !st.Installed || !st.Managed || st.Legacy {
		t.Fatalf("status installed/managed/legacy=%v/%v/%v want true/true/false", st.Installed, st.Managed, st.Legacy)
	}
	if st.CurrentVersion != "2.4.1" {
		t.Fatalf("CurrentVersion=%q want 2.4.1", st.CurrentVersion)
	}
	if !st.InstallSupported {
		t.Fatal("InstallSupported=false want true")
	}

	wantOps := []string{
		"opkg status hrneo",
		"opkg update",
		"opkg install wget-ssl",
		"opkg update",
		"opkg install hrneo",
	}
	assertContainsSequence(t, calls, wantOps)
}

func TestService_Update_LegacyRefusesOfficialUpdate(t *testing.T) {
	tmp := t.TempDir()
	restoreHydraPaths(t, tmp)
	mustWriteExecutableFile(t, legacyHrneoBinary, "#!/bin/sh\necho 'HydraRoute Neo 2.4.1'\n")
	mustWriteExecutableFile(t, legacyNeoCommand, "#!/bin/sh\nexit 0\n")

	inst := newHydraTestInstaller(t, tmp, func(_ context.Context, name string, args ...string) (string, error) {
		if strings.Join(args, " ") == "status hrneo" {
			return "", fmt.Errorf("not installed")
		}
		return "", nil
	})

	svc := NewService(nil, nil)
	svc.SetInstaller(inst)

	err := svc.Update(context.Background())
	if err == nil || !strings.Contains(err.Error(), "внешняя или нестандартная установка") {
		t.Fatalf("Update() err=%v want legacy package warning", err)
	}
}

func TestService_Install_RunningLegacyStopsThenStarts(t *testing.T) {
	tmp := t.TempDir()
	restoreHydraPaths(t, tmp)

	logPath := filepath.Join(tmp, "neo.log")
	writeControlScript(t, legacyNeoCommand, pidFile, logPath, os.Getpid())
	mustWriteExecutableFile(t, legacyHrneoBinary, "#!/bin/sh\necho 'HydraRoute Neo 2.3.0'\n")
	mustWriteTextFile(t, pidFile, fmt.Sprintf("%d", os.Getpid()))

	inst := newHydraTestInstaller(t, tmp, func(_ context.Context, name string, args ...string) (string, error) {
		call := opkgTestCall(name, args...)
		switch call {
		case "opkg status hrneo":
			if fileExists(legacyHrneoBinary) {
				return "Package: hrneo\nVersion: 2.4.1\n", nil
			}
			return "", fmt.Errorf("not installed")
		case "opkg list-upgradable":
			return "", nil
		case "opkg install hrneo":
			mustWriteExecutableFile(t, legacyHrneoBinary, fmt.Sprintf(`#!/bin/sh
case "${1:-}" in
  --version|version|-v)
    echo 'HydraRoute Neo 2.4.1'
    ;;
  start|restart)
    mkdir -p "$(dirname %q)"
    printf "%%s" %d > %q
    ;;
  stop)
    rm -f %q
    ;;
esac
`, pidFile, os.Getpid(), pidFile, pidFile))
			writeControlScript(t, legacyNeoCommand, pidFile, logPath, os.Getpid())
			return "", nil
		default:
			return "", nil
		}
	})

	svc := NewService(nil, nil)
	svc.SetInstaller(inst)

	if err := svc.Install(context.Background()); err != nil {
		t.Fatalf("Install() err: %v", err)
	}

	lines := readCommandLog(t, logPath)
	if len(lines) != 2 || lines[0] != "stop" || lines[1] != "start" {
		t.Fatalf("legacy control log=%v want [stop start]", lines)
	}

	st := svc.RefreshStatus()
	if !st.Running {
		t.Fatal("Running=false want true after restart")
	}
	if !st.Managed || st.Legacy {
		t.Fatalf("managed/legacy=%v/%v want true/false", st.Managed, st.Legacy)
	}
}

func TestService_Status_ManagedPackageShowsAvailableUpdate(t *testing.T) {
	tmp := t.TempDir()
	restoreHydraPaths(t, tmp)
	mustWriteExecutableFile(t, legacyHrneoBinary, "#!/bin/sh\necho 'HydraRoute Neo 2.4.1'\n")

	inst := newHydraTestInstaller(t, tmp, func(_ context.Context, name string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "status hrneo":
			return "Package: hrneo\nVersion: 2.4.1\n", nil
		case "list-upgradable":
			return "hrneo - 2.4.1 - 2.4.2\n", nil
		default:
			return "", nil
		}
	})

	svc := NewService(nil, nil)
	svc.SetInstaller(inst)

	st := svc.RefreshStatus()
	if !st.Managed || st.Legacy {
		t.Fatalf("managed/legacy=%v/%v want true/false", st.Managed, st.Legacy)
	}
	if !st.UpdateAvailable {
		t.Fatal("UpdateAvailable=false want true")
	}
	if st.RequiredVersion != "2.4.2" {
		t.Fatalf("RequiredVersion=%q want 2.4.2", st.RequiredVersion)
	}
	if st.CustomBuild {
		t.Fatal("CustomBuild=true want false")
	}
}

func TestService_Status_UsesInstallerCurrentVersionFallback(t *testing.T) {
	tmp := t.TempDir()
	restoreHydraPaths(t, tmp)
	mustWriteExecutableFile(t, legacyHrneoBinary, "#!/bin/sh\nexit 1\n")

	inst := newHydraTestInstaller(t, tmp, func(_ context.Context, name string, args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "status hrneo":
			return "Package: hrneo\nVersion: 2.4.9\n", nil
		case "list-upgradable":
			return "", nil
		default:
			return "", fmt.Errorf("unsupported command: %s %s", name, strings.Join(args, " "))
		}
	})

	svc := NewService(nil, nil)
	svc.SetInstaller(inst)

	st := svc.RefreshStatus()
	if st.CurrentVersion != "2.4.9" {
		t.Fatalf("CurrentVersion=%q want 2.4.9", st.CurrentVersion)
	}
	if st.Version != "2.4.9" {
		t.Fatalf("Version=%q want 2.4.9", st.Version)
	}
	if !st.VersionMatchesRequired {
		t.Fatal("VersionMatchesRequired=false want true when no upgrade is available")
	}
	if !st.ChecksumMatchesRequired {
		t.Fatal("ChecksumMatchesRequired=false want true when no required checksum is defined")
	}
}

func TestService_Update_ManagedPackageRunsUpgrade(t *testing.T) {
	tmp := t.TempDir()
	restoreHydraPaths(t, tmp)
	mustWriteExecutableFile(t, legacyHrneoBinary, "#!/bin/sh\necho 'HydraRoute Neo 2.4.1'\n")

	var calls []string
	inst := newHydraTestInstaller(t, tmp, func(_ context.Context, name string, args ...string) (string, error) {
		call := opkgTestCall(name, args...)
		calls = append(calls, call)
		switch call {
		case "opkg status hrneo":
			return "Package: hrneo\nVersion: 2.4.1\n", nil
		case "opkg list-upgradable":
			return "hrneo - 2.4.1 - 2.4.2\n", nil
		default:
			return "", nil
		}
	})

	svc := NewService(nil, nil)
	svc.SetInstaller(inst)

	if err := svc.Update(context.Background()); err != nil {
		t.Fatalf("Update() err: %v", err)
	}

	assertContainsSequence(t, calls, []string{
		"opkg status hrneo",
		"opkg list-upgradable",
		"opkg update",
		"opkg install wget-ssl",
		"opkg update",
		"opkg upgrade hrneo",
	})
}

func newHydraTestInstaller(
	t *testing.T,
	tmp string,
	run func(context.Context, string, ...string) (string, error),
) *hydrainstaller.Installer {
	t.Helper()

	opkgDir := filepath.Join(tmp, "opkg")
	if err := os.MkdirAll(opkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	opkgPath := filepath.Join(opkgDir, "opkg")
	if err := os.WriteFile(opkgPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	inst := hydrainstaller.New(legacyHrneoBinary, legacyNeoCommand, "aarch64-3.10", nil)
	inst.SetTestOpkgEnvironment(opkgDir, filepath.Join(tmp, "customfeeds.conf"))
	inst.SetOpkgBinary(opkgPath)
	if run != nil {
		inst.SetCommandRunner(run)
	}
	return inst
}

func restoreHydraPaths(t *testing.T, tmp string) {
	t.Helper()
	oldLegacyBin, oldLegacyNeo, oldPID := legacyHrneoBinary, legacyNeoCommand, pidFile
	legacyHrneoBinary = filepath.Join(tmp, "opt", "bin", "hrneo")
	legacyNeoCommand = filepath.Join(tmp, "opt", "bin", "neo")
	pidFile = filepath.Join(tmp, "var", "run", "hrneo.pid")
	t.Cleanup(func() {
		legacyHrneoBinary = oldLegacyBin
		legacyNeoCommand = oldLegacyNeo
		pidFile = oldPID
	})
}

func writeControlScript(t *testing.T, path, pidPath, logPath string, pid int) {
	t.Helper()
	script := fmt.Sprintf(`#!/bin/sh
set -eu
printf "%%s\n" "$1" >> %q
case "${1:-}" in
  stop)
    rm -f %q
    ;;
  start|restart)
    mkdir -p "$(dirname %q)"
    printf "%%s" %d > %q
    ;;
esac
`, logPath, pidPath, pidPath, pid, pidPath)
	mustWriteExecutableFile(t, path, script)
}

func mustWriteExecutableFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWriteTextFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readCommandLog(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Fields(strings.TrimSpace(string(data)))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func opkgTestCall(name string, args ...string) string {
	return filepath.Base(name) + " " + strings.Join(args, " ")
}

func assertContainsSequence(t *testing.T, got []string, want []string) {
	t.Helper()
	idx := 0
	for _, item := range got {
		if idx < len(want) && item == want[idx] {
			idx++
		}
	}
	if idx != len(want) {
		t.Fatalf("calls=%v do not contain sequence %v", got, want)
	}
}
