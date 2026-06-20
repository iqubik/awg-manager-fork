package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/downloader"
	"github.com/hoaxisr/awg-manager/internal/hydraroute"
	"github.com/hoaxisr/awg-manager/internal/response"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

func newHydraRouteSettingsStore(t *testing.T) *storage.SettingsStore {
	t.Helper()
	store := storage.NewSettingsStore(t.TempDir())
	if _, err := store.Load(); err != nil {
		t.Fatalf("load settings: %v", err)
	}
	return store
}

func TestToDownloaderRoute(t *testing.T) {
	if got := toDownloaderRoute(nil); got != nil {
		t.Fatalf("nil route: got %+v", got)
	}
	got := toDownloaderRoute(&DownloadRouteDTO{Tag: "awg-a", Kind: "awg"})
	if got == nil {
		t.Fatal("expected non-nil route")
	}
	if got.Tag != "awg-a" || got.Kind != "awg" {
		t.Fatalf("unexpected route: %+v", got)
	}
}

func TestDownloadDeviceProxyAdapter(t *testing.T) {
	adapter := downloader.NewDeviceProxyOutboundsProvider(nil)
	if adapter != nil {
		t.Fatalf("nil service should produce nil provider")
	}
	dl := downloader.NewService(downloader.Deps{})
	list := dl.ListOutbounds(context.Background())
	if len(list) == 0 {
		t.Fatal("expected at least direct outbound")
	}
	if list[0].Tag != "direct" {
		t.Fatalf("first outbound tag: got %q want direct", list[0].Tag)
	}
}

func TestDownloadTransportAdaptersNilSafe(t *testing.T) {
	if got := downloader.NewSingboxTunnelPortAdapter(nil); got != nil {
		t.Fatalf("nil singbox op should produce nil tunnel port adapter")
	}
	if got := downloader.NewSubscriptionPortAdapter(nil); got != nil {
		t.Fatalf("nil subscription svc should produce nil port adapter")
	}
	if got := downloader.NewSingboxRuntimeAdapter(nil); got != nil {
		t.Fatalf("nil singbox op should produce nil runtime adapter")
	}
}

func TestDownloadSettingsRouteProvider_DefaultDirect(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewSettingsStore(dir)
	if _, err := store.Load(); err != nil {
		t.Fatalf("load settings: %v", err)
	}
	p := downloader.NewSettingsRouteProvider(store)
	route, err := p.GetDownloadRoute(context.Background())
	if err != nil {
		t.Fatalf("get route: %v", err)
	}
	if route == nil || route.Tag != "direct" {
		t.Fatalf("route = %+v, want direct", route)
	}
}

func TestDownloadSettingsRouteProvider_UsesStoredTag(t *testing.T) {
	dir := t.TempDir()
	store := storage.NewSettingsStore(dir)
	st, err := store.Load()
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	st.Download.RouteTag = "awg-test"
	if err := store.Save(st); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	p := downloader.NewSettingsRouteProvider(store)
	route, err := p.GetDownloadRoute(context.Background())
	if err != nil {
		t.Fatalf("get route: %v", err)
	}
	if route == nil || route.Tag != "awg-test" {
		t.Fatalf("route = %+v, want awg-test", route)
	}

	// Ensure empty value is normalized to direct.
	st.Download.RouteTag = ""
	if err := store.Save(st); err != nil {
		t.Fatalf("save empty routeTag: %v", err)
	}
	route, err = p.GetDownloadRoute(context.Background())
	if err != nil {
		t.Fatalf("get route after empty: %v", err)
	}
	if route == nil || route.Tag != "direct" {
		t.Fatalf("route after empty = %+v, want direct", route)
	}

}

func TestHydraRouteHandler_GetGeoUpdateSchedule(t *testing.T) {
	svc := hydraroute.NewService(nil, nil)
	settingsStore := newHydraRouteSettingsStore(t)
	handler := NewHydraRouteHandler(svc, nil, settingsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/hydraroute/geo-files/schedule", nil)
	rr := httptest.NewRecorder()
	handler.GetGeoUpdateSchedule(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp response.APIResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("response = %+v, want success", resp)
	}

	var data GeoUpdateScheduleDTO
	raw, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal schedule: %v", err)
	}
	if data.Interval != hydraroute.GeoUpdateOff {
		t.Fatalf("Interval = %q, want %q", data.Interval, hydraroute.GeoUpdateOff)
	}
}

func TestHydraRouteHandler_GetGeoUpdateScheduleMapsCurrentGeoSettings(t *testing.T) {
	svc := hydraroute.NewService(nil, nil)
	settingsStore := newHydraRouteSettingsStore(t)
	settings, err := settingsStore.Get()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	settings.GeoFile = storage.GeoFileSettings{
		AutoRefreshEnabled:   true,
		RefreshMode:          "interval",
		RefreshIntervalHours: 12,
		RefreshDailyTime:     "03:00",
	}
	if err := settingsStore.Save(settings); err != nil {
		t.Fatalf("save settings: %v", err)
	}
	handler := NewHydraRouteHandler(svc, nil, settingsStore)

	req := httptest.NewRequest(http.MethodGet, "/api/hydraroute/geo-files/schedule", nil)
	rr := httptest.NewRecorder()
	handler.GetGeoUpdateSchedule(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var resp response.APIResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	raw, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	var data GeoUpdateScheduleDTO
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("unmarshal schedule: %v", err)
	}
	if data.Interval != "12h" {
		t.Fatalf("interval = %q, want %q", data.Interval, "12h")
	}
}

func TestHydraRouteHandler_GetGeoUpdateScheduleWithoutStore(t *testing.T) {
	svc := hydraroute.NewService(nil, nil)
	handler := NewHydraRouteHandler(svc, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/hydraroute/geo-files/schedule", nil)
	rr := httptest.NewRecorder()
	handler.GetGeoUpdateSchedule(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHydraRouteHandler_GetGeoUpdateScheduleWrongMethod(t *testing.T) {
	svc := hydraroute.NewService(nil, nil)
	handler := NewHydraRouteHandler(svc, nil, newHydraRouteSettingsStore(t))

	req := httptest.NewRequest(http.MethodPut, "/api/hydraroute/geo-files/schedule", nil)
	rr := httptest.NewRecorder()
	handler.GetGeoUpdateSchedule(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestHydraRouteHandler_SetGeoUpdateSchedule(t *testing.T) {
	svc := hydraroute.NewService(nil, nil)
	settingsStore := newHydraRouteSettingsStore(t)
	handler := NewHydraRouteHandler(svc, nil, settingsStore)

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/hydraroute/geo-files/schedule",
		strings.NewReader(`{"interval":"daily"}`),
	)
	rr := httptest.NewRecorder()
	handler.SetGeoUpdateSchedule(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	got, err := settingsStore.Get()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if !got.GeoFile.AutoRefreshEnabled || got.GeoFile.RefreshMode != "daily" || got.GeoFile.RefreshDailyTime != "03:00" {
		t.Fatalf("geo settings = %+v, want enabled daily 03:00", got.GeoFile)
	}
}

func TestHydraRouteHandler_SetGeoUpdateScheduleAllValidIntervals(t *testing.T) {
	tests := []struct {
		name      string
		interval  string
		assertion func(t *testing.T, got storage.GeoFileSettings)
	}{
		{
			name:     hydraroute.GeoUpdateOff,
			interval: hydraroute.GeoUpdateOff,
			assertion: func(t *testing.T, got storage.GeoFileSettings) {
				if got.AutoRefreshEnabled {
					t.Fatalf("geo settings = %+v, want disabled", got)
				}
			},
		},
		{
			name:     hydraroute.GeoUpdateHour,
			interval: hydraroute.GeoUpdateHour,
			assertion: func(t *testing.T, got storage.GeoFileSettings) {
				if !got.AutoRefreshEnabled || got.RefreshMode != "interval" || got.RefreshIntervalHours != 1 {
					t.Fatalf("geo settings = %+v, want hourly interval", got)
				}
			},
		},
		{
			name:     hydraroute.GeoUpdate6H,
			interval: hydraroute.GeoUpdate6H,
			assertion: func(t *testing.T, got storage.GeoFileSettings) {
				if !got.AutoRefreshEnabled || got.RefreshMode != "interval" || got.RefreshIntervalHours != 6 {
					t.Fatalf("geo settings = %+v, want 6h interval", got)
				}
			},
		},
		{
			name:     "12h",
			interval: "12h",
			assertion: func(t *testing.T, got storage.GeoFileSettings) {
				if !got.AutoRefreshEnabled || got.RefreshMode != "interval" || got.RefreshIntervalHours != 12 {
					t.Fatalf("geo settings = %+v, want 12h interval", got)
				}
			},
		},
		{
			name:     hydraroute.GeoUpdateDay,
			interval: hydraroute.GeoUpdateDay,
			assertion: func(t *testing.T, got storage.GeoFileSettings) {
				if !got.AutoRefreshEnabled || got.RefreshMode != "daily" || got.RefreshDailyTime != "03:00" {
					t.Fatalf("geo settings = %+v, want daily 03:00", got)
				}
			},
		},
		{
			name:     hydraroute.GeoUpdateWeek,
			interval: hydraroute.GeoUpdateWeek,
			assertion: func(t *testing.T, got storage.GeoFileSettings) {
				if !got.AutoRefreshEnabled || got.RefreshMode != "interval" || got.RefreshIntervalHours != 168 {
					t.Fatalf("geo settings = %+v, want weekly interval", got)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := hydraroute.NewService(nil, nil)
			settingsStore := newHydraRouteSettingsStore(t)
			handler := NewHydraRouteHandler(svc, nil, settingsStore)

			req := httptest.NewRequest(
				http.MethodPut,
				"/api/hydraroute/geo-files/schedule",
				strings.NewReader(`{"interval":"`+tc.interval+`"}`),
			)
			rr := httptest.NewRecorder()
			handler.SetGeoUpdateSchedule(rr, req)

			if rr.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
			}
			got, err := settingsStore.Get()
			if err != nil {
				t.Fatalf("get settings: %v", err)
			}
			tc.assertion(t, got.GeoFile)
		})
	}
}

func TestHydraRouteHandler_SetGeoUpdateScheduleRejectsInvalid(t *testing.T) {
	svc := hydraroute.NewService(nil, nil)
	handler := NewHydraRouteHandler(svc, nil, newHydraRouteSettingsStore(t))

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/hydraroute/geo-files/schedule",
		strings.NewReader(`{"interval":"never"}`),
	)
	rr := httptest.NewRecorder()
	handler.SetGeoUpdateSchedule(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHydraRouteHandler_SetGeoUpdateScheduleInvalidDoesNotMutate(t *testing.T) {
	svc := hydraroute.NewService(nil, nil)
	settingsStore := newHydraRouteSettingsStore(t)
	handler := NewHydraRouteHandler(svc, nil, settingsStore)

	req1 := httptest.NewRequest(
		http.MethodPut,
		"/api/hydraroute/geo-files/schedule",
		strings.NewReader(`{"interval":"daily"}`),
	)
	rr1 := httptest.NewRecorder()
	handler.SetGeoUpdateSchedule(rr1, req1)
	if rr1.Code != http.StatusOK {
		t.Fatalf("status daily = %d, want %d", rr1.Code, http.StatusOK)
	}

	req2 := httptest.NewRequest(
		http.MethodPut,
		"/api/hydraroute/geo-files/schedule",
		strings.NewReader(`{"interval":"never"}`),
	)
	rr2 := httptest.NewRecorder()
	handler.SetGeoUpdateSchedule(rr2, req2)
	if rr2.Code != http.StatusBadRequest {
		t.Fatalf("status invalid = %d, want %d", rr2.Code, http.StatusBadRequest)
	}
	got, err := settingsStore.Get()
	if err != nil {
		t.Fatalf("get settings: %v", err)
	}
	if !got.GeoFile.AutoRefreshEnabled || got.GeoFile.RefreshMode != "daily" || got.GeoFile.RefreshDailyTime != "03:00" {
		t.Fatalf("geo settings after invalid = %+v, want unchanged daily 03:00", got.GeoFile)
	}
}

func TestHydraRouteHandler_SetGeoUpdateScheduleBadJSON(t *testing.T) {
	svc := hydraroute.NewService(nil, nil)
	handler := NewHydraRouteHandler(svc, nil, newHydraRouteSettingsStore(t))

	req := httptest.NewRequest(
		http.MethodPut,
		"/api/hydraroute/geo-files/schedule",
		strings.NewReader(`{`),
	)
	rr := httptest.NewRecorder()
	handler.SetGeoUpdateSchedule(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusBadRequest)
	}
}

func TestHydraRouteHandler_SetGeoUpdateScheduleWrongMethod(t *testing.T) {
	svc := hydraroute.NewService(nil, nil)
	handler := NewHydraRouteHandler(svc, nil, newHydraRouteSettingsStore(t))

	req := httptest.NewRequest(http.MethodPost, "/api/hydraroute/geo-files/schedule", nil)
	rr := httptest.NewRecorder()
	handler.SetGeoUpdateSchedule(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}
