package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
