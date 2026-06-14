package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/monitoring"
)

func newMonitoringTestService() *monitoring.Service {
	return monitoring.NewService(monitoring.SchedulerDeps{})
}

func appendMonitoringSamples(svc *monitoring.Service, targetID, tunnelID string, count int) {
	hist := svc.Scheduler().History()
	for i := 0; i < count; i++ {
		lat := i + 1
		hist.Append(targetID, tunnelID, monitoring.Sample{LatencyMs: &lat, OK: true})
	}
}

func decodeMonitoringSamples(t *testing.T, rr *httptest.ResponseRecorder) []monitoring.Sample {
	t.Helper()
	var resp struct {
		Success bool                `json:"success"`
		Data    []monitoring.Sample `json:"data"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.Data
}

func TestMonitoringHandler_GetHistory_DefaultLimitUses24Hours(t *testing.T) {
	svc := newMonitoringTestService()
	appendMonitoringSamples(svc, "t", "tn", monitoring.MonitoringHistoryCapacity+25)
	h := NewMonitoringHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/monitoring/history?target=t&tunnelId=tn", nil)
	rr := httptest.NewRecorder()
	h.GetHistory(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	data := decodeMonitoringSamples(t, rr)
	if len(data) != monitoring.MonitoringHistoryCapacity {
		t.Fatalf("len = %d, want %d", len(data), monitoring.MonitoringHistoryCapacity)
	}
}

func TestMonitoringHandler_GetHistory_ClampsLargeLimit(t *testing.T) {
	svc := newMonitoringTestService()
	appendMonitoringSamples(svc, "t", "tn", monitoring.MonitoringHistoryCapacity)
	h := NewMonitoringHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/monitoring/history?target=t&tunnelId=tn&limit=99999", nil)
	rr := httptest.NewRecorder()
	h.GetHistory(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	data := decodeMonitoringSamples(t, rr)
	if len(data) != monitoring.MonitoringHistoryCapacity {
		t.Fatalf("len = %d, want %d", len(data), monitoring.MonitoringHistoryCapacity)
	}
}

func TestMonitoringHandler_GetHistory_UsesExplicitLimit(t *testing.T) {
	svc := newMonitoringTestService()
	appendMonitoringSamples(svc, "t", "tn", 100)
	h := NewMonitoringHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/monitoring/history?target=t&tunnelId=tn&limit=10", nil)
	rr := httptest.NewRecorder()
	h.GetHistory(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	data := decodeMonitoringSamples(t, rr)
	if len(data) != 10 {
		t.Fatalf("len = %d, want 10", len(data))
	}
}
