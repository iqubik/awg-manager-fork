package api

import (
	"encoding/json"
	"net/http"

	tunnelservice "github.com/hoaxisr/awg-manager/internal/tunnel/service"
)

// WriteTunnelReferenced responds with HTTP 409 and the structured payload
// the frontend uses for TunnelReferencedModal.
func WriteTunnelReferenced(w http.ResponseWriter, err tunnelservice.ErrTunnelReferenced) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(TunnelReferencedResponse{
		Error: "tunnel_referenced",
		Details: TunnelReferencedDetails{
			TunnelID:    err.TunnelID,
			DeviceProxy: err.DeviceProxy,
			RouterRules: err.RouterRules,
			RouterOther: err.RouterOther,
		},
	})
}
