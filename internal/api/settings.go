package api

import (
	"crypto/rand"
	"fmt"
	"net/http"

	"github.com/hoaxisr/awg-manager/internal/events"
	"github.com/hoaxisr/awg-manager/internal/logging"
	"github.com/hoaxisr/awg-manager/internal/response"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

// ── Response DTOs ────────────────────────────────────────────────

// ServerSettingsDTO mirrors frontend ServerSettings.
type ServerSettingsDTO struct {
	Port      int    `json:"port" example:"8080"`
	Interface string `json:"interface" example:""`
}

// PingCheckDefaultsDTO mirrors frontend PingCheckDefaults.
type PingCheckDefaultsDTO struct {
	Method        string `json:"method" example:"http"`
	Target        string `json:"target" example:"https://www.google.com"`
	Interval      int    `json:"interval" example:"30"`
	DeadInterval  int    `json:"deadInterval" example:"120"`
	FailThreshold int    `json:"failThreshold" example:"3"`
}

// PingCheckSettingsDTO mirrors frontend PingCheckSettings.
type PingCheckSettingsDTO struct {
	Enabled  bool                 `json:"enabled" example:"true"`
	Defaults PingCheckDefaultsDTO `json:"defaults"`
}

// LoggingSettingsDTO mirrors frontend LoggingSettings.
type LoggingSettingsDTO struct {
	Enabled           bool   `json:"enabled" example:"true"`
	MaxAge            int    `json:"maxAge" example:"2"`
	LogLevel          string `json:"logLevel" example:"info"`
	AppMaxEntries     int    `json:"appMaxEntries" example:"5000"`
	SingboxMaxEntries int    `json:"singboxMaxEntries" example:"5000"`
}

// UpdateSettingsDTO mirrors frontend UpdateSettings.
type UpdateSettingsDTO struct {
	CheckEnabled bool `json:"checkEnabled" example:"true"`
}

// DNSRouteSettingsDTO mirrors frontend DNSRouteSettings.
type DNSRouteSettingsDTO struct {
	AutoRefreshEnabled   bool   `json:"autoRefreshEnabled" example:"true"`
	RefreshIntervalHours int    `json:"refreshIntervalHours" example:"24"`
	RefreshMode          string `json:"refreshMode" example:"interval"`
	RefreshDailyTime     string `json:"refreshDailyTime" example:"03:00"`
}

// SettingsData is the payload for GET /settings/get.
type SettingsData struct {
	SchemaVersion       int                  `json:"schemaVersion" example:"16"`
	AuthEnabled         bool                 `json:"authEnabled" example:"false"`
	Server              ServerSettingsDTO    `json:"server"`
	PingCheck           PingCheckSettingsDTO `json:"pingCheck"`
	Logging             LoggingSettingsDTO   `json:"logging"`
	DisableMemorySaving bool                 `json:"disableMemorySaving" example:"false"`
	Updates             UpdateSettingsDTO    `json:"updates"`
	DnsRoute            DNSRouteSettingsDTO  `json:"dnsRoute"`
	// UsageLevel controls which UI sections are visible to the user.
	// Filtering is frontend-only — the API does not enforce it.
	// enums: expert,advanced,basic
	// First enum is the prism mock default (prism picks the first
	// enum value over the example tag); putting `expert` first means
	// dev:mock surfaces all advanced UI without manual toggling.
	UsageLevel string `json:"usageLevel" example:"expert" enums:"expert,advanced,basic"`
}

// SettingsResponse is the envelope for GET /settings/get.
type SettingsResponse struct {
	Success bool         `json:"success" example:"true"`
	Data    SettingsData `json:"data"`
}

// PingCheckToggleService defines the interface for ping check toggle operations.
type PingCheckToggleService interface {
	StartMonitoringAllRunning()
	StopMonitoringAll()
}

// SettingsHandler handles settings API endpoints.
type SettingsHandler struct {
	store              *storage.SettingsStore
	tunnels            *storage.AWGTunnelStore
	pingCheck          PingCheckToggleService
	pingCheckSnapshot  func()
	logsSnapshot       func()
	applyLogSettings   func()
	log                *logging.ScopedLogger
	bus                *events.Bus
}

// NewSettingsHandler creates a new settings handler.
func NewSettingsHandler(store *storage.SettingsStore, appLogger logging.AppLogger) *SettingsHandler {
	return &SettingsHandler{
		store: store,
		log:   logging.NewScopedLogger(appLogger, logging.GroupSystem, logging.SubSettings),
	}
}

// SetTunnelStore sets the tunnel store for ping check toggle logic.
func (h *SettingsHandler) SetTunnelStore(tunnels *storage.AWGTunnelStore) {
	h.tunnels = tunnels
}

// SetPingCheckService sets the ping check service for toggle operations.
func (h *SettingsHandler) SetPingCheckService(svc PingCheckToggleService) {
	h.pingCheck = svc
}

// SetPingCheckSnapshot sets the function that publishes a pingcheck snapshot.
func (h *SettingsHandler) SetPingCheckSnapshot(fn func()) { h.pingCheckSnapshot = fn }

// SetLogsSnapshot sets the function that publishes a logs snapshot.
func (h *SettingsHandler) SetLogsSnapshot(fn func()) { h.logsSnapshot = fn }

// SetApplyLoggingSettings sets the callback that re-applies logging
// settings to the live buffers (MaxAge, MaxEntries) after a successful
// settings update. Called once per Update with no arguments — the
// callback re-reads the settings store itself.
func (h *SettingsHandler) SetApplyLoggingSettings(fn func()) { h.applyLogSettings = fn }

// SetEventBus wires the SSE bus so settings mutations broadcast a
// resource:invalidated hint to all connected clients.
func (h *SettingsHandler) SetEventBus(bus *events.Bus) { h.bus = bus }

// Get returns current settings.
//
//	@Summary		Get settings
//	@Description	Returns the full Settings object (server, pingCheck, logging, dnsRoute, managed, apiKey, ...).
//	@Tags			settings
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	SettingsResponse
//	@Failure		405	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/settings/get [get]
func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.ErrorWithStatus(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	settings, err := h.store.Get()
	if err != nil {
		response.Error(w, err.Error(), "SETTINGS_LOAD_ERROR")
		return
	}

	response.Success(w, settings)
}

// Update saves settings.
//
//	@Summary		Update settings
//	@Description	Persists Settings via patch semantics: any field omitted from the payload is preserved, including top-level bool flags. Send only the fields you want to change, or send the full Settings object to update everything atomically. ApiKey preserved when omitted (rotate via /settings/regenerate-api-key).
//	@Tags			settings
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SettingsData	true	"Settings patch — any subset of fields"
//	@Success		200		{object}	SettingsResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/settings/update [post]
func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	patch, ok := parseJSON[storage.SettingsPatch](w, r, http.MethodPost)
	if !ok {
		return
	}

	oldSettings, err := h.store.Get()
	if err != nil {
		response.Error(w, err.Error(), "SETTINGS_LOAD_ERROR")
		return
	}

	// Apply patch onto a snapshot of the current settings: any field the
	// client did NOT send (nil pointer in patch) keeps its existing value.
	// This replaces the previous zero-value-restore defense, which could
	// not protect top-level bool flags (false vs absent were
	// indistinguishable in a non-pointer DTO).
	//
	// NOTE: sub-structs are replaced wholesale (no recursion). The frontend
	// always sends each sub-struct in full via spread, so omitting one
	// inner field is not a supported partial-update pattern. If a future
	// caller needs field-level granularity inside a sub-struct, add a
	// dedicated *Patch type for that sub-struct.
	//
	// Defense-in-depth: an explicit empty ApiKey ("") would WIPE the key,
	// stranding any Bearer-auth client. The intended rotation path is
	// /settings/regenerate-api-key. Treat explicit empty as "absent" so a
	// stale/buggy client cannot accidentally revoke its own key.
	if patch.ApiKey != nil && *patch.ApiKey == "" {
		patch.ApiKey = nil
	}
	merged := *oldSettings
	storage.ApplyPatch(&merged, &patch)

	// Validate usageLevel after merge. Empty merged.UsageLevel is
	// impossible because oldSettings always carries a value (default
	// settings populate it; migration v15 backfills it), so we only
	// reject explicit invalid values.
	if storage.NormalizeUsageLevel(merged.UsageLevel) != merged.UsageLevel {
		response.ErrorWithStatus(w, http.StatusBadRequest,
			"invalid usageLevel: must be one of basic, advanced, expert",
			"INVALID_USAGE_LEVEL")
		return
	}

	// Detect ping check toggle change before saving
	pingCheckWasEnabled := oldSettings.PingCheck.Enabled
	pingCheckNowEnabled := merged.PingCheck.Enabled
	toggleEnabled := !pingCheckWasEnabled && pingCheckNowEnabled
	toggleDisabled := pingCheckWasEnabled && !pingCheckNowEnabled

	// Detect logging toggle change
	loggingWasEnabled := oldSettings.Logging.Enabled
	loggingNowEnabled := merged.Logging.Enabled

	// Update tunnel configs if enabling
	if h.tunnels != nil && toggleEnabled {
		if err := h.enablePingCheckOnAllTunnels(&merged); err != nil {
			response.Error(w, err.Error(), "TOGGLE_ENABLE_ERROR")
			return
		}
	}

	// Save settings BEFORE starting monitoring (so service reads new values)
	if err := h.store.Save(&merged); err != nil {
		response.Error(w, err.Error(), "SETTINGS_SAVE_ERROR")
		return
	}

	// Apply logging changes (MaxAge / per-bucket MaxEntries) to live
	// buffers. Without this, the buffer keeps the previous cap until the
	// next AppLog tick and the cleanup ticker (up to 5 min later).
	if h.applyLogSettings != nil {
		h.applyLogSettings()
	}

	// Handle ping check toggle AFTER settings are saved
	if h.tunnels != nil {
		if toggleEnabled {
			if h.pingCheck != nil {
				h.pingCheck.StartMonitoringAllRunning()
			}
		} else if toggleDisabled {
			if h.pingCheck != nil {
				h.pingCheck.StopMonitoringAll()
			}
			if err := h.disablePingCheckOnAllTunnels(); err != nil {
				response.Error(w, err.Error(), "TOGGLE_DISABLE_ERROR")
				return
			}
		}
	}

	// Log specific changes
	if loggingNowEnabled && !loggingWasEnabled {
		h.log.Info("logging", "", "Logging enabled")
	} else if loggingWasEnabled && !loggingNowEnabled {
		h.log.Info("logging", "", "Logging disabled")
	}

	if toggleEnabled {
		h.log.Info("pingcheck", "", "Ping Check enabled")
	} else if toggleDisabled {
		h.log.Info("pingcheck", "", "Ping Check disabled")
	}

	if oldSettings.Server.Port != merged.Server.Port {
		h.log.Info("update", "", "Server port changed")
	}
	if oldSettings.AuthEnabled != merged.AuthEnabled {
		if merged.AuthEnabled {
			h.log.Info("auth", "", "Authentication enabled")
		} else {
			h.log.Warn("auth", "", "Authentication disabled")
		}
	}
	if oldSettings.DisableMemorySaving != merged.DisableMemorySaving {
		if merged.DisableMemorySaving {
			h.log.Info("memory-saving", "", "Memory saving disabled")
		} else {
			h.log.Info("memory-saving", "", "Memory saving enabled")
		}
	}

	if h.pingCheckSnapshot != nil && (toggleEnabled || toggleDisabled) {
		h.pingCheckSnapshot()
	}
	if h.logsSnapshot != nil && loggingNowEnabled != loggingWasEnabled {
		h.logsSnapshot()
	}

	response.Success(w, merged)
	publishInvalidated(h.bus, ResourceSettings, "updated")
}

// RegenerateApiKey generates a fresh UUID v4 server-side, persists it
// into Settings.ApiKey, and returns the updated Settings. Lives on the
// backend (not in browser via crypto.randomUUID) because the UI is
// served over plain HTTP and the WebCrypto API is unavailable in
// non-secure contexts.
//
//	@Summary		Regenerate API key
//	@Description	Generates a fresh UUID v4 via crypto/rand, stores it into Settings.ApiKey, and returns the updated Settings. The new key takes effect immediately as a `Authorization: Bearer <key>` substitute for the session cookie.
//	@Tags			settings
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	SettingsResponse
//	@Failure		405	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/settings/regenerate-api-key [post]
func (h *SettingsHandler) RegenerateApiKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.ErrorWithStatus(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	key, err := generateUUIDv4()
	if err != nil {
		response.Error(w, "failed to generate key: "+err.Error(), "API_KEY_GENERATE_ERROR")
		return
	}

	settings, err := h.store.Get()
	if err != nil {
		response.Error(w, err.Error(), "SETTINGS_LOAD_ERROR")
		return
	}
	settings.ApiKey = key
	if err := h.store.Save(settings); err != nil {
		response.Error(w, err.Error(), "SETTINGS_SAVE_ERROR")
		return
	}

	h.log.Info("api-key", "", "API key regenerated")
	response.Success(w, settings)
	publishInvalidated(h.bus, ResourceSettings, "api-key-rotated")
}

// generateUUIDv4 produces an RFC 4122 v4 UUID using crypto/rand.
// Format: 8-4-4-4-12 lowercase hex.
func generateUUIDv4() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant RFC 4122
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// enablePingCheckOnAllTunnels adds pingCheck config with defaults to all tunnels.
func (h *SettingsHandler) enablePingCheckOnAllTunnels(settings *storage.Settings) error {
	tunnels, err := h.tunnels.List()
	if err != nil {
		return err
	}

	defaults := settings.PingCheck.Defaults
	for i := range tunnels {
		tunnel := &tunnels[i]
		if tunnel.PingCheck == nil {
			tunnel.PingCheck = &storage.TunnelPingCheck{
				Enabled:       true,
				Method:        defaults.Method,
				Target:        defaults.Target,
				Interval:      defaults.Interval,
				DeadInterval:  defaults.DeadInterval,
				FailThreshold: defaults.FailThreshold,
				MinSuccess:    1,
				Timeout:       5,
				Restart:       true,
			}
		} else {
			tunnel.PingCheck.Enabled = true
		}
		if err := h.tunnels.Save(tunnel); err != nil {
			return err
		}
	}
	return nil
}

// disablePingCheckOnAllTunnels sets pingCheck.enabled=false on all tunnels.
func (h *SettingsHandler) disablePingCheckOnAllTunnels() error {
	tunnels, err := h.tunnels.List()
	if err != nil {
		return err
	}

	for i := range tunnels {
		tunnel := &tunnels[i]
		if tunnel.PingCheck != nil {
			tunnel.PingCheck.Enabled = false
			if err := h.tunnels.Save(tunnel); err != nil {
				return err
			}
		}
	}
	return nil
}
