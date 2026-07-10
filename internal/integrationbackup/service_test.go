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
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/hydraroute"
	"github.com/hoaxisr/awg-manager/internal/storage"
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
