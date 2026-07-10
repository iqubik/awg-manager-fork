package integrationbackup

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/hydraroute"
	"github.com/hoaxisr/awg-manager/internal/storage"
	"github.com/hoaxisr/awg-manager/internal/sys/routerclock"
)

type fakeSingbox struct {
	dir          string
	restartCalls int
	restartErr   error
	running      bool
}

func (f *fakeSingbox) ConfigDir() string { return f.dir }
func (f *fakeSingbox) ValidateConfigDir(_ context.Context) error {
	data, err := os.ReadFile(filepath.Join(f.dir, "00-base.json"))
	if err != nil {
		return err
	}
	if strings.Contains(string(data), "invalid") {
		return errors.New("sing-box config invalid")
	}
	return nil
}
func (f *fakeSingbox) ValidateConfigPath(_ context.Context, configDir string) error {
	data, err := os.ReadFile(filepath.Join(configDir, "00-base.json"))
	if err != nil {
		return err
	}
	if strings.Contains(string(data), "invalid") {
		return errors.New("sing-box config invalid")
	}
	return nil
}
func (f *fakeSingbox) Control(_ context.Context, action string) error {
	if action == "restart" {
		f.restartCalls++
		if f.restartErr != nil {
			return f.restartErr
		}
	}
	return nil
}
func (f *fakeSingbox) IsRunning() (bool, int) { return f.running, 123 }

type fakeHydra struct {
	running   bool
	failStart error
}

func (f *fakeHydra) ReadConfig() (*hydraroute.Config, error)           { return &hydraroute.Config{}, nil }
func (f *fakeHydra) ListRules() ([]hydraroute.HRRule, []string, error) { return nil, nil, nil }
func (f *fakeHydra) Control(action string) error {
	switch action {
	case "start":
		if f.failStart != nil {
			return f.failStart
		}
		f.running = true
	case "stop":
		f.running = false
	}
	return nil
}
func (f *fakeHydra) GetStatus() hydraroute.Status {
	return hydraroute.Status{Running: f.running, Installed: true}
}

func TestSingboxBackupDryRunDoesNotWrite(t *testing.T) {
	svc, store, sb := newTestService(t)
	writeFileForTest(t, filepath.Join(sb.dir, "00-base.json"), `{"log":{"level":"info"}}`)
	writeFileForTest(t, filepath.Join(sb.dir, "20-router.json"), `{"route":{}}`)
	writeFileForTest(t, filepath.Join(svc.dataDir, "deviceproxy.json"), `{"enabled":true}`)

	cur, err := store.Get()
	if err != nil {
		t.Fatalf("settings get: %v", err)
	}
	cur.CreateNDMSProxyForSingbox = false
	cur.SingboxRouter.Enabled = true
	if err := store.Save(cur); err != nil {
		t.Fatalf("settings save: %v", err)
	}

	_, payload, err := svc.CreateBackup(context.Background(), ComponentSingbox)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	writeFileForTest(t, filepath.Join(sb.dir, "00-base.json"), `{"log":{"level":"debug"}}`)
	cur, _ = store.Get()
	cur.CreateNDMSProxyForSingbox = true
	if err := store.Save(cur); err != nil {
		t.Fatalf("settings mutate: %v", err)
	}

	resp, err := svc.Restore(context.Background(), ComponentSingbox, payload, true)
	if err != nil {
		t.Fatalf("Restore dryRun: %v", err)
	}
	if !resp.DryRun {
		t.Fatal("expected dryRun=true")
	}
	data, _ := os.ReadFile(filepath.Join(sb.dir, "00-base.json"))
	if strings.Contains(string(data), `"info"`) {
		t.Fatal("dry-run should not overwrite file")
	}
	cur, _ = store.Get()
	if !cur.CreateNDMSProxyForSingbox {
		t.Fatal("dry-run should not overwrite settings")
	}
}

func TestRestoreRejectsTraversalArchive(t *testing.T) {
	svc, _, _ := newTestService(t)
	manifest := Manifest{
		Type:          backupType,
		FormatVersion: formatVersion,
		Component:     ComponentSingbox,
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create(manifestName)
	raw, _ := json.Marshal(manifest)
	_, _ = w.Write(raw)
	bad, _ := zw.Create(archiveRoot + "/../evil.txt")
	_, _ = bad.Write([]byte("boom"))
	_ = zw.Close()

	if _, err := svc.Restore(context.Background(), ComponentSingbox, buf.Bytes(), true); err == nil {
		t.Fatal("expected traversal error")
	}
}

func TestRestoreRejectsChecksumMismatch(t *testing.T) {
	svc, _, sb := newTestService(t)
	writeFileForTest(t, filepath.Join(sb.dir, "00-base.json"), `{"log":{"level":"info"}}`)
	_, payload, err := svc.CreateBackup(context.Background(), ComponentSingbox)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	tampered, err := rewriteArchiveEntry(payload, archiveEntryForSource("/opt/etc/awg-manager/singbox/config.d/00-base.json"), []byte(`{"log":{"level":"warn"}}`), false)
	if err != nil {
		t.Fatalf("rewriteArchiveEntry: %v", err)
	}
	if _, err := svc.Restore(context.Background(), ComponentSingbox, tampered, true); err == nil || !strings.Contains(err.Error(), "checksum не совпал") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func TestRestoreDryRunRejectsInvalidSettingsSnapshot(t *testing.T) {
	svc, store, sb := newTestService(t)
	writeFileForTest(t, filepath.Join(sb.dir, "00-base.json"), `{"log":{"level":"info"}}`)
	cur, err := store.Get()
	if err != nil {
		t.Fatalf("settings get: %v", err)
	}
	cur.CreateNDMSProxyForSingbox = true
	if err := store.Save(cur); err != nil {
		t.Fatalf("settings save: %v", err)
	}

	_, payload, err := svc.CreateBackup(context.Background(), ComponentSingbox)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	payload, err = mutateArchive(payload, func(files map[string][]byte, manifest *Manifest) error {
		settingsPath := settingsEntryForComponent(ComponentSingbox)
		files[settingsPath] = []byte(`{"broken":`)
		manifest.Settings.SHA256 = sha256Hex(files[settingsPath])
		return nil
	})
	if err != nil {
		t.Fatalf("mutateArchive: %v", err)
	}

	if _, err := svc.Restore(context.Background(), ComponentSingbox, payload, true); err == nil {
		t.Fatal("expected invalid settings snapshot error")
	}
	data, err := os.ReadFile(filepath.Join(sb.dir, "00-base.json"))
	if err != nil {
		t.Fatalf("read file after failed dry-run: %v", err)
	}
	if string(data) != `{"log":{"level":"info"}}` {
		t.Fatalf("dry-run should not touch live files: %s", data)
	}
}

func TestRestoreRollsBackOnValidationFailure(t *testing.T) {
	svc, store, sb := newTestService(t)
	target := filepath.Join(sb.dir, "00-base.json")
	writeFileForTest(t, target, `{"log":{"level":"info"}}`)

	cur, err := store.Get()
	if err != nil {
		t.Fatalf("settings get: %v", err)
	}
	cur.CreateNDMSProxyForSingbox = false
	if err := store.Save(cur); err != nil {
		t.Fatalf("settings save: %v", err)
	}

	_, payload, err := svc.CreateBackup(context.Background(), ComponentSingbox)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	writeFileForTest(t, target, `{"log":{"level":"debug"}}`)
	cur, _ = store.Get()
	cur.CreateNDMSProxyForSingbox = true
	if err := store.Save(cur); err != nil {
		t.Fatalf("settings mutate: %v", err)
	}

	bad := []byte(`{"log":{"level":"invalid"}}`)
	payload, err = rewriteArchiveEntry(payload, archiveEntryForSource("/opt/etc/awg-manager/singbox/config.d/00-base.json"), bad, true)
	if err != nil {
		t.Fatalf("rewriteArchiveEntry: %v", err)
	}
	_, err = svc.Restore(context.Background(), ComponentSingbox, payload, false)
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("expected validation error, got %v", err)
	}

	got, _ := os.ReadFile(target)
	if string(got) != `{"log":{"level":"debug"}}` {
		t.Fatalf("rollback did not restore original file: %s", got)
	}
	cur, _ = store.Get()
	if !cur.CreateNDMSProxyForSingbox {
		t.Fatal("rollback did not restore original settings")
	}
	if sb.restartCalls != 0 {
		t.Fatalf("temp validation should reject archive before restart, got %d calls", sb.restartCalls)
	}
}

func TestRestoreDryRunValidatesSingboxTempConfig(t *testing.T) {
	svc, _, sb := newTestService(t)
	target := filepath.Join(sb.dir, "00-base.json")
	writeFileForTest(t, target, `{"log":{"level":"info"}}`)

	_, payload, err := svc.CreateBackup(context.Background(), ComponentSingbox)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	payload, err = rewriteArchiveEntry(
		payload,
		archiveEntryForSource("/opt/etc/awg-manager/singbox/config.d/00-base.json"),
		[]byte(`{"log":{"level":"invalid"}}`),
		true,
	)
	if err != nil {
		t.Fatalf("rewriteArchiveEntry: %v", err)
	}

	if _, err := svc.Restore(context.Background(), ComponentSingbox, payload, true); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("expected dry-run validation error, got %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target after dry-run: %v", err)
	}
	if string(got) != `{"log":{"level":"info"}}` {
		t.Fatalf("dry-run changed file content: %s", got)
	}
}

func TestRestoreDryRunPlansDeleteForStaleSingboxFile(t *testing.T) {
	svc, _, sb := newTestService(t)
	writeFileForTest(t, filepath.Join(sb.dir, "00-base.json"), `{"log":{"level":"info"}}`)

	_, payload, err := svc.CreateBackup(context.Background(), ComponentSingbox)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	stalePath := filepath.Join(sb.dir, "99-old.json")
	writeFileForTest(t, stalePath, `{"route":{"final":"old"}}`)

	resp, err := svc.Restore(context.Background(), ComponentSingbox, payload, true)
	if err != nil {
		t.Fatalf("Restore dryRun: %v", err)
	}

	for _, outcome := range resp.Outcomes {
		if outcome.Path == "/opt/etc/awg-manager/singbox/config.d/99-old.json" {
			if outcome.Action != "planned_delete" {
				t.Fatalf("unexpected action for stale file: %s", outcome.Action)
			}
			return
		}
	}
	t.Fatal("expected dry-run delete outcome for stale file")
}

func TestRestoreDeletesStaleSingboxFile(t *testing.T) {
	svc, _, sb := newTestService(t)
	writeFileForTest(t, filepath.Join(sb.dir, "00-base.json"), `{"log":{"level":"info"}}`)

	_, payload, err := svc.CreateBackup(context.Background(), ComponentSingbox)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	stalePath := filepath.Join(sb.dir, "99-old.json")
	writeFileForTest(t, stalePath, `{"route":{"final":"old"}}`)

	if _, err := svc.Restore(context.Background(), ComponentSingbox, payload, false); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("expected stale file to be removed, got err=%v", err)
	}
}

func TestRestoreDeletesStaleSingboxAWGMJSON(t *testing.T) {
	svc, _, sb := newTestService(t)
	writeFileForTest(t, filepath.Join(sb.dir, "00-base.json"), `{"log":{"level":"info"}}`)

	_, payload, err := svc.CreateBackup(context.Background(), ComponentSingbox)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	stalePath := filepath.Join(svc.dataDir, "deviceproxy.json")
	writeFileForTest(t, stalePath, `{"enabled":true}`)

	if _, err := svc.Restore(context.Background(), ComponentSingbox, payload, false); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("expected stale deviceproxy.json to be removed, got err=%v", err)
	}
}

func TestRestoreRollbackRestoresDeletedSingboxFile(t *testing.T) {
	svc, _, sb := newTestService(t)
	writeFileForTest(t, filepath.Join(sb.dir, "00-base.json"), `{"log":{"level":"info"}}`)

	_, payload, err := svc.CreateBackup(context.Background(), ComponentSingbox)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	stalePath := filepath.Join(sb.dir, "99-old.json")
	writeFileForTest(t, stalePath, `{"route":{"final":"old"}}`)
	writeFileForTest(t, filepath.Join(sb.dir, "00-base.json"), `{"log":{"level":"debug"}}`)
	sb.restartErr = errors.New("restart failed")

	if _, err := svc.Restore(context.Background(), ComponentSingbox, payload, false); err == nil || !strings.Contains(err.Error(), "restart failed") {
		t.Fatalf("expected restart failure, got %v", err)
	}

	data, err := os.ReadFile(stalePath)
	if err != nil {
		t.Fatalf("expected stale file restored: %v", err)
	}
	if string(data) != `{"route":{"final":"old"}}` {
		t.Fatalf("unexpected stale file content after rollback: %s", data)
	}
	baseData, err := os.ReadFile(filepath.Join(sb.dir, "00-base.json"))
	if err != nil {
		t.Fatalf("read base after rollback: %v", err)
	}
	if string(baseData) != `{"log":{"level":"debug"}}` {
		t.Fatalf("base file rollback mismatch: %s", baseData)
	}
}

func TestHydraRestoreDeletesStaleOptionalFiles(t *testing.T) {
	svc, _, _ := newTestService(t)
	writeFileForTest(t, filepath.Join(svc.hydraDir, "hrneo.conf"), "autoStart=1\n")

	_, payload, err := svc.CreateBackup(context.Background(), ComponentHydraRoute)
	if err != nil {
		t.Fatalf("CreateBackup hydra: %v", err)
	}

	stalePath := filepath.Join(svc.hydraDir, "domain.conf")
	writeFileForTest(t, stalePath, "example.com\n")

	if _, err := svc.Restore(context.Background(), ComponentHydraRoute, payload, false); err != nil {
		t.Fatalf("Restore hydra: %v", err)
	}
	if _, err := os.Stat(stalePath); !os.IsNotExist(err) {
		t.Fatalf("expected stale hydra file removed, got err=%v", err)
	}
}

func TestRestoreRejectsTooManyArchiveEntries(t *testing.T) {
	svc, _, _ := newTestService(t)
	manifest := Manifest{
		Type:          backupType,
		FormatVersion: formatVersion,
		Component:     ComponentSingbox,
	}
	entries := make(map[string]zipTestEntry, maxArchiveEntries)
	for i := 0; i < maxArchiveEntries; i++ {
		entries[archiveRoot+"/extra-"+strconv.Itoa(i)] = zipTestEntry{data: []byte("x")}
	}
	payload := buildArchiveForTest(t, manifest, entries)
	if _, err := svc.Restore(context.Background(), ComponentSingbox, payload, true); err == nil || !strings.Contains(err.Error(), "слишком много entries") {
		t.Fatalf("expected too-many-entries error, got %v", err)
	}
}

func TestRestoreRejectsOversizedArchiveEntry(t *testing.T) {
	svc, _, _ := newTestService(t)
	manifest := Manifest{
		Type:          backupType,
		FormatVersion: formatVersion,
		Component:     ComponentSingbox,
	}
	payload := buildArchiveForTest(t, manifest, map[string]zipTestEntry{
		archiveRoot + "/huge.bin": {data: bytes.Repeat([]byte("a"), maxArchiveEntryBytes+1)},
	})
	if _, err := svc.Restore(context.Background(), ComponentSingbox, payload, true); err == nil || !strings.Contains(err.Error(), "entry слишком большой") {
		t.Fatalf("expected oversized-entry error, got %v", err)
	}
}

func TestRestoreRejectsArchiveWithTooLargeTotalUncompressedSize(t *testing.T) {
	svc, _, _ := newTestService(t)
	manifest := Manifest{
		Type:          backupType,
		FormatVersion: formatVersion,
		Component:     ComponentSingbox,
	}
	entries := map[string]zipTestEntry{}
	for i := 0; i < 5; i++ {
		entries[archiveRoot+"/chunk-"+strconv.Itoa(i)] = zipTestEntry{data: bytes.Repeat([]byte("a"), 7<<20)}
	}
	payload := buildArchiveForTest(t, manifest, entries)
	if _, err := svc.Restore(context.Background(), ComponentSingbox, payload, true); err == nil || !strings.Contains(err.Error(), "суммарный размер распакованного архива слишком большой") {
		t.Fatalf("expected total-size error, got %v", err)
	}
}

func TestRestoreRejectsArchiveWithSymlinkEntry(t *testing.T) {
	svc, _, _ := newTestService(t)
	manifest := Manifest{
		Type:          backupType,
		FormatVersion: formatVersion,
		Component:     ComponentSingbox,
	}
	payload := buildArchiveForTest(t, manifest, map[string]zipTestEntry{
		archiveRoot + "/link": {data: []byte("target"), mode: os.ModeSymlink | 0o777},
	})
	if _, err := svc.Restore(context.Background(), ComponentSingbox, payload, true); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
}

func TestRestoreRejectsFileArchivePathMismatch(t *testing.T) {
	svc, _, sb := newTestService(t)
	writeFileForTest(t, filepath.Join(sb.dir, "00-base.json"), `{"log":{"level":"info"}}`)
	_, payload, err := svc.CreateBackup(context.Background(), ComponentSingbox)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	payload, err = mutateArchive(payload, func(files map[string][]byte, manifest *Manifest) error {
		manifest.Files[0].ArchivePath = archiveRoot + "/files/not-the-source.json"
		return nil
	})
	if err != nil {
		t.Fatalf("mutateArchive: %v", err)
	}
	if _, err := svc.Restore(context.Background(), ComponentSingbox, payload, true); err == nil || !strings.Contains(err.Error(), "archivePath не соответствует sourcePath") {
		t.Fatalf("expected archivePath mismatch, got %v", err)
	}
}

func TestRestoreRejectsSettingsArchivePathMismatch(t *testing.T) {
	svc, _, sb := newTestService(t)
	writeFileForTest(t, filepath.Join(sb.dir, "00-base.json"), `{"log":{"level":"info"}}`)
	_, payload, err := svc.CreateBackup(context.Background(), ComponentSingbox)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	payload, err = mutateArchive(payload, func(files map[string][]byte, manifest *Manifest) error {
		manifest.Settings.ArchivePath = settingsPrefix + "/other.json"
		return nil
	})
	if err != nil {
		t.Fatalf("mutateArchive: %v", err)
	}
	if _, err := svc.Restore(context.Background(), ComponentSingbox, payload, true); err == nil || !strings.Contains(err.Error(), "settings archivePath не соответствует компоненту") {
		t.Fatalf("expected settings archivePath mismatch, got %v", err)
	}
}

func newTestService(t *testing.T) (*Service, *storage.SettingsStore, *fakeSingbox) {
	t.Helper()
	root := t.TempDir()
	store := storage.NewSettingsStore(root)
	if _, err := store.Load(); err != nil {
		t.Fatalf("settings load: %v", err)
	}
	sb := &fakeSingbox{dir: filepath.Join(root, "singbox", "config.d"), running: true}
	if err := os.MkdirAll(sb.dir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	svc := NewService(root, store, sb, &fakeHydra{})
	svc.hydraDir = filepath.Join(root, "HydraRoute")
	return svc, store, sb
}

func writeFileForTest(t *testing.T, target string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", target, err)
	}
	if err := os.WriteFile(target, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", target, err)
	}
}

func rewriteArchiveEntry(payload []byte, target string, replacement []byte, updateManifest bool) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return nil, err
	}
	files := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		files[f.Name] = data
	}
	files[target] = replacement
	var manifest Manifest
	if err := json.Unmarshal(files[manifestName], &manifest); err != nil {
		return nil, err
	}
	if updateManifest {
		for i := range manifest.Files {
			if manifest.Files[i].ArchivePath == target {
				manifest.Files[i].SHA256 = sha256Hex(replacement)
			}
		}
		raw, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return nil, err
		}
		files[manifestName] = raw
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(files[name]); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type zipTestEntry struct {
	data []byte
	mode os.FileMode
}

func mutateArchive(payload []byte, mutator func(files map[string][]byte, manifest *Manifest) error) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return nil, err
	}
	files := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		files[f.Name] = data
	}
	var manifest Manifest
	if err := json.Unmarshal(files[manifestName], &manifest); err != nil {
		return nil, err
	}
	if err := mutator(files, &manifest); err != nil {
		return nil, err
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	files[manifestName] = raw

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(files[name]); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildArchiveForTest(t *testing.T, manifest Manifest, entries map[string]zipTestEntry) []byte {
	t.Helper()
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry := entries[name]
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if entry.mode != 0 {
			h.SetMode(entry.mode)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatalf("create zip header %s: %v", name, err)
		}
		if _, err := w.Write(entry.data); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	w, err := zw.Create(manifestName)
	if err != nil {
		t.Fatalf("create manifest entry: %v", err)
	}
	if _, err := w.Write(raw); err != nil {
		t.Fatalf("write manifest entry: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	return buf.Bytes()
}

func TestSanitizeFilenameToken(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"MSK", "MSK"},
		{"moscow", "moscow"},
		{"MSK+3", "MSK-3"},
		{"UTC+0", "UTC-0"},
		{"Europe/Moscow", "Europe-Moscow"},
		{"", ""},
	}
	for _, tt := range tests {
		got := sanitizeFilenameToken(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFilenameToken(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBackupTimestampForFilename(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	clock := routerclock.Info{
		Now:           time.Date(2026, 7, 10, 14, 32, 10, 0, loc),
		ZoneName:      "MSK",
		OffsetMinutes: 180,
		Location:      loc,
	}
	got := backupTimestampForFilename(clock.Now, clock.ZoneName)
	if got != "20260710-143210-MSK" {
		t.Errorf("backupTimestampForFilename() = %q, want %q", got, "20260710-143210-MSK")
	}

	clock.ZoneName = ""
	got = backupTimestampForFilename(clock.Now, "")
	if got != "20260710-143210" {
		t.Errorf("backupTimestampForFilename() = %q, want %q", got, "20260710-143210")
	}
}

func TestCreateBackupFilenameUsesRouterClock(t *testing.T) {
	svc, _, sb := newTestService(t)
	writeFileForTest(t, filepath.Join(sb.dir, "00-base.json"), `{"log":{"level":"info"}}`)

	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	svc.now = func() routerclock.Info {
		return routerclock.Info{
			Now:           time.Date(2026, 7, 10, 14, 32, 10, 0, loc),
			ZoneName:      "MSK",
			OffsetMinutes: 180,
			Location:      loc,
		}
	}

	filename, _, err := svc.CreateBackup(context.Background(), ComponentSingbox)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if filename != "awgm-singbox-backup-20260710-143210-MSK.zip" {
		t.Errorf("CreateBackup filename = %q, want %q", filename, "awgm-singbox-backup-20260710-143210-MSK.zip")
	}
}

func TestSetZipEntryLocalModTime(t *testing.T) {
	h := &zip.FileHeader{
		Name:   "test.txt",
		Method: zip.Deflate,
	}
	loc := time.FixedZone("MSK", 3*3600)
	ts := time.Date(2026, 7, 10, 5, 22, 0, 0, loc)
	setZipEntryLocalModTime(h, ts)

	if h.Modified.Year() != 2026 || h.Modified.Month() != 7 || h.Modified.Day() != 10 ||
		h.Modified.Hour() != 5 || h.Modified.Minute() != 22 || h.Modified.Second() != 0 {
		t.Errorf("Modified = %v, want wall-clock 2026-07-10 05:22:00", h.Modified)
	}
	if h.ModifiedTime != uint16((5<<11)|(22<<5)|(0/2)) {
		t.Errorf("ModifiedTime = %x, want %x", h.ModifiedTime, uint16((5<<11)|(22<<5)|(0/2)))
	}
	expectedDate := uint16((2026-1980)<<9 | (7 << 5) | 10)
	if h.ModifiedDate != expectedDate {
		t.Errorf("ModifiedDate = %x, want %x", h.ModifiedDate, expectedDate)
	}
}

func TestCreateBackupZipEntriesUseRouterWallClock(t *testing.T) {
	svc, _, sb := newTestService(t)
	writeFileForTest(t, filepath.Join(sb.dir, "00-base.json"), `{"log":{"level":"info"}}`)

	loc := time.FixedZone("MSK", 3*3600)
	svc.now = func() routerclock.Info {
		return routerclock.Info{
			Now:           time.Date(2026, 7, 10, 5, 22, 0, 0, loc),
			ZoneName:      "MSK",
			OffsetMinutes: 180,
			Location:      loc,
		}
	}

	_, payload, err := svc.CreateBackup(context.Background(), ComponentSingbox)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}

	expected := time.Date(2026, 7, 10, 5, 22, 0, 0, time.UTC)
	checks := map[string]bool{
		"awgm-backup/manifest.json":                                           false,
		"awgm-backup/settings/singbox.json":                                   false,
		"awgm-backup/files/opt/etc/awg-manager/singbox/config.d/00-base.json": false,
	}
	for _, f := range zr.File {
		if _, ok := checks[f.Name]; ok {
			checks[f.Name] = true
			if !f.Modified.Equal(expected) {
				t.Errorf("%s Modified = %v, want %v", f.Name, f.Modified, expected)
			}
		}
	}
	for name, ok := range checks {
		if !ok {
			t.Errorf("missing expected zip entry: %s", name)
		}
	}
}
