package api

import (
	"net/http"

	"github.com/hoaxisr/awg-manager/internal/response"
	"github.com/hoaxisr/awg-manager/internal/singbox/dnsrewrite"
)

type DNSRewritesService interface {
	List() ([]dnsrewrite.DNSRewrite, error)
	Add(dnsrewrite.DNSRewrite) error
	Update(int, dnsrewrite.DNSRewrite) error
	Delete(int) error
	Move(from, to int) error
}

type DNSRewritesHandler struct {
	svc DNSRewritesService
}

func NewDNSRewritesHandler(svc DNSRewritesService) *DNSRewritesHandler {
	return &DNSRewritesHandler{svc: svc}
}

func (h *DNSRewritesHandler) List(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w)
		return
	}
	items, err := h.svc.List()
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	if items == nil {
		items = []dnsrewrite.DNSRewrite{}
	}
	response.Success(w, items)
}

func (h *DNSRewritesHandler) Add(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var rw dnsrewrite.DNSRewrite
	if err := decodeBody(r, &rw); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.Add(rw); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

func (h *DNSRewritesHandler) Update(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var req struct {
		Index   int                   `json:"index"`
		Rewrite dnsrewrite.DNSRewrite `json:"rewrite"`
	}
	if err := decodeBody(r, &req); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.Update(req.Index, req.Rewrite); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

func (h *DNSRewritesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var req struct {
		Index int `json:"index"`
	}
	if err := decodeBody(r, &req); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.Delete(req.Index); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}

func (h *DNSRewritesHandler) Move(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w)
		return
	}
	var req struct {
		From int `json:"from"`
		To   int `json:"to"`
	}
	if err := decodeBody(r, &req); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.svc.Move(req.From, req.To); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.Success(w, map[string]bool{"ok": true})
}
