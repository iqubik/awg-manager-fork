package vlink

import (
	"encoding/json"
	"errors"
	"fmt"
)

// mapClashVless converts a Clash YAML "type: vless" proxy entry into a
// ParsedOutbound. Required fields: server, port, uuid. Optional: flow,
// network/transport, tls/reality blocks.
//
// Field reference: https://wiki.metacubex.one/en/config/proxies/vless/
func mapClashVless(p map[string]any) (*ParsedOutbound, error) {
	host := asString(p["server"])
	if host == "" {
		return nil, errors.New("clash vless: missing server")
	}
	portN, ok := asInt(p["port"])
	if !ok || portN <= 0 || portN > 65535 {
		return nil, errors.New("clash vless: missing or invalid port")
	}
	uuid := asString(p["uuid"])
	if uuid == "" {
		return nil, errors.New("clash vless: missing uuid")
	}

	q := clashFieldsToValues(p)
	stream, err := BuildStreamFromQuery(q, host)
	if err != nil {
		return nil, fmt.Errorf("clash vless: %w", err)
	}

	out := map[string]any{
		"type":        "vless",
		"server":      host,
		"server_port": portN,
		"uuid":        uuid,
	}
	if flow := asString(p["flow"]); flow != "" {
		out["flow"] = flow
	}
	stream.MergeIntoOutbound(out)

	tag := fmt.Sprintf("vless-%s-%d", host, portN)
	out["tag"] = tag

	raw, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return &ParsedOutbound{
		Tag:      tag,
		Protocol: "vless",
		Server:   host,
		Port:     uint16(portN),
		Outbound: raw,
		Label:    asString(p["name"]),
	}, nil
}
