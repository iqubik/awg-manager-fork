package hydraroute

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRefreshStatus_EmptyVersionCachedWithinTTL(t *testing.T) {
	tmp := t.TempDir()
	fakeHrneo := filepath.Join(tmp, "hrneo")
	counter := filepath.Join(tmp, "probe.count")
	fakePID := filepath.Join(tmp, "hrneo.pid")
	fakeNeo := filepath.Join(tmp, "neo")

	script := "#!/bin/sh\n" +
		"echo x >> \"" + counter + "\"\n" +
		"echo unknown\n" // intentionally not semver
	if err := os.WriteFile(fakeHrneo, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake hrneo: %v", err)
	}

	oldLegacyBin, oldLegacyNeo, oldPID := legacyHrneoBinary, legacyNeoCommand, pidFile
	legacyHrneoBinary = fakeHrneo
	pidFile = fakePID
	legacyNeoCommand = fakeNeo
	t.Cleanup(func() {
		legacyHrneoBinary = oldLegacyBin
		pidFile = oldPID
		legacyNeoCommand = oldLegacyNeo
	})

	s := NewService(nil, nil)
	_ = s.RefreshStatus()
	_ = s.RefreshStatus()

	raw, err := os.ReadFile(counter)
	if err != nil {
		t.Fatalf("read counter: %v", err)
	}
	lines := strings.Count(string(raw), "\n")
	if lines != 3 {
		t.Fatalf("version probe calls = %d, want 3 (single probe cycle)", lines)
	}

	if s.versionCached != "" {
		t.Fatalf("versionCached=%q want empty", s.versionCached)
	}
	if s.versionFetchedAt.IsZero() {
		t.Fatalf("versionFetchedAt is zero, want non-zero")
	}

	// Sanity: once TTL expires, next refresh may probe again.
	s.versionFetchedAt = time.Now().Add(-versionCacheTTL - time.Second)
	_ = s.RefreshStatus()
	raw2, err := os.ReadFile(counter)
	if err != nil {
		t.Fatalf("read counter(2): %v", err)
	}
	lines2 := strings.Count(string(raw2), "\n")
	if lines2 != 6 {
		t.Fatalf("version probe calls after TTL = %d, want 6 (two probe cycles)", lines2)
	}
}

func TestRefreshStatus_ReprobesWhenBinaryFingerprintChanges(t *testing.T) {
	tmp := t.TempDir()
	fakeHrneo := filepath.Join(tmp, "hrneo")
	counter := filepath.Join(tmp, "probe.count")
	fakePID := filepath.Join(tmp, "hrneo.pid")
	fakeNeo := filepath.Join(tmp, "neo")

	writeScript := func(version string, extra string) {
		t.Helper()
		script := "#!/bin/sh\n" +
			"echo x >> \"" + counter + "\"\n" +
			"echo \"HydraRoute Neo " + version + "\"\n" +
			extra + "\n"
		if err := os.WriteFile(fakeHrneo, []byte(script), 0o755); err != nil {
			t.Fatalf("write fake hrneo: %v", err)
		}
	}

	writeScript("2.4.1", "")

	oldLegacyBin, oldLegacyNeo, oldPID := legacyHrneoBinary, legacyNeoCommand, pidFile
	legacyHrneoBinary = fakeHrneo
	pidFile = fakePID
	legacyNeoCommand = fakeNeo
	t.Cleanup(func() {
		legacyHrneoBinary = oldLegacyBin
		pidFile = oldPID
		legacyNeoCommand = oldLegacyNeo
	})

	s := NewService(nil, nil)
	st1 := s.RefreshStatus()
	if st1.Version != "2.4.1" {
		t.Fatalf("first version=%q want 2.4.1", st1.Version)
	}

	raw1, err := os.ReadFile(counter)
	if err != nil {
		t.Fatalf("read counter(1): %v", err)
	}
	if got := strings.Count(string(raw1), "\n"); got != 1 {
		t.Fatalf("probe calls after first refresh=%d want 1", got)
	}

	st2 := s.RefreshStatus()
	if st2.Version != "2.4.1" {
		t.Fatalf("second version=%q want 2.4.1", st2.Version)
	}
	raw2, err := os.ReadFile(counter)
	if err != nil {
		t.Fatalf("read counter(2): %v", err)
	}
	if got := strings.Count(string(raw2), "\n"); got != 1 {
		t.Fatalf("probe calls within TTL=%d want 1", got)
	}

	// Change binary content+size to force fingerprint change.
	writeScript("2.4.2", "echo "+strconv.Quote("changed"))
	time.Sleep(15 * time.Millisecond)

	st3 := s.RefreshStatus()
	if st3.Version != "2.4.2" {
		t.Fatalf("third version=%q want 2.4.2", st3.Version)
	}
	raw3, err := os.ReadFile(counter)
	if err != nil {
		t.Fatalf("read counter(3): %v", err)
	}
	if got := strings.Count(string(raw3), "\n"); got != 2 {
		t.Fatalf("probe calls after fingerprint change=%d want 2", got)
	}
}
