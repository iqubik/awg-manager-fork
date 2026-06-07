package api

// AmneziaPremiumLoginRequest is the body for POST /amnezia-premium/login.
type AmneziaPremiumLoginRequest struct {
	VPNKey      string `json:"vpnKey" example:"vpn://..."`
	VPNKeySnake string `json:"vpn_key,omitempty" example:"vpn://..."`
	Remember    *bool  `json:"remember,omitempty" example:"true"`
}

// AmneziaPremiumLoginData is returned after a successful portal login.
type AmneziaPremiumLoginData struct {
	Sid string `json:"sid" example:"v_sid_cookie_value"`
}

// AmneziaPremiumLoginResponse is the typed success envelope for Premium login.
type AmneziaPremiumLoginResponse struct {
	Success bool                    `json:"success" example:"true"`
	Data    AmneziaPremiumLoginData `json:"data"`
}

// AmneziaPremiumAccountInfoRequest is the body for POST /amnezia-premium/account-info.
type AmneziaPremiumAccountInfoRequest struct {
	Sid string `json:"sid" example:"v_sid_cookie_value"`
}

// AmneziaPremiumAccountInfoResponse is a typed envelope around the proxied CP account payload.
type AmneziaPremiumAccountInfoResponse struct {
	Success bool           `json:"success" example:"true"`
	Data    map[string]any `json:"data" swaggertype:"object"`
}

// AmneziaPremiumDownloadConfigRequest is the body for POST /amnezia-premium/download-config.
type AmneziaPremiumDownloadConfigRequest struct {
	Sid         string `json:"sid" example:"v_sid_cookie_value"`
	CountryCode string `json:"countryCode" example:"nl"`
}

// AmneziaPremiumDownloadConfigData contains the downloaded WireGuard/AmneziaWG config.
type AmneziaPremiumDownloadConfigData struct {
	Config string `json:"config" example:"[Interface]\nPrivateKey = ..."`
}

// AmneziaPremiumDownloadConfigResponse is the typed success envelope for config downloads.
type AmneziaPremiumDownloadConfigResponse struct {
	Success bool                             `json:"success" example:"true"`
	Data    AmneziaPremiumDownloadConfigData `json:"data"`
}
