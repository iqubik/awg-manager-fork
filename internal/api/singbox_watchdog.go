package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/hoaxisr/awg-manager/internal/logging"
	"github.com/hoaxisr/awg-manager/internal/response"
	"github.com/hoaxisr/awg-manager/internal/singboxwatchdog"
)

type SingboxWatchdogService interface {
	GetStatus(ctx context.Context) []singboxwatchdog.TargetStatus
	GetLogs(targetID string) []singboxwatchdog.LogEntry
	ClearLogs()
	Configure(cfg singboxwatchdog.TargetConfig) error
	Enable(id string) error
	Disable(id string) error
	CheckNow(ctx context.Context, targetID string)
	CheckAllNow(ctx context.Context)
}

type WatchdogIDRequest struct {
	ID string `json:"id" example:"tunnel:sb-main"`
}

type WatchdogCheckRequest struct {
	ID string `json:"id,omitempty" example:"subscription:sub-demo"`
}

type WatchdogConfigDTO struct {
	ID            string `json:"id" example:"tunnel:sb-main"`
	Kind          string `json:"kind" enums:"tunnel,subscription" example:"tunnel"`
	Ref           string `json:"ref" example:"sb-main"`
	Enabled       bool   `json:"enabled" example:"true"`
	Interval      int    `json:"interval" minimum:"5" maximum:"3600" example:"30"`
	FailThreshold int    `json:"failThreshold" minimum:"1" maximum:"20" example:"3"`
	Timeout       int    `json:"timeout" minimum:"1" maximum:"30" example:"5"`
	RecoveryMode  string `json:"recoveryMode" enums:"off,restart-singbox,switch-member" example:"off"`
	PersistSwitch bool   `json:"persistSwitch,omitempty" example:"false"`
}

type SingboxWatchdogStatusDTO struct {
	ID              string `json:"id" example:"tunnel:sb-main"`
	Kind            string `json:"kind" enums:"tunnel,subscription" example:"tunnel"`
	Ref             string `json:"ref" example:"sb-main"`
	Name            string `json:"name" example:"sb-main"`
	CheckTag        string `json:"checkTag" example:"sb-main"`
	TrafficTag      string `json:"trafficTag" example:"sb-main"`
	SelectorTag     string `json:"selectorTag,omitempty" example:"iq0"`
	ActiveMemberTag string `json:"activeMemberTag,omitempty" example:"member-1"`
	Protocol        string `json:"protocol,omitempty" example:"vless"`
	Security        string `json:"security,omitempty" example:"reality"`
	Transport       string `json:"transport,omitempty" example:"tcp"`
	ProxyInterface  string `json:"proxyInterface,omitempty" example:"Proxy5"`
	KernelInterface string `json:"kernelInterface,omitempty" example:"t2s5"`
	Running         bool   `json:"running" example:"true"`
	Configured      bool   `json:"configured" example:"true"`
	Enabled         bool   `json:"enabled" example:"true"`
	Status          string `json:"status" enums:"alive,warming,recovering,dead,disabled,stopped" example:"alive"`
	Interval        int    `json:"interval" example:"30"`
	Timeout         int    `json:"timeout" example:"5"`
	LastCheck       string `json:"lastCheck,omitempty" example:"2026-06-29T08:00:00Z"`
	LastLatency     int    `json:"lastLatency" example:"94"`
	FailCount       int    `json:"failCount" example:"0"`
	FailThreshold   int    `json:"failThreshold" example:"3"`
	RestartCount    int    `json:"restartCount" example:"0"`
	SwitchCount     int    `json:"switchCount" example:"1"`
	LastError       string `json:"lastError,omitempty" example:"timeout"`
	LastRecovery    string `json:"lastRecovery,omitempty" example:"member_switch"`
	RecoveryMode    string `json:"recoveryMode" enums:"off,restart-singbox,switch-member" example:"switch-member"`
	PersistSwitch   bool   `json:"persistSwitch,omitempty" example:"false"`
}

type SingboxWatchdogLogEntryDTO struct {
	Timestamp   string `json:"timestamp" example:"2026-06-29T08:00:00Z"`
	TargetID    string `json:"targetId" example:"subscription:sub-demo"`
	TargetName  string `json:"targetName" example:"Provider Demo"`
	Kind        string `json:"kind" enums:"tunnel,subscription" example:"subscription"`
	CheckTag    string `json:"checkTag" example:"iq0"`
	Success     bool   `json:"success" example:"true"`
	Latency     int    `json:"latency" example:"94"`
	Error       string `json:"error,omitempty" example:"timeout"`
	FailCount   int    `json:"failCount" example:"0"`
	Threshold   int    `json:"threshold" example:"3"`
	StateChange string `json:"stateChange,omitempty" example:"member_switch"`
	MemberFrom  string `json:"memberFrom,omitempty" example:"member-old"`
	MemberTo    string `json:"memberTo,omitempty" example:"member-new"`
}

type SingboxWatchdogStatusEnvelope struct {
	Success bool                       `json:"success" example:"true"`
	Data    []SingboxWatchdogStatusDTO `json:"data"`
}

type SingboxWatchdogLogsEnvelope struct {
	Success bool                         `json:"success" example:"true"`
	Data    []SingboxWatchdogLogEntryDTO `json:"data"`
}

type WatchdogActionEnvelopeData struct {
	Success bool `json:"success" example:"true"`
}

type WatchdogActionEnvelope struct {
	Success bool                       `json:"success" example:"true"`
	Data    WatchdogActionEnvelopeData `json:"data"`
}

type WatchdogMessageEnvelopeData struct {
	Message string `json:"message" example:"Logs cleared"`
}

type WatchdogMessageEnvelope struct {
	Success bool                        `json:"success" example:"true"`
	Data    WatchdogMessageEnvelopeData `json:"data"`
}

type SingboxWatchdogHandler struct {
	service SingboxWatchdogService
	log     *logging.ScopedLogger
}

func NewSingboxWatchdogHandler(service SingboxWatchdogService, appLogger logging.AppLogger) *SingboxWatchdogHandler {
	return &SingboxWatchdogHandler{
		service: service,
		log:     logging.NewScopedLogger(appLogger, logging.GroupSingbox, logging.SubSBWatchdog),
	}
}

func validateTargetConfig(c singboxwatchdog.TargetConfig) error {
	if c.ID == "" {
		return errors.New("id required")
	}
	if c.Kind != singboxwatchdog.TargetTunnel && c.Kind != singboxwatchdog.TargetSubscription {
		return errors.New("invalid kind")
	}
	if c.Ref == "" {
		return errors.New("ref required")
	}
	if c.Interval < 5 || c.Interval > 3600 {
		return errors.New("interval out of range")
	}
	if c.FailThreshold < 1 || c.FailThreshold > 20 {
		return errors.New("failThreshold out of range")
	}
	if c.Timeout < 1 || c.Timeout > 30 {
		return errors.New("timeout out of range")
	}
	return nil
}

// GetStatus returns Sing-box watchdog status.
//
//	@Summary		Sing-box watchdog status
//	@Tags			singbox
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	SingboxWatchdogStatusEnvelope
//	@Failure		405	{object}	APIErrorEnvelope
//	@Router			/singbox/watchdog/status [get]
func (h *SingboxWatchdogHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	response.Success(w, h.service.GetStatus(r.Context()))
}

// GetLogs returns Sing-box watchdog logs.
//
//	@Summary		Sing-box watchdog logs
//	@Tags			singbox
//	@Produce		json
//	@Security		CookieAuth
//	@Param			targetId	query	string	false	"Filter by target id"
//	@Success		200	{object}	SingboxWatchdogLogsEnvelope
//	@Failure		405	{object}	APIErrorEnvelope
//	@Router			/singbox/watchdog/logs [get]
func (h *SingboxWatchdogHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	response.Success(w, h.service.GetLogs(r.URL.Query().Get("targetId")))
}

// ClearLogs clears Sing-box watchdog logs.
//
//	@Summary		Clear Sing-box watchdog logs
//	@Tags			singbox
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	WatchdogMessageEnvelope
//	@Failure		405	{object}	APIErrorEnvelope
//	@Router			/singbox/watchdog/logs/clear [post]
func (h *SingboxWatchdogHandler) ClearLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	h.service.ClearLogs()
	response.Success(w, map[string]string{"message": "Logs cleared"})
}

// Configure configures one Sing-box watchdog target.
//
//	@Summary		Configure Sing-box watchdog target
//	@Tags			singbox
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		WatchdogConfigDTO	true	"Watchdog target config"
//	@Success		200		{object}	WatchdogActionEnvelope
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/watchdog/configure [post]
func (h *SingboxWatchdogHandler) Configure(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var dto WatchdogConfigDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		response.BadRequest(w, "invalid request")
		return
	}
	cfg := singboxwatchdog.TargetConfig{
		ID:            dto.ID,
		Kind:          singboxwatchdog.TargetKind(dto.Kind),
		Ref:           dto.Ref,
		Enabled:       dto.Enabled,
		Interval:      dto.Interval,
		FailThreshold: dto.FailThreshold,
		Timeout:       dto.Timeout,
		RecoveryMode:  singboxwatchdog.RecoveryMode(dto.RecoveryMode),
		PersistSwitch: dto.PersistSwitch,
	}
	if err := validateTargetConfig(cfg); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.service.Configure(cfg); err != nil {
		if errors.Is(err, singboxwatchdog.ErrTargetNotFound) || errors.Is(err, singboxwatchdog.ErrTargetMismatch) {
			response.BadRequest(w, err.Error())
			return
		}
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, map[string]bool{"success": true})
}

func decodeIDRequest(w http.ResponseWriter, r *http.Request) (WatchdogIDRequest, bool) {
	var req WatchdogIDRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			response.BadRequest(w, "id required")
			return req, false
		}
		response.BadRequest(w, "invalid request")
		return req, false
	}
	if req.ID == "" {
		response.BadRequest(w, "id required")
		return req, false
	}
	return req, true
}

// Enable enables a configured Sing-box watchdog target.
//
//	@Summary		Enable Sing-box watchdog target
//	@Tags			singbox
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		WatchdogIDRequest	true	"Target id"
//	@Success		200		{object}	WatchdogActionEnvelope
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/watchdog/enable [post]
func (h *SingboxWatchdogHandler) Enable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	req, ok := decodeIDRequest(w, r)
	if !ok {
		return
	}
	if err := h.service.Enable(req.ID); err != nil {
		if errors.Is(err, singboxwatchdog.ErrTargetNotFound) {
			response.BadRequest(w, err.Error())
			return
		}
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, map[string]bool{"success": true})
}

// Disable disables a configured Sing-box watchdog target.
//
//	@Summary		Disable Sing-box watchdog target
//	@Tags			singbox
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		WatchdogIDRequest	true	"Target id"
//	@Success		200		{object}	WatchdogActionEnvelope
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/watchdog/disable [post]
func (h *SingboxWatchdogHandler) Disable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	req, ok := decodeIDRequest(w, r)
	if !ok {
		return
	}
	if err := h.service.Disable(req.ID); err != nil {
		if errors.Is(err, singboxwatchdog.ErrTargetNotFound) {
			response.BadRequest(w, err.Error())
			return
		}
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, map[string]bool{"success": true})
}

// CheckNow triggers an immediate Sing-box watchdog check.
//
//	@Summary		Trigger Sing-box watchdog check
//	@Tags			singbox
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		WatchdogCheckRequest	false	"Optional target id"
//	@Success		200		{object}	WatchdogActionEnvelope
//	@Failure		400		{object}	APIErrorEnvelope
//	@Router			/singbox/watchdog/check-now [post]
func (h *SingboxWatchdogHandler) CheckNow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var req WatchdogCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		response.BadRequest(w, "invalid request")
		return
	}
	if req.ID != "" {
		h.service.CheckNow(r.Context(), req.ID)
	} else {
		h.service.CheckAllNow(r.Context())
	}
	response.Success(w, map[string]bool{"success": true})
}
