package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/sys/ndmsinfo"
)

func newTestAWGStore(t *testing.T) (*AWGTunnelStore, string) {
	t.Helper()
	dataDir := filepath.Join(t.TempDir(), "tunnels")
	lockDir := filepath.Join(t.TempDir(), "locks")
	return NewAWGTunnelStoreWithLockDir(dataDir, lockDir), dataDir
}

func TestAWGTunnelStoreListMissingDirReturnsEmptySlice(t *testing.T) {
	store, _ := newTestAWGStore(t)

	got, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if got == nil {
		t.Fatal("List() returned nil slice, want empty non-nil slice")
	}
	if len(got) != 0 {
		t.Fatalf("List() len = %d, want 0", len(got))
	}
}

func TestAWGTunnelStoreSaveDefaultsTypeAndDoesNotEscapeHTML(t *testing.T) {
	store, dataDir := newTestAWGStore(t)

	tun := &AWGTunnel{
		ID:   "awg1",
		Name: "test",
		Interface: AWGInterface{
			AWGObfuscation: AWGObfuscation{I1: "<sig>"},
		},
	}

	if err := store.Save(tun); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if tun.Type != "awg" {
		t.Fatalf("Save() mutated Type = %q, want awg", tun.Type)
	}

	raw, err := os.ReadFile(filepath.Join(dataDir, "awg1.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"type": "awg"`)) {
		t.Fatalf("saved JSON does not contain default type: %s", raw)
	}
	if bytes.Contains(raw, []byte(`\u003c`)) || bytes.Contains(raw, []byte(`\u003e`)) {
		t.Fatalf("saved JSON escaped HTML markers: %s", raw)
	}
	if !bytes.Contains(raw, []byte(`<sig>`)) {
		t.Fatalf("saved JSON does not preserve raw signature marker: %s", raw)
	}
}

func TestAWGTunnelStoreGetBackfillsLegacyDefaults(t *testing.T) {
	store, dataDir := newTestAWGStore(t)

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{
		"id": "legacy",
		"name": "legacy"
	}`)
	if err := os.WriteFile(filepath.Join(dataDir, "legacy.json"), raw, 0644); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get("legacy")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Type != "awg" {
		t.Fatalf("Type = %q, want awg", got.Type)
	}
	if !got.DefaultRoute {
		t.Fatal("DefaultRoute = false, want true for legacy tunnel")
	}
	if !got.DefaultRouteSet {
		t.Fatal("DefaultRouteSet = false, want true for legacy tunnel")
	}
}

func TestAWGTunnelStoreGetMissingReturnsNotFoundError(t *testing.T) {
	store, _ := newTestAWGStore(t)

	got, err := store.Get("missing")
	if err == nil {
		t.Fatal("Get() error = nil, want error")
	}
	if got != nil {
		t.Fatalf("Get() tunnel = %#v, want nil", got)
	}
	if !strings.Contains(err.Error(), "tunnel not found: missing") {
		t.Fatalf("error = %q, want tunnel not found", err)
	}
}

func TestAWGTunnelStoreGetInvalidJSONReturnsParseError(t *testing.T) {
	store, dataDir := newTestAWGStore(t)

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "bad.json"), []byte(`{"id":`), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get("bad")
	if err == nil {
		t.Fatal("Get() error = nil, want parse error")
	}
	if got != nil {
		t.Fatalf("Get() tunnel = %#v, want nil", got)
	}
	if !strings.Contains(err.Error(), "parse tunnel JSON") {
		t.Fatalf("error = %q, want parse tunnel JSON", err)
	}
}

func TestAWGTunnelStoreListSkipsNonJSONDirsAndInvalidJSON(t *testing.T) {
	store, dataDir := newTestAWGStore(t)

	if err := os.MkdirAll(filepath.Join(dataDir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "note.txt"), []byte("ignore"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "bad.json"), []byte(`{"id":`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "ok.json"), []byte(`{"id":"ok","name":"ok"}`), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("List() len = %d, want 1: %#v", len(got), got)
	}
	if got[0].ID != "ok" {
		t.Fatalf("List()[0].ID = %q, want ok", got[0].ID)
	}
	if got[0].Type != "awg" {
		t.Fatalf("List()[0].Type = %q, want awg", got[0].Type)
	}
	if !got[0].DefaultRoute || !got[0].DefaultRouteSet {
		t.Fatalf("legacy defaults not backfilled: %#v", got[0])
	}
}

func TestAWGTunnelStoreDeleteRemovesFile(t *testing.T) {
	store, dataDir := newTestAWGStore(t)

	if err := store.Save(&AWGTunnel{ID: "awg1", Name: "test"}); err != nil {
		t.Fatal(err)
	}

	if err := store.Delete("awg1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dataDir, "awg1.json")); !os.IsNotExist(err) {
		t.Fatalf("file still exists or unexpected stat error: %v", err)
	}
}

func TestAWGTunnelStoreDeleteMissingReturnsNotFound(t *testing.T) {
	store, _ := newTestAWGStore(t)

	err := store.Delete("missing")
	if err == nil {
		t.Fatal("Delete() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "tunnel not found: missing") {
		t.Fatalf("error = %q, want tunnel not found", err)
	}
}

func TestAWGTunnelStoreExists(t *testing.T) {
	store, _ := newTestAWGStore(t)

	if store.Exists("awg1") {
		t.Fatal("Exists() = true before Save, want false")
	}

	if err := store.Save(&AWGTunnel{ID: "awg1", Name: "test"}); err != nil {
		t.Fatal(err)
	}

	if !store.Exists("awg1") {
		t.Fatal("Exists() = false after Save, want true")
	}
}

func TestAWGTunnelStoreClearRuntimeStateClearsActiveWANAndStartedAt(t *testing.T) {
	store, _ := newTestAWGStore(t)

	if err := store.Save(&AWGTunnel{
		ID:        "awg1",
		Name:      "test",
		ActiveWAN: "ISP",
		StartedAt: "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}

	store.ClearRuntimeState("awg1")

	got, err := store.Get("awg1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ActiveWAN != "" {
		t.Fatalf("ActiveWAN = %q, want empty", got.ActiveWAN)
	}
	if got.StartedAt != "" {
		t.Fatalf("StartedAt = %q, want empty", got.StartedAt)
	}
	if got.Name != "test" {
		t.Fatalf("Name = %q, want test", got.Name)
	}
}

func TestAWGTunnelStoreClearRuntimeStateMissingIsNoop(t *testing.T) {
	store, _ := newTestAWGStore(t)
	store.ClearRuntimeState("missing")
}

func TestAWGTunnelStoreNextAvailableIDOS4Fallback(t *testing.T) {
	ndmsinfo.Reset()
	t.Cleanup(ndmsinfo.Reset)

	store, _ := newTestAWGStore(t)

	if err := store.Save(&AWGTunnel{ID: "awgm0", Name: "zero"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&AWGTunnel{ID: "awgm2", Name: "two"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&AWGTunnel{ID: "awg10", Name: "os5-style"}); err != nil {
		t.Fatal(err)
	}

	got, err := store.NextAvailableID()
	if err != nil {
		t.Fatalf("NextAvailableID() error = %v", err)
	}
	if got != "awgm1" {
		t.Fatalf("NextAvailableID() = %q, want awgm1", got)
	}
}
