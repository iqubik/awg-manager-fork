package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/singboxwatchdog"
)

type fakeSingboxWatchdogService struct {
	configureErr error
	enableErr    error
	disableErr   error
	checkID      string
	checkAll     bool
}

func (f *fakeSingboxWatchdogService) GetStatus(ctx context.Context) []singboxwatchdog.TargetStatus {
	return nil
}
func (f *fakeSingboxWatchdogService) GetLogs(targetID string) []singboxwatchdog.LogEntry { return nil }
func (f *fakeSingboxWatchdogService) ClearLogs()                                         {}
func (f *fakeSingboxWatchdogService) Configure(cfg singboxwatchdog.TargetConfig) error {
	return f.configureErr
}
func (f *fakeSingboxWatchdogService) Enable(id string) error  { return f.enableErr }
func (f *fakeSingboxWatchdogService) Disable(id string) error { return f.disableErr }
func (f *fakeSingboxWatchdogService) CheckNow(ctx context.Context, targetID string) {
	f.checkID = targetID
}
func (f *fakeSingboxWatchdogService) CheckAllNow(ctx context.Context) { f.checkAll = true }

func TestSingboxWatchdogHandler_Configure_MapsMissingTargetToBadRequest(t *testing.T) {
	h := NewSingboxWatchdogHandler(&fakeSingboxWatchdogService{
		configureErr: singboxwatchdog.ErrTargetNotFound,
	}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/singbox/watchdog/configure", strings.NewReader(`{
		"id":"tunnel:missing","kind":"tunnel","ref":"missing","enabled":true,"interval":30,"failThreshold":3,"timeout":5,"recoveryMode":"off"
	}`))
	w := httptest.NewRecorder()
	h.Configure(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestSingboxWatchdogHandler_CheckNow_EmptyBodyCallsAll(t *testing.T) {
	svc := &fakeSingboxWatchdogService{}
	h := NewSingboxWatchdogHandler(svc, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/singbox/watchdog/check-now", strings.NewReader(``))
	w := httptest.NewRecorder()
	h.CheckNow(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !svc.checkAll || svc.checkID != "" {
		t.Fatalf("expected CheckAllNow only, got checkAll=%v checkID=%q", svc.checkAll, svc.checkID)
	}
}

func TestSingboxWatchdogHandler_Enable_MapsMissingTargetToBadRequest(t *testing.T) {
	h := NewSingboxWatchdogHandler(&fakeSingboxWatchdogService{
		enableErr: singboxwatchdog.ErrTargetNotFound,
	}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/singbox/watchdog/enable", strings.NewReader(`{"id":"tunnel:missing"}`))
	w := httptest.NewRecorder()
	h.Enable(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestSingboxWatchdogHandler_Disable_MapsMissingTargetToBadRequest(t *testing.T) {
	h := NewSingboxWatchdogHandler(&fakeSingboxWatchdogService{
		disableErr: singboxwatchdog.ErrTargetNotFound,
	}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/singbox/watchdog/disable", strings.NewReader(`{"id":"tunnel:missing"}`))
	w := httptest.NewRecorder()
	h.Disable(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestSingboxWatchdogHandler_Configure_UnexpectedErrorIsInternal(t *testing.T) {
	h := NewSingboxWatchdogHandler(&fakeSingboxWatchdogService{
		configureErr: errors.New("disk full"),
	}, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/singbox/watchdog/configure", strings.NewReader(`{
		"id":"tunnel:sb-main","kind":"tunnel","ref":"sb-main","enabled":true,"interval":30,"failThreshold":3,"timeout":5,"recoveryMode":"off"
	}`))
	w := httptest.NewRecorder()
	h.Configure(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", w.Code, w.Body.String())
	}
}
