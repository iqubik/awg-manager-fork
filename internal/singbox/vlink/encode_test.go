package vlink

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEncodeOutbound_RoundTrip(t *testing.T) {
	links := []string{
		"vless://3a3b1c2e-9999-4321-aaaa-1234567890ab@example.com:443?security=reality&type=tcp&pbk=PBK&sid=ab12cd34&fp=chrome&sni=foo.com&flow=xtls-rprx-vision#myname",
		"vless://uuid-here-1111-2222-333333333333@example.com:443?type=ws&security=tls&path=%2Fabc%3Fed%3D2048&host=cdn.example.com&sni=foo.com&alpn=h2%2Chttp%2F1.1#tag",
		"trojan://mypass@example.com:443?security=tls&sni=h.example.com&alpn=h2#srv",
		"ss://aes-256-gcm:mypass@example.com:8388#srv",
		"hysteria2://mypass@example.com:8443?sni=h.example.com&alpn=h3#srv",
		"hy2://p@example.com:8443?sni=h&insecure=1",
		"naive+https://user:pass@example.com:443#n",
	}
	for _, link := range links {
		t.Run(link[:min(40, len(link))], func(t *testing.T) {
			parsed, err := ParseLinkMany(link)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if len(parsed) != 1 {
				t.Fatalf("expected 1 outbound, got %d", len(parsed))
			}
			encoded, err := EncodeOutbound(parsed[0].Outbound, parsed[0].Label)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			reparsed, err := ParseLinkMany(encoded)
			if err != nil {
				t.Fatalf("reparse encoded %q: %v", encoded, err)
			}
			if len(reparsed) != 1 {
				t.Fatalf("reparse count=%d", len(reparsed))
			}
			var wantOb, gotOb map[string]any
			json.Unmarshal(parsed[0].Outbound, &wantOb)
			json.Unmarshal(reparsed[0].Outbound, &gotOb)
			assertEncodeRoundTrip(t, wantOb, gotOb)
			if parsed[0].Label != "" && reparsed[0].Label != parsed[0].Label {
				t.Fatalf("label: want %q got %q", parsed[0].Label, reparsed[0].Label)
			}
		})
	}
}

func TestEncodeOutbound_VlessRealityWithoutTLSFlag(t *testing.T) {
	raw := []byte(`{"type":"vless","server":"h.com","server_port":443,"uuid":"3a3b1c2e-9999-4321-aaaa-1234567890ab","tls":{"reality":{"enabled":true,"public_key":"PK","short_id":"ab12"},"server_name":"h.com","utls":{"fingerprint":"chrome"}}}`)
	link, err := EncodeOutbound(raw, "t")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(link, "security=reality") {
		t.Fatalf("missing reality security: %q", link)
	}
	if !strings.Contains(link, "pbk=PK") || !strings.Contains(link, "sid=ab12") {
		t.Fatalf("missing reality params: %q", link)
	}
}

func TestEncodeOutbound_MieruSimple_MatchesSampleShape(t *testing.T) {
	parsed := ParseBatch([]string{mieruSimpleSample})
	if len(parsed.Errors) != 0 {
		t.Fatalf("errors: %+v", parsed.Errors)
	}
	encoded, err := EncodeOutbound(parsed.Outbounds[0].Outbound, parsed.Outbounds[0].Label)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(encoded, "mierus://") {
		t.Fatalf("expected mierus scheme, got %q", encoded)
	}
	if !strings.Contains(encoded, "profile=default") {
		t.Fatalf("missing profile: %q", encoded)
	}
}

func assertEncodeRoundTrip(t *testing.T, want, got map[string]any) {
	t.Helper()
	for _, k := range []string{"type", "server", "server_port", "uuid", "password", "method", "username", "flow"} {
		if w, ok := want[k]; ok && w != nil && w != "" {
			if got[k] != w {
				t.Fatalf("%s: want %v got %v", k, w, got[k])
			}
		}
	}
	wantTLS, _ := want["tls"].(map[string]any)
	gotTLS, _ := got["tls"].(map[string]any)
	if wantTLS != nil {
		if gotTLS == nil {
			t.Fatalf("tls: want %v got nil", wantTLS)
		}
		if sni, _ := wantTLS["server_name"].(string); sni != "" {
			if gotSNI, _ := gotTLS["server_name"].(string); gotSNI != sni {
				t.Fatalf("tls.server_name: want %q got %q", sni, gotSNI)
			}
		}
		wantReality, _ := wantTLS["reality"].(map[string]any)
		gotReality, _ := gotTLS["reality"].(map[string]any)
		if wantReality != nil && wantReality["enabled"] == true {
			if gotReality == nil || gotReality["enabled"] != true {
				t.Fatalf("tls.reality: want enabled got %v", gotReality)
			}
			for _, k := range []string{"public_key", "short_id"} {
				if v, _ := wantReality[k].(string); v != "" {
					if gv, _ := gotReality[k].(string); gv != v {
						t.Fatalf("tls.reality.%s: want %q got %q", k, v, gv)
					}
				}
			}
		}
	}
	wantTransport, _ := want["transport"].(map[string]any)
	gotTransport, _ := got["transport"].(map[string]any)
	if wantTransport != nil {
		if gotTransport == nil {
			t.Fatalf("transport: want %v got nil", wantTransport)
		}
		if typ, _ := wantTransport["type"].(string); typ != "" && typ != "tcp" {
			if gotTyp, _ := gotTransport["type"].(string); gotTyp != typ {
				t.Fatalf("transport.type: want %q got %q", typ, gotTyp)
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
