package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/hoaxisr/awg-manager/internal/logging"
	"github.com/hoaxisr/awg-manager/internal/response"
	"github.com/hoaxisr/awg-manager/internal/singbox/router"
	"github.com/hoaxisr/awg-manager/internal/storage"
	tunnelservice "github.com/hoaxisr/awg-manager/internal/tunnel/service"
)

// DTOs live in singbox_router_dto.go.

type SingboxRouterHandler struct {
	svc             router.Service
	log             *logging.ScopedLogger
	deviceProxyRefs tunnelservice.DeviceProxyRefChecker
	routerRefs      tunnelservice.RouterRefChecker
}

func NewSingboxRouterHandler(svc router.Service, appLogger logging.AppLogger) *SingboxRouterHandler {
	return &SingboxRouterHandler{
		svc: svc,
		log: logging.NewScopedLogger(appLogger, logging.GroupRouting, logging.SubSingboxRouter),
	}
}

// SetOutboundRefCheckers wires device-proxy and router reference guards for
// composite-outbound deletion (refuse 409 when the tag is still selected by a
// device-proxy instance).
func (h *SingboxRouterHandler) SetOutboundRefCheckers(dp tunnelservice.DeviceProxyRefChecker, r tunnelservice.RouterRefChecker) {
	h.deviceProxyRefs = dp
	h.routerRefs = r
}

// GetStatus returns the current sing-box router engine status.
//
//	@Summary		Get sing-box router status
//	@Description	Returns the singbox-router status snapshot (running, mode, policy/iptables state, rule/ruleset/outbound counts).
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	SingboxRouterStatusResponse
//	@Failure		405	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/singbox/router/status [get]
func (h *SingboxRouterHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	st, err := h.svc.GetStatus(r.Context())
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, st)
}

// Enable starts the singbox-router engine and installs iptables/policy rules.
//
//	@Summary		Enable singbox-router
//	@Description	Starts the singbox-router engine and installs iptables/policy rules. Returns 400 with code POLICY_NOT_CONFIGURED or POLICY_MISSING when the router policy mode is incomplete. Returns 503 SINGBOX_NOT_READY when sing-box did not become ready within the boot-wait window — iptables install is deliberately skipped to avoid orphaning DNS:53 redirects (issue #221).
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	OkResponse
//	@Failure		400	{object}	APIErrorEnvelope
//	@Failure		405	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Failure		503	{object}	APIErrorEnvelope	"sing-box did not come up in time"
//	@Router			/singbox/router/enable [post]
func (h *SingboxRouterHandler) Enable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	if err := h.svc.Enable(r.Context()); err != nil {
		if errors.Is(err, router.ErrPolicyNotConfigured) {
			response.ErrorWithStatus(w, http.StatusBadRequest, err.Error(), "POLICY_NOT_CONFIGURED")
			return
		}
		if errors.Is(err, router.ErrPolicyMissing) {
			response.ErrorWithStatus(w, http.StatusBadRequest, err.Error(), "POLICY_MISSING")
			return
		}
		if errors.Is(err, router.ErrSingboxNotReady) {
			response.ErrorWithStatus(w, http.StatusServiceUnavailable, err.Error(), "SINGBOX_NOT_READY")
			return
		}
		h.handleErr(w, "request", err)
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

// Disable stops the singbox-router engine and uninstalls iptables/policy rules.
//
//	@Summary		Disable singbox-router
//	@Description	Stops the singbox-router engine and uninstalls iptables/policy rules. Idempotent.
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	OkResponse
//	@Failure		405	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/singbox/router/disable [post]
func (h *SingboxRouterHandler) Disable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	if err := h.svc.Disable(r.Context()); err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

// SwitchMode orchestrates a routing-mode transition (off↔tproxy↔fakeip-tun)
// with directional fail-closed rollback. Progress is reported out-of-band as
// "singbox-router:transition" events on the existing events SSE stream
// (GET /events) — no new stream endpoint.
//
//	@Summary		Switch singbox-router routing mode
//	@Description	Orchestrates a routing-mode transition (off↔tproxy↔fakeip-tun): tears down the old mode then brings up the new one, with directional fail-closed rollback. Per-step progress is published as "singbox-router:transition" events on the existing GET /events SSE stream (see SingboxRouterTransitionData). Returns 400 INVALID_MODE for an unknown mode.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterModeRequest	true	"Target routing mode"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		405		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/mode [post]
func (h *SingboxRouterHandler) SwitchMode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var body SingboxRouterModeRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.BadRequest(w, "invalid request body: "+err.Error())
		return
	}
	switch body.Mode {
	case "off", "tproxy", "fakeip-tun":
	default:
		response.ErrorWithStatus(w, http.StatusBadRequest,
			"invalid routing mode (want off|tproxy|fakeip-tun)", "INVALID_MODE")
		return
	}
	mode := body.Mode
	go func() {
		if err := h.svc.SwitchRoutingMode(context.Background(), mode); err != nil {
			// Terminal errors are also emitted as singbox-router:transition SSE
			// events, but the bus silently drops them with zero subscribers —
			// this log line is the only guaranteed trace of a failed switch.
			h.log.Warn("mode-switch", mode, "SwitchRoutingMode failed: "+err.Error())
		}
	}()
	// Keep the documented OkResponse shape ({"ok":true}); 200 here means
	// "transition accepted and started", progress/terminal state arrives via
	// the singbox-router:transition SSE events.
	response.Success(w, map[string]bool{"ok": true})
}

// GetSettings reads singbox-router settings (policy-mode, defaults, etc.).
//
//	@Summary		Get singbox-router settings
//	@Description	Reads the current singbox-router settings (policy mode, defaults, ...).
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	SingboxRouterSettingsResponse
//	@Failure		405	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/singbox/router/settings [get]
func (h *SingboxRouterHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	s, err := h.svc.GetSettings(r.Context())
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, s)
}

// PutSettings persists singbox-router settings.
//
//	@Summary		Update singbox-router settings
//	@Description	Persists singbox-router settings. The router is restarted only when fields that affect the running config change.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterSettingsData	true	"Singbox-router settings payload"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		405		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/settings [post]
//	@Router			/singbox/router/settings [put]
func (h *SingboxRouterHandler) PutSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		response.MethodNotAllowed(w)
		return
	}
	var sr storage.SingboxRouterSettings
	if err := decodeBody(r, &sr); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.UpdateSettings(r.Context(), sr); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

// ListRules returns all singbox-router routing rules in priority order.
//
//	@Summary		List singbox-router rules
//	@Description	Returns all routing rules in priority (top-first) order.
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	SingboxRouterRulesListResponse
//	@Failure		405	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/singbox/router/rules/list [get]
func (h *SingboxRouterHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	rules, err := h.svc.ListRules(r.Context())
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, rules)
}

// AddRule appends a new singbox-router routing rule.
//
//	@Summary		Add singbox-router rule
//	@Description	Appends a new routing rule. Rule conditions reference rulesets/outbounds that must already exist.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterRuleDTO	true	"Routing rule payload"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/rules/add [post]
func (h *SingboxRouterHandler) AddRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var rule router.Rule
	if err := decodeBody(r, &rule); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.AddRule(r.Context(), rule); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

// UpdateRule replaces a rule at the given index with the provided one.
//
//	@Summary		Update singbox-router rule
//	@Description	Replaces the rule at index with the provided one. Index is the priority slot (0-based).
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterRuleUpdateRequest	true	"Index + replacement rule"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/rules/update [post]
func (h *SingboxRouterHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var body struct {
		Index int         `json:"index"`
		Rule  router.Rule `json:"rule"`
	}
	if err := decodeBody(r, &body); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.UpdateRule(r.Context(), body.Index, body.Rule); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

// DeleteRule removes the rule at the given index.
//
//	@Summary		Delete singbox-router rule
//	@Description	Removes the rule at the given index (0-based priority slot).
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterRuleDeleteRequest	true	"Index of the rule to remove"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/rules/delete [post]
func (h *SingboxRouterHandler) DeleteRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var body struct {
		Index int `json:"index"`
	}
	if err := decodeBody(r, &body); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.DeleteRule(r.Context(), body.Index); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

// MoveRule moves the rule from one priority slot to another.
//
//	@Summary		Move singbox-router rule
//	@Description	Moves the rule from index `from` to index `to` (both 0-based). Adjusts other rules' indices accordingly.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterRuleMoveRequest	true	"From-index and to-index"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/rules/move [post]
func (h *SingboxRouterHandler) MoveRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var body struct {
		From int `json:"from"`
		To   int `json:"to"`
	}
	if err := decodeBody(r, &body); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.MoveRule(r.Context(), body.From, body.To); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

// ListRuleSets returns all configured rulesets.
//
//	@Summary		List singbox-router rulesets
//	@Description	Returns all configured rulesets (downloaded geo files / inline lists), with their tag, type, and freshness metadata.
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	SingboxRouterRuleSetsListResponse
//	@Failure		405	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/singbox/router/rulesets/list [get]
func (h *SingboxRouterHandler) ListRuleSets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	rs, err := h.svc.ListRuleSets(r.Context())
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, rs)
}

// AddRuleSet registers a new ruleset (downloads if remote).
//
//	@Summary		Add singbox-router ruleset
//	@Description	Registers a new ruleset. For remote rulesets the file is downloaded synchronously.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterRuleSetDTO	true	"RuleSet payload"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/rulesets/add [post]
func (h *SingboxRouterHandler) AddRuleSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var rs router.RuleSet
	if err := decodeBody(r, &rs); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.AddRuleSet(r.Context(), rs); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

// UpdateRuleSet replaces the ruleset identified by tag with new content.
//
//	@Summary		Update singbox-router ruleset
//	@Description	Replaces the ruleset identified by tag with new content. If the payload tag differs, references are renamed atomically.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterRuleSetUpdateRequest	true	"Tag + new RuleSet payload"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		404		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/rulesets/update [post]
func (h *SingboxRouterHandler) UpdateRuleSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var body struct {
		Tag     string         `json:"tag"`
		RuleSet router.RuleSet `json:"ruleSet"`
	}
	if err := decodeBody(r, &body); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if body.Tag == "" {
		response.BadRequest(w, "tag is required")
		return
	}
	if err := h.svc.UpdateRuleSet(r.Context(), body.Tag, body.RuleSet); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

// DeleteRuleSet removes the ruleset identified by tag.
//
//	@Summary		Delete singbox-router ruleset
//	@Description	Removes the ruleset identified by tag. Refuses if any rule references it; pass force=true to remove this rule_set tag from referencing route and DNS rules.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterRuleSetDeleteRequest	true	"Tag + optional force flag"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		409		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/rulesets/delete [post]
func (h *SingboxRouterHandler) DeleteRuleSet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var body struct {
		Tag   string `json:"tag"`
		Force bool   `json:"force"`
	}
	if err := decodeBody(r, &body); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.DeleteRuleSet(r.Context(), body.Tag, body.Force); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

// DatRuleSetURL returns the local tokenized URL that sing-box can fetch directly.
//
//	@Summary		Build dat→SRS rule-set URL
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Param			kind	query	string	true	"geosite or geoip"
//	@Param			tag		query	[]string	true	"Geo tag(s)"
//	@Success		200	{object}	SingboxRouterDatRuleSetURLResponse
//	@Failure		400	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/singbox/router/rulesets/dat-url [get]
func (h *SingboxRouterHandler) DatRuleSetURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	kind := r.URL.Query().Get("kind")
	tags := nonEmptyQueryValues(r.URL.Query()["tag"])
	if (kind != "geosite" && kind != "geoip") || len(tags) == 0 {
		response.BadRequest(w, "kind must be geosite or geoip, tag is required")
		return
	}
	u, err := h.svc.DatRuleSetURL(r.Context(), kind, tags)
	if err != nil {
		h.handleErr(w, "dat-url", err)
		return
	}
	response.Success(w, SingboxRouterDatRuleSetURLData{URL: u})
}

// DatRuleSetSRS serves a compiled .srs artifact for sing-box. It is intentionally
// not protected by session cookies because sing-box fetches it as a plain remote
// rule_set URL; access is controlled by the token in the URL.
func (h *SingboxRouterHandler) DatRuleSetSRS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	kind := r.URL.Query().Get("kind")
	tags := nonEmptyQueryValues(r.URL.Query()["tag"])
	if (kind != "geosite" && kind != "geoip") || len(tags) == 0 {
		response.BadRequest(w, "kind must be geosite or geoip, tag is required")
		return
	}
	p, err := h.svc.DatRuleSetFile(
		r.Context(),
		kind,
		tags,
		r.URL.Query().Get("token"),
	)
	if err != nil {
		if errors.Is(err, router.ErrDatRuleSetForbidden) {
			response.ErrorWithStatus(w, http.StatusForbidden, "invalid dat rule-set token", "FORBIDDEN")
			return
		}
		h.handleErr(w, "dat-srs", err)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="rule-set.srs"`)
	http.ServeFile(w, r, p)
}

func nonEmptyQueryValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

// SetRouteFinal updates route.final.
//
//	@Summary		Set route.final outbound
//	@Description	Updates the route.final fallback outbound. Use "direct" for default sing-box direct, or the tag of any existing outbound (composite, AWG, sing-box tunnel).
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterRouteFinalRequest	true	"New final outbound tag"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		405		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/route/final [post]
func (h *SingboxRouterHandler) SetRouteFinal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var req SingboxRouterRouteFinalRequest
	if err := decodeBody(r, &req); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.SetRouteFinal(r.Context(), req.Final); err != nil {
		h.handleErr(w, "route-final", err)
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

// ListOutbounds returns all composite outbounds.
//
//	@Summary		List singbox-router outbounds
//	@Description	Returns all composite outbounds (sing-box selectors/urltests over multiple base outbounds).
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	SingboxRouterOutboundsListResponse
//	@Failure		405	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/singbox/router/outbounds/list [get]
func (h *SingboxRouterHandler) ListOutbounds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	o, err := h.svc.ListCompositeOutbounds(r.Context())
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, o)
}

// AddOutbound creates a new composite outbound.
//
//	@Summary		Add singbox-router outbound
//	@Description	Creates a new composite outbound. The base outbounds it references must already exist.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterOutboundDTO	true	"Composite outbound payload"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/outbounds/add [post]
func (h *SingboxRouterHandler) AddOutbound(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var o router.Outbound
	if err := decodeBody(r, &o); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.AddCompositeOutbound(r.Context(), o); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

// UpdateOutbound replaces the composite outbound identified by tag.
//
//	@Summary		Update singbox-router outbound
//	@Description	Replaces the composite outbound identified by tag with the provided one.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterOutboundUpdateRequest	true	"Tag + replacement outbound"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/outbounds/update [post]
func (h *SingboxRouterHandler) UpdateOutbound(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var body struct {
		Tag      string          `json:"tag"`
		Outbound router.Outbound `json:"outbound"`
	}
	if err := decodeBody(r, &body); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.UpdateCompositeOutbound(r.Context(), body.Tag, body.Outbound); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

// DeleteOutbound removes the composite outbound identified by tag.
//
//	@Summary		Delete singbox-router outbound
//	@Description	Removes the composite outbound identified by tag. Refuses if any rule references it; pass force=true to override.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterOutboundDeleteRequest	true	"Tag + optional force flag"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		409		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/outbounds/delete [post]
func (h *SingboxRouterHandler) DeleteOutbound(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var body struct {
		Tag   string `json:"tag"`
		Force bool   `json:"force"`
	}
	if err := decodeBody(r, &body); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := tunnelservice.CheckOutboundTagReferences(body.Tag, body.Tag, h.deviceProxyRefs, h.routerRefs); err != nil {
		var refErr tunnelservice.ErrTunnelReferenced
		if errors.As(err, &refErr) {
			WriteTunnelReferenced(w, refErr)
			return
		}
	}
	if err := h.svc.DeleteCompositeOutbound(r.Context(), body.Tag, body.Force); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

// ListPresets returns the catalog of built-in singbox-router presets.
//
//	@Summary		List singbox-router presets
//	@Description	Returns the catalog of built-in presets the user can apply (each preset = a curated bundle of rules + rulesets).
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	SingboxRouterPresetsListResponse
//	@Failure		405	{object}	APIErrorEnvelope
//	@Router			/singbox/router/presets/list [get]
func (h *SingboxRouterHandler) ListPresets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	list, err := h.svc.ListPresets()
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, list)
}

// ApplyPreset materialises the named preset against the chosen outbound.
//
//	@Summary		Apply singbox-router preset
//	@Description	Materialises the preset (id) into rules + rulesets, routing matched traffic via the selected outbound. Existing rules with the same tag are overwritten.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterApplyPresetRequest	true	"Preset id + target outbound"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/presets/apply [post]
func (h *SingboxRouterHandler) ApplyPreset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var body struct {
		ID       string `json:"id"`
		Outbound string `json:"outbound"`
	}
	if err := decodeBody(r, &body); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.ApplyPreset(r.Context(), body.ID, body.Outbound); err != nil {
		h.handleErr(w, "request", err)
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

// ListWANInterfaces returns all router WAN interfaces for the
// WAN-binding picker. No up/down filtering — the UI shows every
// interface and the user picks.
//
//	@Summary		List WAN interfaces
//	@Description	Returns all router WAN interfaces (no up/down filtering) used by the WAN-binding picker in singbox-router settings. Always a JSON array, never null. The `name` field is the kernel system-name and is the value that should be persisted into `wanInterface`.
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	SingboxRouterWANInterfacesListResponse
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/singbox/router/wan-interfaces [get]
func (h *SingboxRouterHandler) ListWANInterfaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	ifaces, err := h.svc.ListWANInterfaces(r.Context())
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	if ifaces == nil {
		ifaces = []router.WANInterfaceInfo{}
	}
	response.Success(w, ifaces)
}

// ListBindableInterfaces returns interfaces a user can bind a direct outbound to.
//
//	@Summary		List bindable interfaces for direct outbounds
//	@Description	Returns router interfaces (minus our own and AWG/WG auto-covered) that a direct outbound can bind to. Fields id and priority are not populated for this endpoint (only name, label, up are meaningful).
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	SingboxRouterWANInterfacesListResponse
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/singbox/router/bindable-interfaces [get]
func (h *SingboxRouterHandler) ListBindableInterfaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	ifaces, err := h.svc.ListBindableInterfaces(r.Context())
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	if ifaces == nil {
		ifaces = []router.WANInterfaceInfo{}
	}
	response.Success(w, ifaces)
}

// ListIngressEligibleInterfaces returns interfaces eligible for sing-box ingress-scope.
//
//	@Summary		List ingress-eligible interfaces
//	@Description	Returns router interfaces eligible for sing-box ingress-scope (bindable minus WAN minus LAN bridges). Used by the ingress multiselect in singbox-router settings.
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	SingboxRouterWANInterfacesListResponse
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/singbox/router/ingress-eligible-interfaces [get]
func (h *SingboxRouterHandler) ListIngressEligibleInterfaces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	ifaces, err := h.svc.ListIngressEligibleInterfaces(r.Context())
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	if ifaces == nil {
		ifaces = []router.WANInterfaceInfo{}
	}
	response.Success(w, ifaces)
}

// PoliciesCollection routes by HTTP method:
//
//	GET  → ListPolicies (returns []router.PolicyInfo)
//	POST → CreatePolicy (body: {description}, returns router.PolicyInfo)
func (h *SingboxRouterHandler) PoliciesCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listPolicies(w, r)
	case http.MethodPost:
		h.createPolicy(w, r)
	default:
		response.MethodNotAllowed(w)
	}
}

// listPolicies returns all NDMS policies known to the singbox-router engine.
//
//	@Summary		List singbox-router policies
//	@Description	Returns all NDMS policies known to the singbox-router engine. Always a JSON array, never null.
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	SingboxRouterPoliciesListResponse
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/singbox/router/policies [get]
func (h *SingboxRouterHandler) listPolicies(w http.ResponseWriter, r *http.Request) {
	policies, err := h.svc.ListPolicies(r.Context())
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	if policies == nil {
		policies = []router.PolicyInfo{}
	}
	response.Success(w, policies)
}

// createPolicy creates a new NDMS policy with the given description.
//
//	@Summary		Create singbox-router policy
//	@Description	Creates a new NDMS policy with the given description. Returns the created policy.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterCreatePolicyRequest	true	"Policy description"
//	@Success		200		{object}	SingboxRouterPolicyResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/policies [post]
func (h *SingboxRouterHandler) createPolicy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Description string `json:"description"`
	}
	if err := decodeBody(r, &req); err != nil {
		response.BadRequest(w, "invalid body")
		return
	}
	policy, err := h.svc.CreatePolicy(r.Context(), req.Description)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, policy)
}

// ListPolicyDevices handles GET /api/singbox/router/policy-devices?name=X
//
//	@Summary		List singbox-router policy devices
//	@Description	Returns the LAN devices currently bound to the named policy. Always a JSON array, never null.
//	@Tags			singbox-router
//	@Produce		json
//	@Security		CookieAuth
//	@Param			name	query		string	true	"Policy name"
//	@Success		200		{object}	SingboxRouterPolicyDevicesListResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/policy-devices [get]
func (h *SingboxRouterHandler) ListPolicyDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	policyName := r.URL.Query().Get("name")
	if policyName == "" {
		response.Error(w, "missing name parameter", "MISSING_NAME")
		return
	}
	devices, err := h.svc.ListPolicyDevices(r.Context(), policyName)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	if devices == nil {
		devices = []router.PolicyDevice{}
	}
	response.Success(w, devices)
}

// BindDevice handles POST /api/singbox/router/policy-devices/bind
//
//	@Summary		Bind device to singbox-router policy
//	@Description	Binds the LAN device (MAC) to the named policy. Replaces any existing binding.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterBindDeviceRequest	true	"Device MAC + target policy name"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/policy-devices/bind [post]
func (h *SingboxRouterHandler) BindDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var req struct {
		MAC        string `json:"mac"`
		PolicyName string `json:"policyName"`
	}
	if err := decodeBody(r, &req); err != nil {
		response.BadRequest(w, "invalid body")
		return
	}
	if err := h.svc.BindDevice(r.Context(), req.MAC, req.PolicyName); err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

// UnbindDevice handles POST /api/singbox/router/policy-devices/unbind
//
//	@Summary		Unbind device from singbox-router policy
//	@Description	Removes any policy binding for the LAN device identified by MAC.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterUnbindDeviceRequest	true	"Device MAC"
//	@Success		200		{object}	OkResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/policy-devices/unbind [post]
func (h *SingboxRouterHandler) UnbindDevice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var req struct {
		MAC string `json:"mac"`
	}
	if err := decodeBody(r, &req); err != nil {
		response.BadRequest(w, "invalid body")
		return
	}
	if err := h.svc.UnbindDevice(r.Context(), req.MAC); err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

// SingboxRouterRouteFinalRequest is the body for POST /singbox/router/route/final.
func (h *SingboxRouterHandler) handleErr(w http.ResponseWriter, action string, err error) {
	h.log.Warn(action, "", err.Error())
	switch {
	case errors.Is(err, router.ErrNetfilterComponentMissing),
		errors.Is(err, router.ErrIPTablesModTProxyMissing):
		response.Error(w, err.Error(), "NETFILTER_MISSING")
	case errors.Is(err, router.ErrRuleSetReferenced),
		errors.Is(err, router.ErrOutboundReferenced),
		errors.Is(err, router.ErrRuleSetTagConflict),
		errors.Is(err, router.ErrOutboundTagConflict),
		errors.Is(err, router.ErrDNSServerTagConflict),
		errors.Is(err, router.ErrDNSServerReferenced):
		response.Error(w, err.Error(), "CONFLICT")
	case errors.Is(err, router.ErrRuleIndexOutOfRange),
		errors.Is(err, router.ErrDNSRuleIndexOutOfRange),
		errors.Is(err, router.ErrDNSServerNotFound),
		errors.Is(err, router.ErrRuleSetNotFound),
		errors.Is(err, router.ErrOutboundNotFound):
		response.Error(w, err.Error(), "NOT_FOUND")
	case errors.Is(err, router.ErrInvalidMatchers),
		errors.Is(err, router.ErrDNSInvalidServer):
		response.Error(w, err.Error(), "INVALID_MATCHERS")
	case errors.Is(err, router.ErrQoSClassesInvalid):
		// 400 with the detailed Russian message (DSCP range/duplicate/limit/
		// outbound) intact so the settings UI can surface it verbatim.
		response.Error(w, err.Error(), "QOS_CLASSES_INVALID")
	case errors.Is(err, router.ErrReservedInboundTag):
		// 400: user rules must not claim the reserved qos-* inbound namespace
		// (they'd be inert shadow rules — the managed slot merges first).
		response.Error(w, err.Error(), "RESERVED_INBOUND_TAG")
	default:
		response.InternalError(w, err.Error())
	}
}
