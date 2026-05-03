package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hoaxisr/awg-manager/internal/events"
	"github.com/hoaxisr/awg-manager/internal/logging"
	"github.com/hoaxisr/awg-manager/internal/response"
)

// ── Response DTOs ────────────────────────────────────────────────

// LogEntryDTO mirrors frontend LogEntry.
type LogEntryDTO struct {
	Timestamp string `json:"timestamp" example:"2024-01-15T10:30:00Z"`
	Level     string `json:"level" example:"info"`
	Group     string `json:"group" example:"tunnel"`
	Subgroup  string `json:"subgroup" example:"tun_abc123"`
	Action    string `json:"action" example:"connect"`
	Target    string `json:"target" example:"vpn.example.com:51820"`
	Message   string `json:"message" example:"Tunnel connected"`
}

// LogsData mirrors frontend LogsResponse.
type LogsData struct {
	Enabled         bool          `json:"enabled" example:"true"`
	Logs            []LogEntryDTO `json:"logs"`
	Total           int           `json:"total" example:"42"`
	Bucket          string        `json:"bucket" example:"app"`
	BufferSize      int           `json:"bufferSize" example:"123"`
	BufferCapacity  int           `json:"bufferCapacity" example:"5000"`
	OldestTimestamp string        `json:"oldestTimestamp,omitempty" example:"2024-01-15T08:00:00Z"`
}

// LogsResponseEnvelope is the envelope for GET /logs.
type LogsResponseEnvelope struct {
	Success bool     `json:"success" example:"true"`
	Data    LogsData `json:"data"`
}

// SubgroupsData is the payload for GET /logs/subgroups.
type SubgroupsData struct {
	Group     string   `json:"group" example:"routing"`
	Subgroups []string `json:"subgroups"`
}

// SubgroupsResponseEnvelope is the envelope for GET /logs/subgroups.
type SubgroupsResponseEnvelope struct {
	Success bool          `json:"success" example:"true"`
	Data    SubgroupsData `json:"data"`
}

// LoggingHandler handles logging API endpoints.
type LoggingHandler struct {
	svc *logging.Service
	bus *events.Bus
	log *logging.ScopedLogger
}

// NewLoggingHandler creates a new logging handler.
func NewLoggingHandler(svc *logging.Service, appLogger logging.AppLogger) *LoggingHandler {
	return &LoggingHandler{
		svc: svc,
		log: logging.NewScopedLogger(appLogger, logging.GroupSystem, logging.SubSettings),
	}
}

// SetEventBus sets the event bus for SSE snapshot publishing.
func (h *LoggingHandler) SetEventBus(bus *events.Bus) { h.bus = bus }

// PublishSnapshot is a retained no-op hook — the legacy `snapshot:logs`
// SSE event was removed (state-sync redesign), and the frontend now fetches
// logs via REST. Keeping the method preserves the callback wiring in
// server.go / settings.go without forcing a broader refactor.
func (h *LoggingHandler) PublishSnapshot() {}

// LogsResponse represents the response for get logs endpoint.
type LogsResponse struct {
	Enabled         bool               `json:"enabled"`
	Logs            []logging.LogEntry `json:"logs"`
	Total           int                `json:"total"`
	Bucket          string             `json:"bucket"`
	BufferSize      int                `json:"bufferSize"`
	BufferCapacity  int                `json:"bufferCapacity"`
	OldestTimestamp string             `json:"oldestTimestamp,omitempty"`
}

func parseBucket(raw string, def logging.Bucket) (logging.Bucket, bool) {
	if raw == "" {
		return def, true
	}
	switch logging.Bucket(strings.ToLower(raw)) {
	case logging.BucketApp:
		return logging.BucketApp, true
	case logging.BucketSingbox:
		return logging.BucketSingbox, true
	}
	return "", false
}

// GetLogs returns log entries from the requested bucket with optional
// filtering and pagination.
//
// GET /api/logs?bucket=app|singbox&group=&subgroup=&level=&limit=&offset=&since=
//
//	@Summary		Get logs
//	@Description	Returns log entries from the selected bucket. `bucket=app` (default) covers tunnel/routing/server/system events; `bucket=singbox` covers sing-box forwarder events isolated from app history.
//	@Tags			logs
//	@Produce		json
//	@Security		CookieAuth
//	@Param			bucket		query		string	false	"Bucket selector"		Enums(app, singbox)
//	@Param			group		query		string	false	"Filter by group"
//	@Param			subgroup	query		string	false	"Filter by subgroup"
//	@Param			level		query		string	false	"Filter by level"
//	@Param			limit		query		int		false	"Max entries to return (default 200)"
//	@Param			offset		query		int		false	"Skip first N matching entries"
//	@Param			since		query		int		false	"Unix seconds — return only entries strictly after this"
//	@Success		200	{object}	LogsResponseEnvelope
//	@Failure		400	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/logs [get]
func (h *LoggingHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.ErrorWithStatus(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	bucket, ok := parseBucket(r.URL.Query().Get("bucket"), logging.BucketApp)
	if !ok {
		response.ErrorWithStatus(w, http.StatusBadRequest,
			"invalid bucket: must be 'app' or 'singbox'", "INVALID_BUCKET")
		return
	}

	group := r.URL.Query().Get("group")
	subgroup := r.URL.Query().Get("subgroup")
	level := r.URL.Query().Get("level")

	// Backward compat for old "category" param
	if cat := r.URL.Query().Get("category"); cat != "" && group == "" {
		switch cat {
		case "tunnel":
			group = logging.GroupTunnel
		case "settings":
			group, subgroup = logging.GroupSystem, logging.SubSettings
		case "system":
			group = logging.GroupSystem
		case "dns-route":
			group, subgroup = logging.GroupRouting, logging.SubDnsRoute
		}
	}

	limit := 200
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	// `since` (unix seconds) is used by the frontend on SSE reconnect to
	// catch up only the log entries that arrived while it was disconnected.
	var since time.Time
	if s := r.URL.Query().Get("since"); s != "" {
		if ts, err := strconv.ParseInt(s, 10, 64); err == nil && ts > 0 {
			since = time.Unix(ts, 0)
		}
	}

	logs, total := h.svc.GetLogs(bucket, group, subgroup, level, since, limit, offset)
	if logs == nil {
		logs = []logging.LogEntry{}
	}

	stats := h.svc.Stats(bucket)
	resp := LogsResponse{
		Enabled:        h.svc.IsEnabled(),
		Logs:           logs,
		Total:          total,
		Bucket:         string(bucket),
		BufferSize:     stats.Size,
		BufferCapacity: stats.Capacity,
	}
	if !stats.Oldest.IsZero() {
		resp.OldestTimestamp = stats.Oldest.UTC().Format(time.RFC3339)
	}
	response.Success(w, resp)
}

// ClearLogs removes all entries from the requested bucket.
//
// POST /api/logs/clear?bucket=app|singbox
//
//	@Summary		Clear logs
//	@Description	Clears all entries from the requested bucket. `bucket` is required — there is no implicit "clear everything" since app and sing-box logs serve different audiences.
//	@Tags			logs
//	@Produce		json
//	@Security		CookieAuth
//	@Param			bucket	query		string	true	"Bucket to clear"	Enums(app, singbox)
//	@Success		200	{object}	APIEnvelope
//	@Failure		400	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/logs/clear [post]
func (h *LoggingHandler) ClearLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.ErrorWithStatus(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	raw := r.URL.Query().Get("bucket")
	if raw == "" {
		response.ErrorWithStatus(w, http.StatusBadRequest,
			"bucket query parameter is required: 'app' or 'singbox'", "MISSING_BUCKET")
		return
	}
	bucket, ok := parseBucket(raw, logging.BucketApp)
	if !ok {
		response.ErrorWithStatus(w, http.StatusBadRequest,
			"invalid bucket: must be 'app' or 'singbox'", "INVALID_BUCKET")
		return
	}

	h.svc.Clear(bucket)
	h.log.Info("clear-logs", string(bucket), "Logs cleared")
	h.PublishSnapshot()
	response.Success(w, map[string]any{"cleared": true, "bucket": string(bucket)})
}

// GetSubgroups returns the static catalog of subgroups for the requested
// group. Used by the frontend to render a second-row chip filter.
//
// GET /api/logs/subgroups?group=routing
//
//	@Summary		List known subgroups for a group
//	@Description	Returns the static subgroup catalog from internal/logging.KnownSubgroups. Order is presentation-stable. Empty group returns 400.
//	@Tags			logs
//	@Produce		json
//	@Security		CookieAuth
//	@Param			group	query		string	true	"Group name"	Enums(tunnel, routing, server, system, singbox)
//	@Success		200	{object}	SubgroupsResponseEnvelope
//	@Failure		400	{object}	APIErrorEnvelope
//	@Router			/logs/subgroups [get]
func (h *LoggingHandler) GetSubgroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.ErrorWithStatus(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
		return
	}
	group := r.URL.Query().Get("group")
	if group == "" {
		response.ErrorWithStatus(w, http.StatusBadRequest,
			"group query parameter is required", "MISSING_GROUP")
		return
	}
	subs, ok := logging.KnownSubgroups[group]
	if !ok {
		// Unknown group — return empty list, not 404; lets the UI render
		// nothing without a noisy error toast.
		subs = []string{}
	}
	out := make([]string, len(subs))
	copy(out, subs)
	response.Success(w, SubgroupsData{Group: group, Subgroups: out})
}
