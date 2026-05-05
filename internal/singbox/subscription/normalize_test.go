package subscription

import (
	"encoding/base64"
	"testing"
)

func TestNormalize_PlainText(t *testing.T) {
	body := []byte("vless://a@b:1\ntrojan://c@d:2\n#comment\n\n")
	lines := NormalizeBody(body, "text/plain")
	if len(lines) != 2 {
		t.Errorf("lines=%v, expected 2 (vless + trojan; comment skipped)", lines)
	}
}

func TestNormalize_SingleBase64(t *testing.T) {
	plain := "vless://a@b:1\ntrojan://c@d:2\n"
	b64 := base64.StdEncoding.EncodeToString([]byte(plain))
	lines := NormalizeBody([]byte(b64), "text/plain")
	if len(lines) != 2 || lines[0] != "vless://a@b:1" {
		t.Errorf("lines=%v", lines)
	}
}

func TestNormalize_DoubleBase64(t *testing.T) {
	inner := "vless://a@b:1\n"
	mid := base64.StdEncoding.EncodeToString([]byte(inner))
	outer := base64.StdEncoding.EncodeToString([]byte(mid))
	lines := NormalizeBody([]byte(outer), "text/plain")
	if len(lines) != 1 || lines[0] != "vless://a@b:1" {
		t.Errorf("lines=%v", lines)
	}
}

func TestNormalize_HTMLAnchorExtraction(t *testing.T) {
	body := []byte(`<html><body><a href="vless://abc@host.example.com:443?security=tls#tag">link</a></body></html>`)
	lines := NormalizeBody(body, "text/html")
	if len(lines) != 1 {
		t.Errorf("lines=%v", lines)
	}
}

func TestNormalize_MixedLineEndings(t *testing.T) {
	body := []byte("vless://a@b:1\r\ntrojan://c@d:2\rss://e@f:3\n")
	lines := NormalizeBody(body, "text/plain")
	if len(lines) < 2 {
		t.Errorf("lines=%v want >=2", lines)
	}
}
