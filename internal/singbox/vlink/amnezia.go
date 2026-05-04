package vlink

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrAmneziaUnsupportedProtocol is returned when the inner Xray config in a
// vpn:// link uses a protocol other than vless (e.g. trojan, vmess). The
// caller can switch on this to decide between hard-fail and skip-with-count.
type ErrAmneziaUnsupportedProtocol struct {
	Protocol string
}

func (e *ErrAmneziaUnsupportedProtocol) Error() string {
	return fmt.Sprintf("amnezia: unsupported inner protocol %q", e.Protocol)
}

type xrayConfig struct {
	Outbounds []xrayOutbound `json:"outbounds"`
}

type xrayOutbound struct {
	Protocol       string          `json:"protocol"`
	Settings       json.RawMessage `json:"settings"`
	StreamSettings json.RawMessage `json:"streamSettings"`
	Tag            string          `json:"tag"`
}

func parseAmnezia(input string) (*ParsedOutbound, error) {
	const prefix = "vpn://"
	if !strings.HasPrefix(strings.ToLower(input), prefix) {
		return nil, errors.New("amnezia: missing vpn:// prefix")
	}
	tag := ""
	body := input[len(prefix):]
	if hash := strings.Index(body, "#"); hash >= 0 {
		tag = body[hash+1:]
		body = body[:hash]
	}
	decoded, err := DecodeBase64Url(body)
	if err != nil {
		return nil, fmt.Errorf("amnezia: base64: %w", err)
	}
	var cfg xrayConfig
	if err := json.Unmarshal(decoded, &cfg); err != nil {
		return nil, fmt.Errorf("amnezia: json: %w", err)
	}
	for _, ob := range cfg.Outbounds {
		switch strings.ToLower(ob.Protocol) {
		case "freedom", "blackhole", "":
			continue
		case "vless":
			return amneziaVlessToOutbound(ob, tag)
		default:
			return nil, &ErrAmneziaUnsupportedProtocol{Protocol: ob.Protocol}
		}
	}
	return nil, errors.New("amnezia: no usable outbound found")
}

func amneziaVlessToOutbound(ob xrayOutbound, tag string) (*ParsedOutbound, error) {
	var settings struct {
		VNext []struct {
			Address string `json:"address"`
			Port    int    `json:"port"`
			Users   []struct {
				ID         string `json:"id"`
				Flow       string `json:"flow"`
				Encryption string `json:"encryption"`
			} `json:"users"`
		} `json:"vnext"`
	}
	if err := json.Unmarshal(ob.Settings, &settings); err != nil {
		return nil, fmt.Errorf("amnezia.vless: settings: %w", err)
	}
	if len(settings.VNext) == 0 || len(settings.VNext[0].Users) == 0 {
		return nil, errors.New("amnezia.vless: empty vnext or users")
	}
	v := settings.VNext[0]
	user := v.Users[0]

	var stream struct {
		Network         string `json:"network"`
		Security        string `json:"security"`
		TLSSettings     struct {
			ServerName  string   `json:"serverName"`
			ALPN        []string `json:"alpn"`
			Fingerprint string   `json:"fingerprint"`
		} `json:"tlsSettings"`
		RealitySettings struct {
			ServerName  string `json:"serverName"`
			PublicKey   string `json:"publicKey"`
			ShortID     string `json:"shortId"`
			Fingerprint string `json:"fingerprint"`
		} `json:"realitySettings"`
		GRPCSettings struct {
			ServiceName string `json:"serviceName"`
		} `json:"grpcSettings"`
	}
	json.Unmarshal(ob.StreamSettings, &stream)

	out := map[string]any{
		"type":        "vless",
		"server":      v.Address,
		"server_port": v.Port,
		"uuid":        user.ID,
	}
	if user.Flow != "" {
		out["flow"] = normalizeFlow(user.Flow)
	}

	switch strings.ToLower(stream.Network) {
	case "grpc":
		out["transport"] = map[string]any{
			"type":         "grpc",
			"service_name": stream.GRPCSettings.ServiceName,
		}
	}

	switch strings.ToLower(stream.Security) {
	case "tls":
		tls := map[string]any{"enabled": true, "server_name": stream.TLSSettings.ServerName}
		if len(stream.TLSSettings.ALPN) > 0 {
			tls["alpn"] = stream.TLSSettings.ALPN
		}
		if stream.TLSSettings.Fingerprint != "" {
			tls["utls"] = map[string]any{"enabled": true, "fingerprint": stream.TLSSettings.Fingerprint}
		}
		out["tls"] = tls
	case "reality":
		tls := map[string]any{"enabled": true, "server_name": stream.RealitySettings.ServerName}
		if stream.RealitySettings.Fingerprint != "" {
			tls["utls"] = map[string]any{"enabled": true, "fingerprint": stream.RealitySettings.Fingerprint}
		}
		tls["reality"] = map[string]any{
			"enabled":    true,
			"public_key": stream.RealitySettings.PublicKey,
			"short_id":   stream.RealitySettings.ShortID,
		}
		out["tls"] = tls
	}

	if tag == "" && ob.Tag != "" {
		tag = ob.Tag
	}
	if tag == "" {
		tag = fmt.Sprintf("amnezia-vless-%s-%d", v.Address, v.Port)
	}
	out["tag"] = tag

	raw, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return &ParsedOutbound{
		Tag:      tag,
		Protocol: "vless",
		Server:   v.Address,
		Port:     uint16(v.Port),
		Outbound: raw,
	}, nil
}
