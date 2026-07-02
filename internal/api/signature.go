package api

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/hoaxisr/awg-manager/internal/response"
	"github.com/hoaxisr/awg-manager/internal/signature"
)

// ── Response DTOs ────────────────────────────────────────────────

// SignaturePacketsDTO is the packets field in SignatureCaptureResult.
type SignaturePacketsDTO struct {
	I1 string `json:"i1" example:"0a1b2c3d"`
	I2 string `json:"i2" example:"4e5f6a7b"`
	I3 string `json:"i3" example:"8c9d0e1f"`
	I4 string `json:"i4" example:"2a3b4c5d"`
	I5 string `json:"i5" example:"6e7f8a9b"`
}

// SignatureCaptureData mirrors frontend SignatureCaptureResult.
type SignatureCaptureData struct {
	OK      bool                `json:"ok" example:"true"`
	Source  string              `json:"source" example:"Wireguard0"`
	Packets SignaturePacketsDTO `json:"packets"`
	Warning string              `json:"warning,omitempty" example:""`
}

// SignatureCaptureResponse is the envelope for GET /signature/capture.
type SignatureCaptureResponse struct {
	Success bool                 `json:"success" example:"true"`
	Data    SignatureCaptureData `json:"data"`
}

type SignatureGenerateRequest struct {
	Protocol string `json:"protocol" example:"quic_initial"`
	MTU      *int   `json:"mtu,omitempty" example:"1280"`
}

type SignatureGenerateData struct {
	OK       bool                `json:"ok" example:"true"`
	Source   string              `json:"source" example:"generated"`
	Protocol string              `json:"protocol" example:"quic_initial"`
	ByteSize int                 `json:"byteSize" example:"344"`
	Packets  SignaturePacketsDTO `json:"packets"`
	Warning  string              `json:"warning,omitempty" example:""`
}

type SignatureGenerateResponse struct {
	Success bool                  `json:"success" example:"true"`
	Data    SignatureGenerateData `json:"data"`
}

var validDomain = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*\.[a-zA-Z]{2,}$`)

type SignatureHandler struct{}

func NewSignatureHandler() *SignatureHandler {
	return &SignatureHandler{}
}

// Capture runs TLS certificate capture for a domain.
//
//	@Summary		Signature capture
//	@Tags			signature
//	@Produce		json
//	@Security		CookieAuth
//	@Param			domain	query	string	true	"Domain name"
//	@Success		200	{object}	SignatureCaptureResponse
//	@Failure		400	{object}	APIErrorEnvelope
//	@Failure		500	{object}	APIErrorEnvelope
//	@Router			/signature/capture [get]
func (h *SignatureHandler) Capture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}

	domain := r.URL.Query().Get("domain")
	if domain == "" {
		response.Error(w, "Укажите домен", "MISSING_DOMAIN")
		return
	}

	domain = signature.NormalizeDomain(domain)

	if !validDomain.MatchString(domain) {
		response.Error(w, "Некорректный домен", "INVALID_DOMAIN")
		return
	}

	result := signature.Capture(domain)

	if result.Source == "error" {
		response.ErrorWithStatus(w, http.StatusBadGateway, result.Warning, "CAPTURE_FAILED")
		return
	}

	response.Success(w, result)
}

// Generate builds synthetic signature packets for a mimicry protocol.
//
//	@Summary		Signature generate
//	@Tags			signature
//	@Accept			json
//	@Produce		json
//	@Security		CookieAuth
//	@Param			request	body		SignatureGenerateRequest	true	"Generation parameters"
//	@Success		200		{object}	SignatureGenerateResponse
//	@Failure		400		{object}	APIErrorEnvelope
//	@Failure		500		{object}	APIErrorEnvelope
//	@Router			/signature/generate [post]
func (h *SignatureHandler) Generate(w http.ResponseWriter, r *http.Request) {
	req, ok := parseJSON[SignatureGenerateRequest](w, r, http.MethodPost)
	if !ok {
		return
	}
	if req.Protocol == "" {
		response.Error(w, "Укажите protocol", "MISSING_PROTOCOL")
		return
	}
	mtu := 1280
	if req.MTU != nil {
		if *req.MTU <= 0 {
			response.Error(w, "Некорректный mtu", "INVALID_MTU")
			return
		}
		mtu = *req.MTU
	}

	result, err := signature.Generate(req.Protocol, mtu)
	if err != nil {
		switch {
		case strings.HasPrefix(err.Error(), "invalid protocol:"):
			response.Error(w, "Неподдерживаемый protocol", "INVALID_PROTOCOL")
		case strings.HasPrefix(err.Error(), "signature too large:"):
			response.Error(w, "Подпись слишком большая", "SIGNATURE_TOO_LARGE")
		default:
			response.InternalError(w, err.Error())
		}
		return
	}

	response.Success(w, result)
}
