package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/logging"
)

type loggingTestSettings struct {
	enabled       bool
	logLevel      string
	appMax        int
	singboxMax    int
	loggingMaxAge int
}

func (s *loggingTestSettings) IsLoggingEnabled() bool { return s.enabled }
func (s *loggingTestSettings) GetLoggingMaxAge() int  { return s.loggingMaxAge }
func (s *loggingTestSettings) GetLogLevel() string {
	if s.logLevel == "" {
		return string(logging.LevelDebug)
	}
	return s.logLevel
}
func (s *loggingTestSettings) GetAppMaxEntries() int     { return s.appMax }
func (s *loggingTestSettings) GetSingboxMaxEntries() int { return s.singboxMax }

func TestQueryList(t *testing.T) {
	q := map[string][]string{
		"group": {" tunnel , routing ", "routing", "", "server,system"},
	}
	got := queryList(q, "group")
	want := []string{"tunnel", "routing", "server", "system"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q (full=%v)", i, got[i], want[i], got)
		}
	}
}

func TestGetLogs_MultiSelectSingboxSubgroups(t *testing.T) {
	settings := &loggingTestSettings{enabled: true, logLevel: string(logging.LevelDebug)}
	svc := logging.NewService(settings)
	defer svc.Stop()

	svc.AppLog(logging.LevelInfo, logging.GroupSingbox, logging.SubSBInbound, "a", "x", "in")
	svc.AppLog(logging.LevelInfo, logging.GroupSingbox, logging.SubSBDNS, "b", "x", "dns")
	svc.AppLog(logging.LevelInfo, logging.GroupSingbox, logging.SubSBRouter, "c", "x", "router")

	h := NewLoggingHandler(svc, svc)
	req := httptest.NewRequest(http.MethodGet, "/api/logs?bucket=singbox&group=singbox&subgroup=inbound&subgroup=dns", nil)
	w := httptest.NewRecorder()
	h.GetLogs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", w.Code, w.Body.String())
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Logs []logging.LogEntry `json:"logs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Success {
		t.Fatalf("success=false body=%s", w.Body.String())
	}
	if len(body.Data.Logs) != 2 {
		t.Fatalf("logs len = %d, want 2", len(body.Data.Logs))
	}
}

func TestGetLogs_MultiSelectAppGroups(t *testing.T) {
	settings := &loggingTestSettings{enabled: true, logLevel: string(logging.LevelDebug)}
	svc := logging.NewService(settings)
	defer svc.Stop()

	svc.AppLog(logging.LevelInfo, logging.GroupTunnel, logging.SubLifecycle, "a", "x", "tunnel")
	svc.AppLog(logging.LevelInfo, logging.GroupRouting, logging.SubDnsRoute, "b", "x", "routing")
	svc.AppLog(logging.LevelInfo, logging.GroupSystem, logging.SubSettings, "c", "x", "system")

	h := NewLoggingHandler(svc, svc)
	req := httptest.NewRequest(http.MethodGet, "/api/logs?bucket=app&group=tunnel&group=routing", nil)
	w := httptest.NewRecorder()
	h.GetLogs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", w.Code, w.Body.String())
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Total int                `json:"total"`
			Logs  []logging.LogEntry `json:"logs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Total != 2 {
		t.Fatalf("total = %d, want 2", body.Data.Total)
	}
	if len(body.Data.Logs) != 2 {
		t.Fatalf("logs len = %d, want 2", len(body.Data.Logs))
	}
}

func TestGetLogs_CategoryBackwardCompat(t *testing.T) {
	settings := &loggingTestSettings{enabled: true, logLevel: string(logging.LevelDebug)}
	svc := logging.NewService(settings)
	defer svc.Stop()

	svc.AppLog(logging.LevelInfo, logging.GroupSystem, logging.SubSettings, "save", "cfg", "settings")
	svc.AppLog(logging.LevelInfo, logging.GroupTunnel, logging.SubLifecycle, "up", "awg0", "tunnel")

	h := NewLoggingHandler(svc, svc)
	req := httptest.NewRequest(http.MethodGet, "/api/logs?bucket=app&category=settings", nil)
	w := httptest.NewRecorder()
	h.GetLogs(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 body=%s", w.Code, w.Body.String())
	}

	var body struct {
		Data struct {
			Total int                `json:"total"`
			Logs  []logging.LogEntry `json:"logs"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Data.Total != 1 || len(body.Data.Logs) != 1 {
		t.Fatalf("want 1 settings log, got total=%d len=%d", body.Data.Total, len(body.Data.Logs))
	}
	if body.Data.Logs[0].Subgroup != logging.SubSettings {
		t.Fatalf("subgroup = %q, want %q", body.Data.Logs[0].Subgroup, logging.SubSettings)
	}
}

func TestGetLogs_SanitizeQuery(t *testing.T) {
	settings := &loggingTestSettings{enabled: true, logLevel: string(logging.LevelDebug)}
	svc := logging.NewService(settings)
	defer svc.Stop()

	svc.AppLog(logging.LevelInfo, logging.GroupSingbox, logging.SubSBDNS, "run", "dns", "lookup succeed for node.example.org: 203.0.113.77")

	h := NewLoggingHandler(svc, svc)

	t.Run("sanitized by default", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/logs?bucket=singbox", nil)
		w := httptest.NewRecorder()
		h.GetLogs(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 body=%s", w.Code, w.Body.String())
		}

		var body struct {
			Data struct {
				Sanitized bool          `json:"sanitized"`
				Logs      []LogEntryDTO `json:"logs"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !body.Data.Sanitized || !body.Data.Logs[0].Sanitized {
			t.Fatalf("sanitized response not marked sanitized: %+v", body.Data)
		}
		if strings.Contains(body.Data.Logs[0].Message, "node.example.org") || strings.Contains(body.Data.Logs[0].Message, "203.0.113.77") {
			t.Fatalf("raw values leaked in default response: %q", body.Data.Logs[0].Message)
		}
	})

	t.Run("raw on request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/logs?bucket=singbox&sanitize=false", nil)
		w := httptest.NewRecorder()
		h.GetLogs(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 body=%s", w.Code, w.Body.String())
		}

		var body struct {
			Data struct {
				Sanitized bool          `json:"sanitized"`
				Logs      []LogEntryDTO `json:"logs"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if body.Data.Sanitized || body.Data.Logs[0].Sanitized {
			t.Fatalf("raw response marked sanitized: %+v", body.Data)
		}
		if !strings.Contains(body.Data.Logs[0].Message, "node.example.org") || !strings.Contains(body.Data.Logs[0].Message, "203.0.113.77") {
			t.Fatalf("raw values missing: %q", body.Data.Logs[0].Message)
		}
	})

	t.Run("sanitized on request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/logs?bucket=singbox&sanitize=true", nil)
		w := httptest.NewRecorder()
		h.GetLogs(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 body=%s", w.Code, w.Body.String())
		}

		var body struct {
			Data struct {
				Sanitized bool          `json:"sanitized"`
				Logs      []LogEntryDTO `json:"logs"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !body.Data.Sanitized || !body.Data.Logs[0].Sanitized {
			t.Fatalf("sanitized response not marked sanitized: %+v", body.Data)
		}
		if strings.Contains(body.Data.Logs[0].Message, "node.example.org") || strings.Contains(body.Data.Logs[0].Message, "203.0.113.77") {
			t.Fatalf("raw values leaked in sanitized response: %q", body.Data.Logs[0].Message)
		}
	})

	t.Run("sanitized on request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/logs?bucket=singbox&sanitize=true", nil)
		w := httptest.NewRecorder()
		h.GetLogs(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 body=%s", w.Code, w.Body.String())
		}

		var body struct {
			Data struct {
				Sanitized bool          `json:"sanitized"`
				Logs      []LogEntryDTO `json:"logs"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !body.Data.Sanitized || !body.Data.Logs[0].Sanitized {
			t.Fatalf("sanitized response not marked sanitized: %+v", body.Data)
		}
		if strings.Contains(body.Data.Logs[0].Message, "node.example.org") || strings.Contains(body.Data.Logs[0].Message, "203.0.113.77") {
			t.Fatalf("raw values leaked in sanitized response: %q", body.Data.Logs[0].Message)
		}
	})
}
