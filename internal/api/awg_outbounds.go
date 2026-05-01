// internal/api/awg_outbounds.go
package api

import (
	"context"
	"net/http"

	"github.com/hoaxisr/awg-manager/internal/response"
	"github.com/hoaxisr/awg-manager/internal/singbox/awgoutbounds"
)

// AWGOutboundsService is the narrow contract this handler needs.
// Implemented by awgoutbounds.Service.
type AWGOutboundsService interface {
	ListTags(ctx context.Context) ([]awgoutbounds.TagInfo, error)
}

// AWGOutboundTagDTO mirrors awgoutbounds.TagInfo for OpenAPI exposure.
type AWGOutboundTagDTO struct {
	Tag   string `json:"tag" example:"awg-vpn0"`
	Label string `json:"label" example:"My VPN"`
	Kind  string `json:"kind" example:"managed"`
	Iface string `json:"iface" example:"t2s0"`
}

// AWGOutboundsHandler exposes the catalog of AWG-direct outbound tags
// for the frontend (singbox-router rule editor outbound dropdown).
type AWGOutboundsHandler struct {
	svc AWGOutboundsService
}

func NewAWGOutboundsHandler(svc AWGOutboundsService) *AWGOutboundsHandler {
	return &AWGOutboundsHandler{svc: svc}
}

// ServeHTTP returns the catalog of AWG-direct outbound tags.
//
//	@Summary		List AWG outbound tags
//	@Description	Returns the catalog of AWG-direct outbound tags currently exposed to sing-box (one per managed/system AWG tunnel). Used by the singbox-router rule editor outbound dropdown.
//	@Tags			singbox
//	@Produce		json
//	@Security		CookieAuth
//	@Success		200	{object}	AWGOutboundTagsResponse
//	@Failure		405	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/singbox/awg-outbounds/tags [get]
func (h *AWGOutboundsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	tags, err := h.svc.ListTags(r.Context())
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	if tags == nil {
		tags = []awgoutbounds.TagInfo{}
	}
	// Wrapped in the standard {success, data} envelope so the frontend
	// request<T>() helper (which always expects the envelope) returns
	// the array correctly. Previously json-encoded raw, which made
	// request<T>'s data.data resolve to undefined and silently emptied
	// the AWG tunnels group in the rule-editor outbound dropdown.
	response.Success(w, tags)
}

// AWGOutboundTagsResponse is the envelope shape for /singbox/awg-outbounds/tags.
type AWGOutboundTagsResponse struct {
	Success bool                `json:"success" example:"true"`
	Data    []AWGOutboundTagDTO `json:"data"`
}
