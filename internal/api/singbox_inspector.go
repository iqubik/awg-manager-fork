package api

import (
	"net/http"

	"github.com/hoaxisr/awg-manager/internal/response"
	"github.com/hoaxisr/awg-manager/internal/singbox/router"
)

// ── Inspector DTOs ───────────────────────────────────────────────

// SingboxRouterInspectRequest is the body for POST /singbox/router/inspect.
type SingboxRouterInspectRequest struct {
	Domain   string `json:"domain" example:"google.com"`
	Port     int    `json:"port,omitempty" example:"443"`
	Protocol string `json:"protocol,omitempty" example:"tcp"`
}

// SingboxRouterInspectMatchDTO mirrors router.RuleMatchResult.
type SingboxRouterInspectMatchDTO struct {
	Index      int      `json:"index" example:"0"`
	Matched    bool     `json:"matched" example:"true"`
	Action     string   `json:"action" example:"route"`
	Outbound   string   `json:"outbound,omitempty" example:"vpn"`
	Conditions []string `json:"conditions,omitempty"`
	Reason     string   `json:"reason,omitempty" example:"совпало по: domain_suffix"`
}

// SingboxRouterInspectData mirrors router.InspectResult.
type SingboxRouterInspectData struct {
	Input       string                         `json:"input" example:"google.com"`
	InputType   string                         `json:"inputType" example:"domain"`
	Matches     []SingboxRouterInspectMatchDTO `json:"matches"`
	Destination string                         `json:"destination" example:"vpn"`
	MatchedRule int                            `json:"matchedRule" example:"0"`
	Final       string                         `json:"final" example:"direct"`
	Note        string                         `json:"note,omitempty"`
}

// SingboxRouterInspectResponse is the envelope for POST /singbox/router/inspect.
type SingboxRouterInspectResponse struct {
	Success bool                     `json:"success" example:"true"`
	Data    SingboxRouterInspectData `json:"data"`
}

// Inspect simulates which router rule would match the given domain/IP.
//
//	@Summary		Inspect router routing decision
//	@Description	Simulates which router rule would match the given domain or IP, returning the would-be outbound. Pure simulator — does not invoke sing-box. rule_set matchers are NOT evaluated in this version.
//	@Tags			singbox-router
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			body	body		SingboxRouterInspectRequest	true	"Domain or IP to test, plus optional port/protocol"
//	@Success		200		{object}	SingboxRouterInspectResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/singbox/router/inspect [post]
func (h *SingboxRouterHandler) Inspect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var req SingboxRouterInspectRequest
	if err := decodeBody(r, &req); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if req.Domain == "" {
		response.Error(w, "domain обязателен", "MISSING_DOMAIN")
		return
	}
	res, err := h.svc.Inspect(r.Context(), router.InspectInput{
		Domain:   req.Domain,
		Port:     req.Port,
		Protocol: req.Protocol,
	})
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, res)
}
