package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/events"
	"github.com/hoaxisr/awg-manager/internal/managed"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

func TestExport_MethodGuard(t *testing.T) {
	h := &ManagedServerBackupHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/managed/export", nil)
	w := httptest.NewRecorder()
	h.Export(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", w.Code)
	}
}

func TestImport_MethodGuard(t *testing.T) {
	h := &ManagedServerBackupHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/managed/import", nil)
	w := httptest.NewRecorder()
	h.Import(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", w.Code)
	}
}

func TestDrift_MethodGuard(t *testing.T) {
	h := &ManagedServerBackupHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/managed/drift", nil)
	w := httptest.NewRecorder()
	h.Drift(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", w.Code)
	}
}

func TestRestoreDrift_MethodGuard(t *testing.T) {
	h := &ManagedServerBackupHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/managed/restore-drift", nil)
	w := httptest.NewRecorder()
	h.RestoreDrift(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want 405", w.Code)
	}
}

func TestImport_RequiresTypeAndVersion(t *testing.T) {
	h := &ManagedServerBackupHandler{svc: &managed.Service{}, bus: events.NewBus()}
	req := httptest.NewRequest(http.MethodPost, "/api/managed/import", bytes.NewBufferString(`{"managedServers":[],"options":{"allowRenumber":false}}`))
	w := httptest.NewRecorder()
	h.Import(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want 400", w.Code)
	}
}

func TestBackupDTO_RoundtripPreservesI1ToI5(t *testing.T) {
	src := storage.ManagedServer{
		InterfaceName: "Wireguard7",
		Address:       "10.7.0.1",
		Mask:          "255.255.255.0",
		ListenPort:    51827,
		PrivateKey:    "PRIV7=",
		Policy:        "none",
		I1:            "I1",
		I2:            "I2",
		I3:            "I3",
		I4:            "I4",
		I5:            "I5",
	}
	dto := managedServerToBackupDTO(src)
	got := backupDTOToManagedServer(dto)
	if got.I1 != src.I1 || got.I2 != src.I2 || got.I3 != src.I3 || got.I4 != src.I4 || got.I5 != src.I5 {
		t.Fatalf("I-fields mismatch after roundtrip: got=%+v", got)
	}
}
