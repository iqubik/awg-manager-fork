package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hoaxisr/awg-manager/internal/managed"
	"github.com/hoaxisr/awg-manager/internal/response"
)

// ManagedServerBackupHandler exposes Export / Import / Drift / RestoreDrift.
type ManagedServerBackupHandler struct {
	svc *managed.Service
}

// NewManagedServerBackupHandler creates a new backup handler.
func NewManagedServerBackupHandler(svc *managed.Service) *ManagedServerBackupHandler {
	return &ManagedServerBackupHandler{svc: svc}
}

// ManagedServerBackupFile is the on-disk JSON shape.
type ManagedServerBackupFile struct {
	Version        int                           `json:"version"`
	Type           string                        `json:"type"`
	ExportedAt     time.Time                     `json:"exportedAt"`
	ManagedServers []managed.ManagedServerExport `json:"managedServers"`
}

const (
	backupFileType    = "awg-manager-managed-server-backup"
	backupFileVersion = 1
)

// ManagedServerImportRequest is the body of POST /api/managed/import.
type ManagedServerImportRequest struct {
	ManagedServers []managed.ManagedServerExport `json:"managedServers"`
	Options        managed.RestoreOptions        `json:"options"`
	Version        int                           `json:"version,omitempty"`
	Type           string                        `json:"type,omitempty"`
}

// ManagedServerRestoreDriftRequest is the body of POST /api/managed/restore-drift.
type ManagedServerRestoreDriftRequest struct {
	Options managed.RestoreOptions `json:"options"`
}

// ManagedServerRestoreResponse is the response of /import and /restore-drift.
type ManagedServerRestoreResponse struct {
	Outcomes []managed.RestoreOutcome `json:"outcomes"`
}

// ManagedServerDriftResponse is the response of GET /api/managed/drift.
type ManagedServerDriftResponse struct {
	Drift []managed.ManagedServerExport `json:"drift"`
}

// Export handles GET /api/managed/export.
//
//	@Summary		Export all managed servers
//	@Description	Returns a JSON backup file with every managed server including private keys.
//	@Tags			managed
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	ManagedServerBackupFile
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/managed/export [get]
func (h *ManagedServerBackupHandler) Export(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	servers, err := h.svc.ExportAll(r.Context())
	if err != nil {
		response.InternalError(w, "export: "+err.Error())
		return
	}
	now := time.Now().UTC()
	body := ManagedServerBackupFile{
		Version:        backupFileVersion,
		Type:           backupFileType,
		ExportedAt:     now,
		ManagedServers: servers,
	}
	filename := fmt.Sprintf("managed-backup-%s.json", now.Format("2006-01-02"))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

// Import handles POST /api/managed/import.
//
//	@Summary		Import a managed-server backup
//	@Description	Restores managed servers from a backup file. Per-server atomic with pre-flight conflict detection.
//	@Tags			managed
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		ManagedServerImportRequest	true	"backup contents + options"
//	@Success		200		{object}	ManagedServerRestoreResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/managed/import [post]
func (h *ManagedServerBackupHandler) Import(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var req ManagedServerImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "invalid request: "+err.Error(), "INVALID_REQUEST")
		return
	}
	if req.Type != "" && req.Type != backupFileType {
		response.Error(w, "unknown file type: "+req.Type, "INVALID_REQUEST")
		return
	}
	if req.Version != 0 && req.Version != backupFileVersion {
		response.Error(w, fmt.Sprintf("unsupported version %d (only %d)", req.Version, backupFileVersion), "INVALID_REQUEST")
		return
	}
	outcomes := h.svc.Restore(r.Context(), req.ManagedServers, req.Options)
	response.Success(w, ManagedServerRestoreResponse{Outcomes: outcomes})
}

// Drift handles GET /api/managed/drift.
//
//	@Summary		List managed servers missing from NDMS
//	@Description	Returns settings.json entries whose NDMS interface is absent.
//	@Tags			managed
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	ManagedServerDriftResponse
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/managed/drift [get]
func (h *ManagedServerBackupHandler) Drift(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	drift, err := h.svc.Drift(r.Context())
	if err != nil {
		response.InternalError(w, "drift: "+err.Error())
		return
	}
	response.Success(w, ManagedServerDriftResponse{Drift: drift})
}

// RestoreDrift handles POST /api/managed/restore-drift.
//
//	@Summary		Restore drifted managed servers
//	@Description	Detects drift internally then runs Restore on it. Convenience entry for the boot-time banner.
//	@Tags			managed
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		ManagedServerRestoreDriftRequest	false	"options"
//	@Success		200		{object}	ManagedServerRestoreResponse
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/managed/restore-drift [post]
func (h *ManagedServerBackupHandler) RestoreDrift(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var req ManagedServerRestoreDriftRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
		response.Error(w, "invalid request: "+err.Error(), "INVALID_REQUEST")
		return
	}
	drift, err := h.svc.Drift(r.Context())
	if err != nil {
		response.InternalError(w, "drift: "+err.Error())
		return
	}
	outcomes := h.svc.Restore(r.Context(), drift, req.Options)
	response.Success(w, ManagedServerRestoreResponse{Outcomes: outcomes})
}
