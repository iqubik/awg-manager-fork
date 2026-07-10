package api

import (
	"io"
	"net/http"
	"strings"

	"github.com/hoaxisr/awg-manager/internal/events"
	"github.com/hoaxisr/awg-manager/internal/integrationbackup"
	"github.com/hoaxisr/awg-manager/internal/response"
)

// IntegrationRestoreOutcomeDTO описывает действие restore для одного файла.
type IntegrationRestoreOutcomeDTO struct {
	Path   string `json:"path" example:"/opt/etc/awg-manager/singbox/config.d/20-router.json"`
	Action string `json:"action" example:"restored" enums:"planned,planned_delete,restored,deleted,skipped,conflict,rollback"`
	Error  string `json:"error,omitempty" example:"checksum не совпал"`
}

// IntegrationRestoreResponseDTO — полезная нагрузка restore/dry-run ответа.
type IntegrationRestoreResponseDTO struct {
	Component string                         `json:"component" example:"singbox"`
	DryRun    bool                           `json:"dryRun" example:"true"`
	Outcomes  []IntegrationRestoreOutcomeDTO `json:"outcomes"`
	Warnings  []string                       `json:"warnings,omitempty" example:"rollback выполнен после ошибки restore"`
}

// IntegrationRestoreEnvelope — swagger-friendly success envelope.
type IntegrationRestoreEnvelope struct {
	Success bool                          `json:"success" example:"true"`
	Data    IntegrationRestoreResponseDTO `json:"data"`
}

type IntegrationBackupHandler struct {
	svc *integrationbackup.Service
	bus *events.Bus
}

func NewIntegrationBackupHandler(svc *integrationbackup.Service) *IntegrationBackupHandler {
	return &IntegrationBackupHandler{svc: svc}
}

func (h *IntegrationBackupHandler) SetEventBus(bus *events.Bus) { h.bus = bus }

// SingboxBackup handles GET /api/singbox/backup.
//
//	@Summary		Скачать backup sing-box
//	@Description	Возвращает ZIP-архив с allowlist-файлами sing-box и безопасным snapshot настроек.
//	@Tags			singbox
//	@Produce		application/zip
//	@Security		CookieAuth
//	@Success		200	{file}		binary
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/singbox/backup [get]
func (h *IntegrationBackupHandler) SingboxBackup(w http.ResponseWriter, r *http.Request) {
	h.backup(integrationbackup.ComponentSingbox, w, r)
}

// SingboxRestore handles POST /api/singbox/restore.
//
//	@Summary		Восстановить backup sing-box
//	@Description	При dryRun=true выполняет только проверку ZIP-архива и показывает план действий без записи на диск.
//	@Tags			singbox
//	@Accept			application/zip
//	@Produce		json
//	@Security		CookieAuth
//	@Param			dryRun	query		boolean						false	"Только проверить архив без записи на диск"
//	@Param			body	body		string						true	"ZIP-архив backup"
//	@Success		200		{object}	IntegrationRestoreEnvelope
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/restore [post]
func (h *IntegrationBackupHandler) SingboxRestore(w http.ResponseWriter, r *http.Request) {
	h.restore(integrationbackup.ComponentSingbox, w, r)
}

// HydraRouteBackup handles GET /api/hydraroute/backup.
//
//	@Summary		Скачать backup HydraRoute
//	@Description	Возвращает ZIP-архив с allowlist-файлами HydraRoute и безопасным snapshot настроек.
//	@Tags			hydraroute
//	@Produce		application/zip
//	@Security		CookieAuth
//	@Success		200	{file}		binary
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/hydraroute/backup [get]
func (h *IntegrationBackupHandler) HydraRouteBackup(w http.ResponseWriter, r *http.Request) {
	h.backup(integrationbackup.ComponentHydraRoute, w, r)
}

// HydraRouteRestore handles POST /api/hydraroute/restore.
//
//	@Summary		Восстановить backup HydraRoute
//	@Description	При dryRun=true выполняет только проверку ZIP-архива и показывает план действий без записи на диск.
//	@Tags			hydraroute
//	@Accept			application/zip
//	@Produce		json
//	@Security		CookieAuth
//	@Param			dryRun	query		boolean						false	"Только проверить архив без записи на диск"
//	@Param			body	body		string						true	"ZIP-архив backup"
//	@Success		200		{object}	IntegrationRestoreEnvelope
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/hydraroute/restore [post]
func (h *IntegrationBackupHandler) HydraRouteRestore(w http.ResponseWriter, r *http.Request) {
	h.restore(integrationbackup.ComponentHydraRoute, w, r)
}

func (h *IntegrationBackupHandler) backup(component string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	filename, payload, err := h.svc.CreateBackup(r.Context(), component)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func (h *IntegrationBackupHandler) restore(component string, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 128<<20)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		response.BadRequest(w, "не удалось прочитать тело ZIP-архива")
		return
	}
	dryRun := isTruthy(r.URL.Query().Get("dryRun"))
	out, err := h.svc.Restore(r.Context(), component, payload, dryRun)
	if err != nil {
		response.Error(w, err.Error(), "INTEGRATION_RESTORE_FAILED")
		return
	}
	if !dryRun {
		switch component {
		case integrationbackup.ComponentSingbox:
			publishInvalidated(h.bus, ResourceSingboxStatus, "restore")
			publishInvalidated(h.bus, ResourceSingboxTunnels, "restore")
			publishInvalidated(h.bus, ResourceSettings, "restore")
		case integrationbackup.ComponentHydraRoute:
			publishInvalidated(h.bus, ResourceRoutingHydrarouteStatus, "restore")
			publishInvalidated(h.bus, ResourceRoutingDnsRoutes, "restore")
			publishInvalidated(h.bus, ResourceRoutingStaticRoutes, "restore")
			publishInvalidated(h.bus, ResourceRoutingClientRoutes, "restore")
			publishInvalidated(h.bus, ResourceSettings, "restore")
		}
	}
	response.Success(w, out)
}

func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
