package hydraroute

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestDetect_NotInstalled(t *testing.T) {
	bin, _ := withFakeHydraPaths(t)
	_ = bin

	got := Detect()
	if got.Installed {
		t.Fatalf("Installed = true, want false")
	}
	if got.Running {
		t.Fatalf("Running = true, want false")
	}
	if got.ProcessState != StateNotInstalled {
		t.Fatalf("ProcessState = %q, want %q", got.ProcessState, StateNotInstalled)
	}
}

func TestDetect_InstalledStopped(t *testing.T) {
	bin, _ := withFakeHydraPaths(t)
	mustWriteExecutable(t, bin)

	got := Detect()
	if !got.Installed {
		t.Fatalf("Installed = false, want true")
	}
	if got.Running {
		t.Fatalf("Running = true, want false")
	}
	if got.ProcessState != StateStopped {
		t.Fatalf("ProcessState = %q, want %q", got.ProcessState, StateStopped)
	}
}

func TestDetect_Running(t *testing.T) {
	bin, pid := withFakeHydraPaths(t)
	mustWriteExecutable(t, bin)
	self := os.Getpid()
	mustWriteText(t, pid, []byte(strconv.Itoa(self)))

	got := Detect()
	if !got.Installed {
		t.Fatalf("Installed = false, want true")
	}
	if !got.Running {
		t.Fatalf("Running = false, want true")
	}
	if got.PID != self {
		t.Fatalf("PID = %d, want %d", got.PID, self)
	}
	if got.ProcessState != StateRunning {
		t.Fatalf("ProcessState = %q, want %q", got.ProcessState, StateRunning)
	}
}

func TestDetect_DeadStalePID(t *testing.T) {
	bin, pid := withFakeHydraPaths(t)
	mustWriteExecutable(t, bin)
	mustWriteText(t, pid, []byte("999999"))

	got := Detect()
	if !got.Installed {
		t.Fatalf("Installed = false, want true")
	}
	if got.Running {
		t.Fatalf("Running = true, want false")
	}
	if got.StalePID != 999999 {
		t.Fatalf("StalePID = %d, want 999999", got.StalePID)
	}
	if got.ProcessState != StateDead {
		t.Fatalf("ProcessState = %q, want %q", got.ProcessState, StateDead)
	}
}

func TestParseVersionOutput(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "2.4.1", want: "2.4.1"},
		{in: "v2.4.1", want: "2.4.1"},
		{in: "HydraRoute Neo 2.4.1", want: "2.4.1"},
		{in: "hrneo version 2.4.1", want: "2.4.1"},
	}
	for _, tc := range tests {
		got := parseVersionOutput(tc.in)
		if got != tc.want {
			t.Fatalf("parseVersionOutput(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolvePaths_UsesOfficialLayout(t *testing.T) {
	tmp := t.TempDir()
	oldLegacyBin, oldLegacyNeo, oldPID := legacyHrneoBinary, legacyNeoCommand, pidFile
	legacyHrneoBinary = filepath.Join(tmp, "opt", "bin", "hrneo")
	legacyNeoCommand = filepath.Join(tmp, "opt", "bin", "neo")
	pidFile = filepath.Join(tmp, "var", "run", "hrneo.pid")
	t.Cleanup(func() {
		legacyHrneoBinary = oldLegacyBin
		legacyNeoCommand = oldLegacyNeo
		pidFile = oldPID
	})

	mustWriteExecutable(t, legacyHrneoBinary)
	mustWriteExecutable(t, legacyNeoCommand)

	got := ResolvePaths()
	if got.Binary != legacyHrneoBinary || got.Control != legacyNeoCommand {
		t.Fatalf("ResolvePaths()=%+v want official legacy paths", got)
	}
}

func withFakeHydraPaths(t *testing.T) (string, string) {
	t.Helper()
	tmp := t.TempDir()
	oldLegacyBin, oldLegacyNeo, oldPID := legacyHrneoBinary, legacyNeoCommand, pidFile
	legacyHrneoBinary = filepath.Join(tmp, "hrneo")
	legacyNeoCommand = filepath.Join(tmp, "neo")
	pidFile = filepath.Join(tmp, "hrneo.pid")
	t.Cleanup(func() {
		legacyHrneoBinary = oldLegacyBin
		legacyNeoCommand = oldLegacyNeo
		pidFile = oldPID
	})
	return legacyHrneoBinary, pidFile
}

func mustWriteExecutable(t *testing.T, path string) {
	t.Helper()
	mustWriteText(t, path, []byte("#!/bin/sh\nexit 0\n"))
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
}

func mustWriteText(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
