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
	Group     string `json:"group" example:"singbox"`
	Subgroup  string `json:"subgroup" example:"dns"`
	Action    string `json:"action" example:"run"`
	Target    string `json:"target" example:"dns"`
	Message   string `json:"message" example:"lookup succeed for node.example.org: 203.0.113.77"`
	Sanitized bool   `json:"sanitized" example:"false"`
}

// LogsData mirrors frontend LogsResponse.
type LogsData struct {
	Enabled         bool          `json:"enabled" example:"true"`
	Logs            []LogEntryDTO `json:"logs"`
	Total           int           `json:"total" example:"42"`
	Bucket          string        `json:"bucket" example:"singbox"`
	BufferSize      int           `json:"bufferSize" example:"123"`
	BufferCapacity  int           `json:"bufferCapacity" example:"5000"`
	Sanitized       bool          `json:"sanitized" example:"false"`
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
	Enabled         bool          `json:"enabled"`
	Logs            []LogEntryDTO `json:"logs"`
	Total           int           `json:"total"`
	Bucket          string        `json:"bucket"`
	BufferSize      int           `json:"bufferSize"`
	BufferCapacity  int           `json:"bufferCapacity"`
	Sanitized       bool          `json:"sanitized"`
	OldestTimestamp string        `json:"oldestTimestamp,omitempty"`
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

func parseBoolQuery(raw string, def bool) (bool, bool) {
	v := strings.ToLower(strings.TrimSpace(raw))
	switch v {
	case "":
		return def, true
	case "1", "true", "t", "yes", "y", "on":
		return true, true
	case "0", "false", "f", "no", "n", "off":
		return false, true
	}
	return false, false
}

func logEntryDTO(entry logging.LogEntry, sanitized bool) LogEntryDTO {
	target := entry.Target
	message := entry.Message
	if sanitized {
		target = logging.SanitizeLogText(target)
		message = logging.SanitizeLogText(message)
	}

	return LogEntryDTO{
		Timestamp: entry.Timestamp.UTC().Format(time.RFC3339Nano),
		Level:     entry.Level,
		Group:     entry.Group,
		Subgroup:  entry.Subgroup,
		Action:    entry.Action,
		Target:    target,
		Message:   message,
		Sanitized: sanitized,
	}
}

func logEntryDTOs(entries []logging.LogEntry, sanitized bool) []LogEntryDTO {
	if len(entries) == 0 {
		return []LogEntryDTO{}
	}
	out := make([]LogEntryDTO, 0, len(entries))
	for _, entry := range entries {
		out = append(out, logEntryDTO(entry, sanitized))
	}
	return out
}

// queryList reads repeated query params and comma-separated values.
// For example: group=a&group=b and group=a,b.
func queryList(q map[string][]string, key string) []string {
	values := q[key]
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}

	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			v := strings.TrimSpace(part)
			if v == "" {
				continue
			}
			if _, ok := seen[v]; ok {
				continue
			}
			seen[v] = struct{}{}
			out = append(out, v)
		}
	}

	return out
}

// GetLogs returns log entries from the requested bucket with optional
// filtering and pagination.
//
// GET /api/logs?bucket=app|singbox&group=&subgroup=&level=&limit=&offset=&since=&sanitize=
//
//	@Summary		Get logs
//	@Description	Returns log entries from the selected bucket. `bucket=app` (default) covers tunnel/routing/server/system events; `bucket=singbox` covers sing-box forwarder events isolated from app history. By default target/message are backend-masked; pass `sanitize=false` only when the authenticated admin UI explicitly reveals raw logs.
//	@Tags			logs
//	@Produce		json
//	@Security		CookieAuth
//	@Param			bucket		query		string	false	"Bucket selector"		Enums(app, singbox)
//	@Param			group		query		[]string	false	"Filter by group (repeat param or comma-separated values)"	collectionFormat(multi)
//	@Param			subgroup	query		[]string	false	"Filter by subgroup (repeat param or comma-separated values)"	collectionFormat(multi)
//	@Param			level		query		string	false	"Filter by level"
//	@Param			limit		query		int		false	"Max entries to return (default 200)"
//	@Param			offset		query		int		false	"Skip first N matching entries"
//	@Param			since		query		int		false	"Unix seconds — return only entries strictly after this"
//	@Param			sanitize	query		bool	false	"Return backend-sanitized target/message values; default true, set false to reveal raw values"
//	@Success		200	{object}	LogsResponseEnvelope
//	@Failure		400	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/logs [get]
func (h *LoggingHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.ErrorWithStatus(w, http.StatusMethodNotAllowed, "Method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	q := r.URL.Query()
	bucket, ok := parseBucket(q.Get("bucket"), logging.BucketApp)
	if !ok {
		response.ErrorWithStatus(w, http.StatusBadRequest,
			"invalid bucket: must be 'app' or 'singbox'", "INVALID_BUCKET")
		return
	}

	sanitize, ok := parseBoolQuery(q.Get("sanitize"), true)
	if !ok {
		response.ErrorWithStatus(w, http.StatusBadRequest,
			"invalid sanitize: must be true or false", "INVALID_SANITIZE")
		return
	}

	groups := queryList(q, "group")
	subgroups := queryList(q, "subgroup")
	level := q.Get("level")

	// Backward compat for old "category" param
	if cat := q.Get("category"); cat != "" && len(groups) == 0 {
		switch cat {
		case "tunnel":
			groups = []string{logging.GroupTunnel}
		case "settings":
			groups, subgroups = []string{logging.GroupSystem}, []string{logging.SubSettings}
		case "system":
			groups = []string{logging.GroupSystem}
		case "dns-route":
			groups, subgroups = []string{logging.GroupRouting}, []string{logging.SubDnsRoute}
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

	entries, total := h.svc.GetLogsMulti(bucket, groups, subgroups, level, since, limit, offset)

	stats := h.svc.Stats(bucket)
	resp := LogsResponse{
		Enabled:        h.svc.IsEnabled(),
		Logs:           logEntryDTOs(entries, sanitize),
		Total:          total,
		Bucket:         string(bucket),
		BufferSize:     stats.Size,
		BufferCapacity: stats.Capacity,
		Sanitized:      sanitize,
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
