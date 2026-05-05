package subscription

import "encoding/json"

// BuildSelector emits a sing-box selector outbound JSON wrapping memberTags.
// defaultTag must be one of memberTags; if empty, the first member is used.
func BuildSelector(selectorTag string, memberTags []string, defaultTag string) json.RawMessage {
	if defaultTag == "" && len(memberTags) > 0 {
		defaultTag = memberTags[0]
	}
	out := map[string]any{
		"type":                        "selector",
		"tag":                         selectorTag,
		"outbounds":                   memberTags,
		"interrupt_exist_connections": false,
	}
	if defaultTag != "" {
		out["default"] = defaultTag
	}
	raw, _ := json.Marshal(out)
	return raw
}

// BuildMixedInbound emits the SOCKS5/HTTP listener that pairs with the
// selector. NDMS bridge picks up the listener as a Proxy interface.
func BuildMixedInbound(inboundTag string, listenPort uint16) json.RawMessage {
	out := map[string]any{
		"type":        "mixed",
		"tag":         inboundTag,
		"listen":      "127.0.0.1",
		"listen_port": listenPort,
	}
	raw, _ := json.Marshal(out)
	return raw
}

// BuildRouteRule emits the inbound→outbound route entry that ties the
// mixed inbound to the selector.
func BuildRouteRule(inboundTag, selectorTag string) json.RawMessage {
	out := map[string]any{
		"inbound":  inboundTag,
		"outbound": selectorTag,
	}
	raw, _ := json.Marshal(out)
	return raw
}
