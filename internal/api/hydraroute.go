package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hoaxisr/awg-manager/internal/downloader"
	"github.com/hoaxisr/awg-manager/internal/events"
	"github.com/hoaxisr/awg-manager/internal/hydraroute"
	"github.com/hoaxisr/awg-manager/internal/response"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

// ── Response DTOs ────────────────────────────────────────────────

// HydraRouteConfigData mirrors frontend HydraRouteConfig.
type HydraRouteConfigData struct {
	AutoStart          bool     `json:"autoStart" example:"true"`
	ClearIPSet         bool     `json:"clearIPSet" example:"false"`
	CIDR               bool     `json:"cidr" example:"true"`
	IpsetEnableTimeout bool     `json:"ipsetEnableTimeout" example:"false"`
	IpsetTimeout       int      `json:"ipsetTimeout" example:"0"`
	IpsetMaxElem       int      `json:"ipsetMaxElem" example:"65536"`
	DirectRouteEnabled bool     `json:"directRouteEnabled" example:"false"`
	GlobalRouting      bool     `json:"globalRouting" example:"false"`
	ConntrackFlush     bool     `json:"conntrackFlush" example:"true"`
	Log                string   `json:"log" example:"warn"`
	LogFile            string   `json:"logFile" example:"/var/log/hrneo.log"`
	GeoIPFiles         []string `json:"geoIPFiles" example:"/opt/etc/hrneo/geoip.db"`
	GeoSiteFiles       []string `json:"geoSiteFiles" example:"/opt/etc/hrneo/geosite.db"`
	PolicyOrder        []string `json:"policyOrder" example:"default"`
}

// HydraRouteConfigResponse is the envelope for GET /hydraroute/config.
type HydraRouteConfigResponse struct {
	Success bool                 `json:"success" example:"true"`
	Data    HydraRouteConfigData `json:"data"`
}

// GeoFileEntryDTO mirrors frontend GeoFileEntry.
type GeoFileEntryDTO struct {
	Type     string `json:"type" example:"geosite"`
	Path     string `json:"path" example:"/opt/etc/hrneo/geosite.db"`
	URL      string `json:"url" example:"https://cdn.example.com/geosite.db"`
	Size     int64  `json:"size" example:"3145728"`
	TagCount int    `json:"tagCount" example:"420"`
	Updated  string `json:"updated" example:"2024-01-15T02:00:00Z"`
	External bool   `json:"external,omitempty" example:"true"`
}

// GeoFilesResponse is the envelope for GET /hydraroute/geo-files.
type GeoFilesResponse struct {
	Success bool              `json:"success" example:"true"`
	Data    []GeoFileEntryDTO `json:"data"`
}

// GeoUpdateScheduleDTO mirrors frontend GeoUpdateSchedule.
type GeoUpdateScheduleDTO struct {
	Interval  string `json:"interval" example:"off"`
	UpdatedAt string `json:"updatedAt,omitempty" example:"2024-01-15T02:00:00Z"`
}

// GeoUpdateScheduleResponse is the envelope for /hydraroute/geo-files/schedule.
type GeoUpdateScheduleResponse struct {
	Success bool                 `json:"success" example:"true"`
	Data    GeoUpdateScheduleDTO `json:"data"`
}

// GeoTagDTO mirrors frontend GeoTag.
type GeoTagDTO struct {
	Name  string `json:"name" example:"google"`
	Count int    `json:"count" example:"1250"`
}

// GeoTagsResponse is the envelope for GET /hydraroute/geo-tags.
type GeoTagsResponse struct {
	Success bool        `json:"success" example:"true"`
	Data    []GeoTagDTO `json:"data"`
}

// GeoExpandData is the payload for GET /hydraroute/geo-expand.
type GeoExpandData struct {
	Lines []string `json:"lines"`
	Path  string   `json:"path"`
	Count int      `json:"count"`
}

// GeoFileResponse is the envelope for endpoints that return a single
// geo file entry (POST /hydraroute/geo-files/add).
type GeoFileResponse struct {
	Success bool            `json:"success" example:"true"`
	Data    GeoFileEntryDTO `json:"data"`
}

// GeoFileUpdatedData reports how many geo files were re-downloaded by
// POST /hydraroute/geo-files/update. The shape is the same whether the
// caller targeted one path or all paths.
type GeoFileUpdatedData struct {
	Updated int    `json:"updated" example:"1"`
	Partial bool   `json:"partial,omitempty" example:"true"`
	Error   string `json:"error,omitempty"`
}

// GeoFileUpdatedResponse is the envelope for POST /hydraroute/geo-files/update.
type GeoFileUpdatedResponse struct {
	Success bool               `json:"success" example:"true"`
	Data    GeoFileUpdatedData `json:"data"`
}

// IpsetUsageData mirrors hydraroute.IpsetUsage.
type IpsetUsageData struct {
	MaxElem int            `json:"maxElem" example:"65536"`
	Usage   map[string]int `json:"usage"`
}

// IpsetUsageResponse is the envelope for GET /hydraroute/ipset-usage.
type IpsetUsageResponse struct {
	Success bool           `json:"success" example:"true"`
	Data    IpsetUsageData `json:"data"`
}

// PolicyOrderData reports the current HrNeo policy order.
type PolicyOrderData struct {
	Order []string `json:"order" example:"default"`
}

// PolicyOrderResponse is the envelope for POST /hydraroute/policy-order.
type PolicyOrderResponse struct {
	Success bool            `json:"success" example:"true"`
	Data    PolicyOrderData `json:"data"`
}

// OversizedTagDTO mirrors hydraroute.OversizedTag.
type OversizedTagDTO struct {
	Name  string `json:"name" example:"netflix"`
	Count int    `json:"count" example:"82000"`
	File  string `json:"file" example:"/opt/etc/hrneo/geosite.db"`
}

// OversizedTagsData reports geoip tags excluded by HrNeo together with
// the active IpsetMaxElem so the frontend can render the disabled-tags
// pane and enforce picker limits.
type OversizedTagsData struct {
	Installed bool              `json:"installed" example:"true"`
	MaxElem   int               `json:"maxelem" example:"65536"`
	Tags      []OversizedTagDTO `json:"tags"`
}

// OversizedTagsResponse is the envelope for GET /hydraroute/oversized-tags.
type OversizedTagsResponse struct {
	Success bool              `json:"success" example:"true"`
	Data    OversizedTagsData `json:"data"`
}

// ── Request DTOs ─────────────────────────────────────────────────

// AddGeoFileRequest is the body for POST /hydraroute/geo-files/add.
type AddGeoFileRequest struct {
	Type  string            `json:"type" example:"geosite"`
	URL   string            `json:"url" example:"https://cdn.example.com/geosite.db"`
	Route *DownloadRouteDTO `json:"route,omitempty"`
}

// UpdateGeoFileRequest is the body for POST /hydraroute/geo-files/update.
// Empty path triggers a bulk refresh of every tracked geo file.
type UpdateGeoFileRequest struct {
	Path  string            `json:"path" example:"/opt/etc/hrneo/geosite.db"`
	Route *DownloadRouteDTO `json:"route,omitempty"`
}

// TakeGeoFileControlRequest is the body for POST /hydraroute/geo-files/take-control.
type TakeGeoFileControlRequest struct {
	Path string `json:"path" example:"/opt/etc/HydraRoute/geosite_GA.dat"`
}

// GeoFilesRescannedData reports how many new paths were adopted from hrneo.conf.
type GeoFilesRescannedData struct {
	Adopted int `json:"adopted" example:"1"`
}

// GeoFilesRescannedResponse is the envelope for POST /hydraroute/geo-files/rescan.
type GeoFilesRescannedResponse struct {
	Success bool                  `json:"success" example:"true"`
	Data    GeoFilesRescannedData `json:"data"`
}

// SetPolicyOrderRequest is the body for POST /hydraroute/policy-order.
type SetPolicyOrderRequest struct {
	Order []string `json:"order" example:"default"`
}

// SetGeoUpdateScheduleRequest is the body for PUT /hydraroute/geo-files/schedule.
type SetGeoUpdateScheduleRequest struct {
	Interval string `json:"interval" example:"daily"`
}

// HydraRouteHandler handles HydraRoute Neo settings API endpoints.
type HydraRouteHandler struct {
	svc         *hydraroute.Service
	bus         *events.Bus
	downloadSvc *downloader.Service
	settings    *storage.SettingsStore
}

// NewHydraRouteHandler creates a new HydraRoute settings handler.
func NewHydraRouteHandler(
	svc *hydraroute.Service,
	downloadSvc *downloader.Service,
	settings *storage.SettingsStore,
) *HydraRouteHandler {
	if downloadSvc == nil {
		downloadSvc = downloader.NewService(downloader.Deps{})
	}
	return &HydraRouteHandler{
		svc:         svc,
		downloadSvc: downloadSvc,
		settings:    settings,
	}
}

func toDownloaderRoute(route *DownloadRouteDTO) *downloader.Route {
	if route == nil {
		return nil
	}
	return &downloader.Route{
		Tag:  route.Tag,
		Kind: route.Kind,
	}
}

func legacyGeoScheduleFromSettings(geo storage.GeoFileSettings) GeoUpdateScheduleDTO {
	if !geo.AutoRefreshEnabled {
		return GeoUpdateScheduleDTO{Interval: hydraroute.GeoUpdateOff}
	}

	if strings.EqualFold(strings.TrimSpace(geo.RefreshMode), "daily") {
		return GeoUpdateScheduleDTO{Interval: hydraroute.GeoUpdateDay}
	}

	hours := geo.RefreshIntervalHours
	switch hours {
	case 0:
		return GeoUpdateScheduleDTO{Interval: hydraroute.GeoUpdateOff}
	case 1:
		return GeoUpdateScheduleDTO{Interval: hydraroute.GeoUpdateHour}
	case 6:
		return GeoUpdateScheduleDTO{Interval: hydraroute.GeoUpdate6H}
	case 168:
		return GeoUpdateScheduleDTO{Interval: hydraroute.GeoUpdateWeek}
	default:
		return GeoUpdateScheduleDTO{Interval: fmt.Sprintf("%dh", hours)}
	}
}

func geoFileSettingsFromLegacyInterval(
	interval string,
	base storage.GeoFileSettings,
) (storage.GeoFileSettings, error) {
	normalized := strings.ToLower(strings.TrimSpace(interval))
	switch normalized {
	case "", hydraroute.GeoUpdateOff:
		base.AutoRefreshEnabled = false
		return base, nil
	case hydraroute.GeoUpdateHour:
		base.AutoRefreshEnabled = true
		base.RefreshMode = "interval"
		base.RefreshIntervalHours = 1
		return base, nil
	case hydraroute.GeoUpdate6H:
		base.AutoRefreshEnabled = true
		base.RefreshMode = "interval"
		base.RefreshIntervalHours = 6
		return base, nil
	case hydraroute.GeoUpdateDay:
		base.AutoRefreshEnabled = true
		base.RefreshMode = "daily"
		if strings.TrimSpace(base.RefreshDailyTime) == "" {
			base.RefreshDailyTime = "03:00"
		}
		return base, nil
	case hydraroute.GeoUpdateWeek:
		base.AutoRefreshEnabled = true
		base.RefreshMode = "interval"
		base.RefreshIntervalHours = 168
		return base, nil
	}

	if strings.HasSuffix(normalized, "h") {
		hours, err := strconv.Atoi(strings.TrimSuffix(normalized, "h"))
		if err != nil || hours <= 0 {
			return base, fmt.Errorf("unsupported geo schedule interval %q", interval)
		}
		base.AutoRefreshEnabled = true
		base.RefreshMode = "interval"
		base.RefreshIntervalHours = hours
		return base, nil
	}

	return base, fmt.Errorf("unsupported geo schedule interval %q", interval)
}

// SetEventBus wires the SSE bus so HR Neo mutations that touch the DNS
// route list (policy order, native rule import, config write) can emit
// resource:invalidated hints for `routing.dnsRoutes`, and so HR daemon
// state changes publish `routing.hydrarouteStatus` hints.
func (h *HydraRouteHandler) SetEventBus(bus *events.Bus) { h.bus = bus }

// GetConfig returns the current HydraRoute config.
//
//	@Summary		Get HydraRoute config
//	@Description	Available when HydraRoute service is wired on the device.
//	@Tags			hydraroute
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	HydraRouteConfigResponse
//	@Router			/hydraroute/config [get]
func (h *HydraRouteHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}

	cfg, err := h.svc.ReadConfig()
	if err != nil {
		response.Error(w, err.Error(), "CONFIG_READ_ERROR")
		return
	}

	response.Success(w, cfg)
}

// UpdateConfig writes the HydraRoute config.
//
//	@Summary		Update HydraRoute config
//	@Description	Persists the HydraRoute (HrNeo) config and schedules a neo restart. The cached status becomes stale and is invalidated via SSE.
//	@Tags			hydraroute
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		HydraRouteConfigData	true	"hydraroute.Config"
//	@Success		200		{object}	HydraRouteConfigResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/hydraroute/config/update [put]
func (h *HydraRouteHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		response.MethodNotAllowed(w)
		return
	}

	var cfg hydraroute.Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		response.Error(w, "invalid request body: "+err.Error(), "BAD_REQUEST")
		return
	}

	if err := h.svc.WriteConfig(&cfg); err != nil {
		response.Error(w, err.Error(), "CONFIG_WRITE_ERROR")
		return
	}

	// WriteConfig schedules a neo restart that flips the running flag,
	// so the cached hydraroute status becomes stale.
	publishInvalidated(h.bus, ResourceRoutingHydrarouteStatus, "config-write")
	response.Success(w, cfg)
}

// ListGeoFiles returns all tracked geo data files.
//
//	@Summary		List HydraRoute geo files
//	@Tags			hydraroute
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	GeoFilesResponse
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/hydraroute/geo-files [get]
func (h *HydraRouteHandler) ListGeoFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}

	gds := h.svc.GetGeoData()
	if gds == nil {
		response.Success(w, []hydraroute.GeoFileEntry{})
		return
	}

	response.Success(w, response.MustNotNil(gds.List()))
}

// GetGeoUpdateSchedule returns the current geo-data auto-update interval.
//
//	@Summary		Get HydraRoute geo file auto-update schedule
//	@Tags			hydraroute
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	GeoUpdateScheduleResponse
//	@Failure		400	{object}	APIErrorEnvelope
//	@Router			/hydraroute/geo-files/schedule [get]
func (h *HydraRouteHandler) GetGeoUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}

	if h.settings == nil {
		response.Error(w, "settings store not initialized", "NOT_INITIALIZED")
		return
	}

	settings, err := h.settings.Get()
	if err != nil {
		response.Error(w, "failed to load settings: "+err.Error(), "SETTINGS_READ_ERROR")
		return
	}

	response.Success(w, legacyGeoScheduleFromSettings(settings.GeoFile))
}

// SetGeoUpdateSchedule updates the persisted geo-data auto-update interval.
//
//	@Summary		Set HydraRoute geo file auto-update schedule
//	@Tags			hydraroute
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SetGeoUpdateScheduleRequest	true	"Geo auto-update schedule interval"
//	@Success		200		{object}	GeoUpdateScheduleResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Router			/hydraroute/geo-files/schedule [put]
func (h *HydraRouteHandler) SetGeoUpdateSchedule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		response.MethodNotAllowed(w)
		return
	}

	var req SetGeoUpdateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "invalid request body: "+err.Error(), "BAD_REQUEST")
		return
	}

	if h.settings == nil {
		response.Error(w, "settings store not initialized", "NOT_INITIALIZED")
		return
	}

	settings, err := h.settings.Get()
	if err != nil {
		response.Error(w, "failed to load settings: "+err.Error(), "SETTINGS_READ_ERROR")
		return
	}

	nextGeo, err := geoFileSettingsFromLegacyInterval(req.Interval, settings.GeoFile)
	if err != nil {
		response.Error(w, err.Error(), "GEO_SCHEDULE_ERROR")
		return
	}

	settings.GeoFile = nextGeo
	if err := h.settings.Save(settings); err != nil {
		response.Error(w, "failed to save settings: "+err.Error(), "SETTINGS_WRITE_ERROR")
		return
	}

	response.Success(w, legacyGeoScheduleFromSettings(nextGeo))
}

// AddGeoFile downloads and registers a new geo data file.
//
//	@Summary		Add HydraRoute geo file
//	@Tags			hydraroute
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		AddGeoFileRequest	true	"Geo file source descriptor"
//	@Success		200		{object}	GeoFileResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/hydraroute/geo-files/add [post]
func (h *HydraRouteHandler) AddGeoFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}

	var req AddGeoFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "invalid request body: "+err.Error(), "BAD_REQUEST")
		return
	}

	gds := h.svc.GetGeoData()
	if gds == nil {
		response.Error(w, "geo data store not initialized", "NOT_INITIALIZED")
		return
	}
	lease, err := h.downloadSvc.ResolveClient(r.Context(), toDownloaderRoute(req.Route))
	if err != nil {
		response.Error(w, err.Error(), "GEO_DOWNLOAD_ROUTE_ERROR")
		return
	}
	defer lease.Close()

	routeLabel := lease.Route.DisplayName()

	entry, err := gds.DownloadWithClientVia(r.Context(), req.Type, req.URL, lease.Client, routeLabel)
	if err != nil {
		response.Error(w, fmt.Sprintf("download via %s: %v", routeLabel, err), "GEO_DOWNLOAD_ERROR")
		return
	}

	if err := h.svc.SyncGeoFilesToConfig(); err != nil {
		response.Error(w, "downloaded but failed to sync config: "+err.Error(), "SYNC_ERROR")
		return
	}

	response.Success(w, entry)
}

// DeleteGeoFile removes a tracked geo data file.
//
//	@Summary		Delete HydraRoute geo file
//	@Description	Removes the tracked geo data file at the given path and re-syncs the geo file list to config.
//	@Tags			hydraroute
//	@Produce		json
//	@Security		CookieAuth
//	@Param			path	query		string	true	"Filesystem path of the geo file"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/hydraroute/geo-files/delete [delete]
func (h *HydraRouteHandler) DeleteGeoFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		response.MethodNotAllowed(w)
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		response.Error(w, "path query parameter is required", "BAD_REQUEST")
		return
	}

	gds := h.svc.GetGeoData()
	if gds == nil {
		response.Error(w, "geo data store not initialized", "NOT_INITIALIZED")
		return
	}

	if err := gds.Delete(path); err != nil {
		response.Error(w, err.Error(), "GEO_DELETE_ERROR")
		return
	}

	if err := h.svc.SyncGeoFilesToConfig(); err != nil {
		response.Error(w, "deleted but failed to sync config: "+err.Error(), "SYNC_ERROR")
		return
	}

	response.Success(w, map[string]bool{"ok": true})
}

// TakeGeoFileControl moves an external HydraRoute file into awg-manager/geo.
//
//	@Summary		Take HydraRoute geo file under awg-manager control
//	@Tags			hydraroute
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		TakeGeoFileControlRequest	true	"Filesystem path of the external geo file"
//	@Success		200		{object}	GeoFileResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/hydraroute/geo-files/take-control [post]
func (h *HydraRouteHandler) TakeGeoFileControl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}

	var req TakeGeoFileControlRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "invalid request body: "+err.Error(), "BAD_REQUEST")
		return
	}
	if req.Path == "" {
		response.Error(w, "path is required", "BAD_REQUEST")
		return
	}

	gds := h.svc.GetGeoData()
	if gds == nil {
		response.Error(w, "geo data store not initialized", "NOT_INITIALIZED")
		return
	}

	entry, err := gds.TakeControl(req.Path)
	if err != nil {
		response.Error(w, err.Error(), "GEO_TAKE_CONTROL_ERROR")
		return
	}

	if err := h.svc.SyncGeoFilesToConfig(); err != nil {
		response.Error(w, "moved but failed to sync config: "+err.Error(), "SYNC_ERROR")
		return
	}

	response.Success(w, entry)
}

// RescanGeoFiles adopts geo paths from hrneo.conf not yet in the catalog.
//
//	@Summary		Rescan HydraRoute geo files from hrneo.conf
//	@Tags			hydraroute
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	GeoFilesRescannedResponse
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/hydraroute/geo-files/rescan [post]
func (h *HydraRouteHandler) RescanGeoFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}

	n, err := h.svc.RescanGeoFiles()
	if err != nil {
		response.Error(w, err.Error(), "GEO_RESCAN_ERROR")
		return
	}

	response.Success(w, GeoFilesRescannedData{Adopted: n})
}

// UpdateGeoFile re-downloads a geo data file (or all files if path is empty).
//
//	@Summary		Refresh HydraRoute geo file(s)
//	@Description	Empty path triggers a bulk refresh of every tracked geo file. Single path refreshes one file. Both branches return an updated count for caller-side feedback; the frontend refetches the list afterwards.
//	@Tags			hydraroute
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		UpdateGeoFileRequest	true	"Path to refresh, or empty to refresh all"
//	@Success		200		{object}	GeoFileUpdatedResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/hydraroute/geo-files/update [post]
func (h *HydraRouteHandler) UpdateGeoFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}

	var req UpdateGeoFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "invalid request body: "+err.Error(), "BAD_REQUEST")
		return
	}

	gds := h.svc.GetGeoData()
	if gds == nil {
		response.Error(w, "geo data store not initialized", "NOT_INITIALIZED")
		return
	}
	lease, err := h.downloadSvc.ResolveClient(r.Context(), toDownloaderRoute(req.Route))
	if err != nil {
		response.Error(w, err.Error(), "GEO_DOWNLOAD_ROUTE_ERROR")
		return
	}
	defer lease.Close()

	routeLabel := lease.Route.DisplayName()

	out := GeoFileUpdatedData{}
	if req.Path == "" {
		count, err := gds.UpdateAllWithClientVia(r.Context(), lease.Client, routeLabel)
		out.Updated = count
		if err != nil {
			out.Partial = count > 0
			out.Error = fmt.Sprintf("update via %s: %v", routeLabel, err)
			if count == 0 {
				response.Error(w, fmt.Sprintf("update via %s: %v", routeLabel, err), "GEO_UPDATE_ERROR")
				return
			}
		}
	} else {
		if _, err := gds.UpdateWithClientVia(r.Context(), req.Path, lease.Client, routeLabel); err != nil {
			response.Error(w, fmt.Sprintf("update via %s: %v", routeLabel, err), "GEO_UPDATE_ERROR")
			return
		}
		out.Updated = 1
	}

	if err := h.svc.SyncGeoFilesToConfig(); err != nil {
		if out.Updated > 0 {
			out.Partial = true
			if out.Error != "" {
				out.Error += "; "
			}
			out.Error += "sync config: " + err.Error()
			response.Success(w, out)
			return
		}
		response.Error(w, "updated but failed to sync config: "+err.Error(), "SYNC_ERROR")
		return
	}

	response.Success(w, out)
}

// GetGeoTags returns the tag list for a specific geo data file.
//
//	@Summary		HydraRoute geo tags
//	@Tags			hydraroute
//	@Produce		json
//	@Security		CookieAuth
//	@Param			path	query	string	true	"Geo file path"
//	@Success		200	{array}	string
//	@Router			/hydraroute/geo-tags [get]
func (h *HydraRouteHandler) GetGeoTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		response.Error(w, "path query parameter is required", "BAD_REQUEST")
		return
	}

	gds := h.svc.GetGeoData()
	if gds == nil {
		response.Error(w, "geo data store not initialized", "NOT_INITIALIZED")
		return
	}

	tags, err := gds.GetTags(path)
	if err != nil {
		response.Error(w, err.Error(), "GEO_TAGS_ERROR")
		return
	}

	response.Success(w, response.MustNotNil(tags))
}

// ExpandGeoTag expands a geosite:/geoip: tag into inline rule list lines.
//
//	@Summary		HydraRoute geo tag expand
//	@Tags			hydraroute
//	@Produce		json
//	@Security		CookieAuth
//	@Param			kind	query	string	true	"geosite or geoip"
//	@Param			tag	query	string	true	"Tag name"
//	@Success		200	{object}	GeoExpandData
//	@Router			/hydraroute/geo-expand [get]
func (h *HydraRouteHandler) ExpandGeoTag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}

	kind := r.URL.Query().Get("kind")
	tag := r.URL.Query().Get("tag")
	if kind == "" || tag == "" {
		response.Error(w, "kind and tag query parameters are required", "BAD_REQUEST")
		return
	}

	gds := h.svc.GetGeoData()
	if gds == nil {
		response.Error(w, "geo data store not initialized", "NOT_INITIALIZED")
		return
	}

	lines, path, err := gds.ExpandGeoTag(kind, tag)
	if err != nil {
		response.Error(w, err.Error(), "GEO_EXPAND_ERROR")
		return
	}

	response.Success(w, GeoExpandData{
		Lines: lines,
		Path:  path,
		Count: len(lines),
	})
}

// GetIpsetUsage returns the current ipset usage per kernel interface.
//
//	@Summary		HydraRoute ipset usage
//	@Tags			hydraroute
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	IpsetUsageResponse
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/hydraroute/ipset-usage [get]
func (h *HydraRouteHandler) GetIpsetUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}

	usage, err := h.svc.CalculateIpsetUsage()
	if err != nil {
		response.Error(w, err.Error(), "IPSET_USAGE_ERROR")
		return
	}

	response.Success(w, usage)
}

// SetPolicyOrder updates the PolicyOrder in hrneo.conf.
//
//	@Summary		Set HydraRoute policy order
//	@Tags			hydraroute
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SetPolicyOrderRequest	true	"Ordered list of policy names"
//	@Success		200		{object}	PolicyOrderResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/hydraroute/policy-order [post]
func (h *HydraRouteHandler) SetPolicyOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}

	var req SetPolicyOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, "invalid request body: "+err.Error(), "BAD_REQUEST")
		return
	}

	if err := h.svc.SetPolicyOrder(req.Order); err != nil {
		response.Error(w, err.Error(), "POLICY_ORDER_ERROR")
		return
	}

	publishInvalidated(h.bus, ResourceRoutingDnsRoutes, "policy-order")
	// Policy-order changes trigger a neo restart too.
	publishInvalidated(h.bus, ResourceRoutingHydrarouteStatus, "policy-order")
	response.Success(w, map[string][]string{"order": req.Order})
}

// GetOversizedTags returns the list of geoip tags HR Neo excluded plus
// the current IpsetMaxElem so the frontend can render the 'Отключённые
// теги' pane and enforce picker limits.
//
//	@Summary		HydraRoute oversized geo tags
//	@Tags			hydraroute
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	OversizedTagsResponse
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/hydraroute/oversized-tags [get]
func (h *HydraRouteHandler) GetOversizedTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}

	status := h.svc.GetStatus()
	if !status.Installed {
		response.Success(w, map[string]interface{}{
			"installed": false,
			"maxelem":   0,
			"tags":      []hydraroute.OversizedTag{},
		})
		return
	}

	cfg, err := h.svc.ReadConfig()
	if err != nil {
		response.Error(w, err.Error(), "CONFIG_READ_ERROR")
		return
	}

	tags, err := h.svc.OversizedTags(r.Context())
	if err != nil {
		response.Error(w, err.Error(), "OVERSIZED_ERROR")
		return
	}

	response.Success(w, map[string]interface{}{
		"installed": true,
		"maxelem":   cfg.EffectiveMaxElem(),
		"tags":      response.MustNotNil(tags),
	})
}
