package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/diagnostics"
)

type fakeDiagnosticsRunner struct {
	runErr    error
	resultErr error
	result    []byte
	status    diagnostics.RunStatus
}

func (f *fakeDiagnosticsRunner) Run(context.Context) error { return f.runErr }
func (f *fakeDiagnosticsRunner) RunWithStream(context.Context, diagnostics.RunOptions) (<-chan diagnostics.DiagEvent, error) {
	ch := make(chan diagnostics.DiagEvent)
	close(ch)
	return ch, nil
}
func (f *fakeDiagnosticsRunner) Status() diagnostics.RunStatus { return f.status }
func (f *fakeDiagnosticsRunner) Result() ([]byte, error) {
	if f.resultErr != nil {
		return nil, f.resultErr
	}
	return f.result, nil
}

func TestHealthHandlerServeHTTP(t *testing.T) {
	h := NewHealthHandler("2.5.0", "iid")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"ok":true`) {
		t.Fatalf("GET health failed: code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/health", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST health code=%d", rr.Code)
	}
}

func TestBootStatusHandlerGet(t *testing.T) {
	h := NewBootStatusHandler("iid")
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boot-status", nil)
	h.Get(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"instanceId":"iid"`) {
		t.Fatalf("GET boot-status failed: code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/boot-status", nil)
	h.Get(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST boot-status code=%d", rr.Code)
	}
}

func TestDiagnosticsHandlerSimpleContracts(t *testing.T) {
	r := &fakeDiagnosticsRunner{status: diagnostics.RunStatus{Status: "idle"}, result: []byte(`{"ok":true}`)}
	h := NewDiagnosticsHandler(r)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/diagnostics/run", nil)
	h.Run(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"status":"running"`) {
		t.Fatalf("Run success failed: code=%d body=%s", rr.Code, rr.Body.String())
	}

	r.runErr = errors.New("busy")
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/diagnostics/run", nil)
	h.Run(rr, req)
	if rr.Code != http.StatusConflict || !strings.Contains(rr.Body.String(), `DIAGNOSTICS_RUNNING`) {
		t.Fatalf("Run error failed: code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/diagnostics/status", nil)
	h.Status(rr, req)
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"status":"idle"`) {
		t.Fatalf("Status failed: code=%d body=%s", rr.Code, rr.Body.String())
	}

	r.resultErr = errors.New("none")
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/diagnostics/result", nil)
	h.Result(rr, req)
	if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), `NO_REPORT`) {
		t.Fatalf("Result error failed: code=%d body=%s", rr.Code, rr.Body.String())
	}
}
