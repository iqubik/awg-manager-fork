package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hoaxisr/awg-manager/internal/response"
	"github.com/hoaxisr/awg-manager/internal/singbox/subscription"
)

// SubscriptionHandler exposes /api/singbox/subscriptions/* endpoints.
type SubscriptionHandler struct {
	svc *subscription.Service
}

func NewSubscriptionHandler(svc *subscription.Service) *SubscriptionHandler {
	return &SubscriptionHandler{svc: svc}
}

// SubscriptionMemberDTO carries per-member parsed metadata for the UI.
type SubscriptionMemberDTO struct {
	Tag       string `json:"tag" example:"sub-abc12345-aabbccdd"`
	Protocol  string `json:"protocol" example:"vless"`
	Server    string `json:"server" example:"de01.example.com"`
	Port      int    `json:"port" example:"443"`
	Transport string `json:"transport,omitempty" example:"ws"`
	Security  string `json:"security,omitempty" example:"tls"`
}

// SubscriptionDTO mirrors subscription.Subscription for OpenAPI exposure.
type SubscriptionDTO struct {
	ID             string                  `json:"id" example:"abc123"`
	Label          string                  `json:"label" example:"Provider X"`
	URL            string                  `json:"url" example:"https://prov.example/sub/a"`
	Headers        []SubscriptionHeader    `json:"headers"`
	RefreshHours   int                     `json:"refreshHours" example:"24"`
	LastFetched    string                  `json:"lastFetched"`
	LastError      string                  `json:"lastError,omitempty"`
	SelectorTag    string                  `json:"selectorTag" example:"sub-abc12345"`
	InboundTag     string                  `json:"inboundTag" example:"sub-abc12345-in"`
	ListenPort     int                     `json:"listenPort" example:"11080"`
	MemberTags     []string                `json:"memberTags"`
	Members        []SubscriptionMemberDTO `json:"members"`
	OrphanTags     []string                `json:"orphanTags"`
	ActiveMember   string                  `json:"activeMember" example:"sub-abc-aaaa"`
	Enabled        bool                    `json:"enabled"`
	IsDefaultRoute bool                    `json:"isDefaultRoute"`
}

// SubscriptionHeader is a single custom HTTP header for the fetch request.
type SubscriptionHeader struct {
	Name  string `json:"name" example:"User-Agent"`
	Value string `json:"value" example:"Happ/4.6.0"`
}

// SubscriptionListResponse is the envelope for GET /api/singbox/subscriptions.
type SubscriptionListResponse struct {
	Success bool              `json:"success"`
	Data    []SubscriptionDTO `json:"data"`
}

// SubscriptionResponse is the envelope for single-subscription responses.
type SubscriptionResponse struct {
	Success bool            `json:"success"`
	Data    SubscriptionDTO `json:"data"`
}

// CreateSubscriptionRequest is the body for POST /api/singbox/subscriptions/create.
type CreateSubscriptionRequest struct {
	Label        string               `json:"label"`
	URL          string               `json:"url"`
	Headers      []SubscriptionHeader `json:"headers"`
	RefreshHours int                  `json:"refreshHours"`
	Enabled      bool                 `json:"enabled"`
}

// UpdateSubscriptionRequest is the body for PUT /api/singbox/subscriptions/update.
// All fields are optional; absent fields leave the stored value unchanged.
type UpdateSubscriptionRequest struct {
	Label        *string               `json:"label,omitempty"`
	URL          *string               `json:"url,omitempty"`
	Headers      *[]SubscriptionHeader `json:"headers,omitempty"`
	RefreshHours *int                  `json:"refreshHours,omitempty"`
	Enabled      *bool                 `json:"enabled,omitempty"`
}

// ActiveMemberRequest is the body for POST /api/singbox/subscriptions/active-member.
type ActiveMemberRequest struct {
	MemberTag string `json:"memberTag"`
}

// DefaultRouteRequest is the body for POST /api/singbox/subscriptions/default-route.
type DefaultRouteRequest struct {
	Enabled bool `json:"enabled"`
}

// toDTO converts a domain Subscription to its API representation.
func toSubscriptionDTO(s subscription.Subscription) SubscriptionDTO {
	hh := make([]SubscriptionHeader, len(s.Headers))
	for i, h := range s.Headers {
		hh[i] = SubscriptionHeader{Name: h.Name, Value: h.Value}
	}
	last := ""
	if !s.LastFetched.IsZero() {
		last = s.LastFetched.Format("2006-01-02T15:04:05Z07:00")
	}
	memberTags := s.MemberTags
	if memberTags == nil {
		memberTags = []string{}
	}
	orphans := s.OrphanTags
	if orphans == nil {
		orphans = []string{}
	}
	memberDTOs := make([]SubscriptionMemberDTO, len(s.Members))
	for i, m := range s.Members {
		memberDTOs[i] = SubscriptionMemberDTO{
			Tag:       m.Tag,
			Protocol:  m.Protocol,
			Server:    m.Server,
			Port:      int(m.Port),
			Transport: m.Transport,
			Security:  m.Security,
		}
	}
	return SubscriptionDTO{
		ID:             s.ID,
		Label:          s.Label,
		URL:            s.URL,
		Headers:        hh,
		RefreshHours:   s.RefreshHours,
		LastFetched:    last,
		LastError:      s.LastError,
		SelectorTag:    s.SelectorTag,
		InboundTag:     s.InboundTag,
		ListenPort:     int(s.ListenPort),
		MemberTags:     memberTags,
		Members:        memberDTOs,
		OrphanTags:     orphans,
		ActiveMember:   s.ActiveMember,
		Enabled:        s.Enabled,
		IsDefaultRoute: s.IsDefaultRoute,
	}
}

func fromSubscriptionHeaders(in []SubscriptionHeader) []subscription.Header {
	out := make([]subscription.Header, len(in))
	for i, h := range in {
		out[i] = subscription.Header{Name: h.Name, Value: h.Value}
	}
	return out
}

// validateSubscriptionHeaders enforces spec limits: <=32 headers, name <=256, value <=2048.
func validateSubscriptionHeaders(hh []SubscriptionHeader) error {
	if len(hh) > 32 {
		return fmt.Errorf("too many headers (%d > 32)", len(hh))
	}
	for _, h := range hh {
		if len(h.Name) > 256 {
			return fmt.Errorf("header name too long: %d > 256", len(h.Name))
		}
		if len(h.Value) > 2048 {
			return fmt.Errorf("header value too long: %d > 2048", len(h.Value))
		}
	}
	return nil
}

// List handles GET /api/singbox/subscriptions
func (h *SubscriptionHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	all := []SubscriptionDTO{}
	for _, s := range h.svc.List() {
		all = append(all, toSubscriptionDTO(s))
	}
	response.Success(w, all)
}

// Create handles POST /api/singbox/subscriptions/create
func (h *SubscriptionHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var req CreateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorWithStatus(w, http.StatusBadRequest, "bad request body", "INVALID_JSON")
		return
	}
	if err := validateSubscriptionHeaders(req.Headers); err != nil {
		response.ErrorWithStatus(w, http.StatusBadRequest, err.Error(), "INVALID_HEADERS")
		return
	}
	in := subscription.CreateInput{
		Label:        req.Label,
		URL:          req.URL,
		Headers:      fromSubscriptionHeaders(req.Headers),
		RefreshHours: req.RefreshHours,
		Enabled:      req.Enabled,
	}
	sub, err := h.svc.Create(r.Context(), in)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, toSubscriptionDTO(*sub))
}

// Get handles GET /api/singbox/subscriptions/get?id=
func (h *SubscriptionHandler) Get(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		response.ErrorWithStatus(w, http.StatusBadRequest, "id required", "MISSING_ID")
		return
	}
	sub, err := h.svc.Get(id)
	if err != nil {
		response.ErrorWithStatus(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
		return
	}
	response.Success(w, toSubscriptionDTO(*sub))
}

// Update handles PUT /api/singbox/subscriptions/update?id=
func (h *SubscriptionHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		response.MethodNotAllowed(w)
		return
	}
	id := r.URL.Query().Get("id")
	var req UpdateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorWithStatus(w, http.StatusBadRequest, "bad request body", "INVALID_JSON")
		return
	}
	if req.Headers != nil {
		if err := validateSubscriptionHeaders(*req.Headers); err != nil {
			response.ErrorWithStatus(w, http.StatusBadRequest, err.Error(), "INVALID_HEADERS")
			return
		}
	}
	patch := subscription.UpdatePatch{
		Label:        req.Label,
		URL:          req.URL,
		RefreshHours: req.RefreshHours,
		Enabled:      req.Enabled,
	}
	if req.Headers != nil {
		hh := fromSubscriptionHeaders(*req.Headers)
		patch.Headers = &hh
	}
	sub, err := h.svc.Update(id, patch)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, toSubscriptionDTO(*sub))
}

// Delete handles DELETE /api/singbox/subscriptions/delete?id=&cascade=
func (h *SubscriptionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		response.MethodNotAllowed(w)
		return
	}
	id := r.URL.Query().Get("id")
	cascade, _ := strconv.ParseBool(r.URL.Query().Get("cascade"))
	if err := h.svc.Delete(r.Context(), id, cascade); err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, struct {
		OK bool `json:"ok"`
	}{true})
}

// Refresh handles POST /api/singbox/subscriptions/refresh?id=
func (h *SubscriptionHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	id := r.URL.Query().Get("id")
	res, err := h.svc.Refresh(r.Context(), id)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, res)
}

// ActiveMember handles POST /api/singbox/subscriptions/active-member?id=
func (h *SubscriptionHandler) ActiveMember(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	id := r.URL.Query().Get("id")
	var req ActiveMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorWithStatus(w, http.StatusBadRequest, "bad request body", "INVALID_JSON")
		return
	}
	if err := h.svc.SetActiveMember(r.Context(), id, req.MemberTag); err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, struct {
		OK bool `json:"ok"`
	}{true})
}

// DefaultRoute handles POST /api/singbox/subscriptions/default-route?id=
func (h *SubscriptionHandler) DefaultRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	id := r.URL.Query().Get("id")
	var req DefaultRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.ErrorWithStatus(w, http.StatusBadRequest, "bad request body", "INVALID_JSON")
		return
	}
	if err := h.svc.SetDefaultRoute(r.Context(), id, req.Enabled); err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, struct {
		OK bool `json:"ok"`
	}{true})
}

// OrphansDelete handles POST /api/singbox/subscriptions/orphans/delete?id=
func (h *SubscriptionHandler) OrphansDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	id := r.URL.Query().Get("id")
	if err := h.svc.DeleteOrphans(r.Context(), id); err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, struct {
		OK bool `json:"ok"`
	}{true})
}
