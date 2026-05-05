// Package vlink: Clash / mihomo YAML subscription support.
//
// Entry points: IsClashYAML detects the format; ParseClashBody parses the
// body and returns a BatchResult identical in shape to ParseBatch. Per-
// protocol mappers live in clash_<protocol>.go (mirrors existing per-
// protocol layout for share-link parsers).
package vlink

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// scanLimit is how many bytes IsClashYAML inspects. A real Clash subscription
// has "proxies:" within the first few hundred bytes; 4 KB is a forgiving cap.
const scanLimit = 4 * 1024

// matches a top-level "proxies:" key — accepts block (proxies: + newline),
// inline ("proxies: []"), null marker ("proxies: null"), and any other
// permissive form. Tolerable false positives are documented above.
var proxiesHeaderRe = regexp.MustCompile(`(?m)^proxies:`)

// IsClashYAML reports whether body looks like a Clash/mihomo subscription
// (top-level "proxies:" key in valid YAML). Cheap: scans the first 4 KB only.
// False positives on bodies that happen to contain "proxies:" mid-document
// are tolerable because ParseClashBody will then parse and find no entries.
func IsClashYAML(body []byte) bool {
	if len(body) == 0 {
		return false
	}
	head := body
	if len(head) > scanLimit {
		head = head[:scanLimit]
	}
	// Reject obvious non-YAML preludes.
	trimmed := bytes.TrimSpace(head)
	if len(trimmed) == 0 {
		return false
	}
	if trimmed[0] == '<' {
		return false // HTML
	}
	return proxiesHeaderRe.Match(head)
}

// clashFieldsToValues collapses Clash YAML proxy fields into a synthetic
// url.Values. The resulting Values is fed to BuildStreamFromQuery, so the
// keys must match what stream.go expects: type, path, host, serviceName,
// security, sni, alpn, fp, insecure, pbk, sid.
//
// Reality is detected when reality-opts is present; otherwise tls:true →
// security=tls. Network is read from "network" (ws/grpc/http/h2/tcp).
func clashFieldsToValues(p map[string]any) url.Values {
	v := url.Values{}

	// Network / transport
	netRaw := strings.ToLower(asString(p["network"]))
	if netRaw == "" {
		netRaw = "tcp"
	}
	v.Set("type", netRaw)

	switch netRaw {
	case "ws":
		ws := nestedMap(p, "ws-opts")
		if path := asString(ws["path"]); path != "" {
			v.Set("path", path)
		}
		hdrs := nestedMap(ws, "headers")
		if host := asString(hdrs["Host"]); host != "" {
			v.Set("host", host)
		}
	case "grpc":
		gp := nestedMap(p, "grpc-opts")
		if name := asString(gp["grpc-service-name"]); name != "" {
			v.Set("serviceName", name)
		}
	case "http", "h2":
		hp := nestedMap(p, "http-opts")
		if path := asString(hp["path"]); path != "" {
			v.Set("path", path)
		}
		if host := asString(hp["host"]); host != "" {
			v.Set("host", host)
		}
	}

	// TLS / Reality
	reality := nestedMap(p, "reality-opts")
	switch {
	case len(reality) > 0:
		v.Set("security", "reality")
		if pk := asString(reality["public-key"]); pk != "" {
			v.Set("pbk", pk)
		}
		if sid := asString(reality["short-id"]); sid != "" {
			v.Set("sid", sid)
		}
	case asBool(p["tls"]):
		v.Set("security", "tls")
	}

	if sni := firstNonEmpty(asString(p["servername"]), asString(p["sni"])); sni != "" {
		v.Set("sni", sni)
	}
	if asBool(p["skip-cert-verify"]) {
		v.Set("insecure", "1")
	}
	if alpn := asStringSlice(p["alpn"]); len(alpn) > 0 {
		v.Set("alpn", strings.Join(alpn, ","))
	}
	if fp := asString(p["client-fingerprint"]); fp != "" {
		v.Set("fp", fp)
	}

	return v
}

// asString returns v as a string, with light coercion: bool/int/float become
// their %v form, nil becomes "".
func asString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// asInt extracts v as an int with light coercion. Booleans never coerce.
func asInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case string:
		s := strings.TrimSpace(x)
		if n, err := strconv.Atoi(s); err == nil {
			return n, true
		}
	}
	return 0, false
}

// asBool accepts bool, non-zero numerics, "true"/"yes"/"1".
func asBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case int:
		return x != 0
	case int64:
		return x != 0
	case float64:
		return x != 0
	case string:
		switch strings.ToLower(strings.TrimSpace(x)) {
		case "1", "true", "yes":
			return true
		}
	}
	return false
}

// asStringSlice normalises both YAML lists and scalar strings to []string.
// Anything else returns nil.
func asStringSlice(v any) []string {
	switch x := v.(type) {
	case []any:
		out := make([]string, 0, len(x))
		for _, e := range x {
			if s := asString(e); s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return append([]string(nil), x...)
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	}
	return nil
}

// nestedMap returns p[key] as map[string]any, or empty map if missing/wrong type.
func nestedMap(p map[string]any, key string) map[string]any {
	if p == nil {
		return map[string]any{}
	}
	if m, ok := p[key].(map[string]any); ok {
		return m
	}
	return map[string]any{}
}
