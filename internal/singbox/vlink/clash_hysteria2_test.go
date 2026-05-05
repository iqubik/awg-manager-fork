package vlink

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMapClashHysteria2_HappyPath(t *testing.T) {
	in := map[string]any{
		"name":          "Hy2-1",
		"type":          "hysteria2",
		"server":        "hy2.example.com",
		"port":          443,
		"password":      "hy2pass",
		"sni":           "sni.example.com",
		"obfs":          "salamander",
		"obfs-password": "obfs-secret",
		"up":            50,
		"down":          200,
	}
	got, err := mapClashHysteria2(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.Protocol != "hysteria2" {
		t.Errorf("Protocol=%q want hysteria2", got.Protocol)
	}
	var ob map[string]any
	if err := json.Unmarshal(got.Outbound, &ob); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ob["password"] != "hy2pass" {
		t.Errorf("password=%v", ob["password"])
	}
	if upN, ok := asInt(ob["up_mbps"]); !ok || upN != 50 {
		t.Errorf("up_mbps=%v want 50", ob["up_mbps"])
	}
	if downN, ok := asInt(ob["down_mbps"]); !ok || downN != 200 {
		t.Errorf("down_mbps=%v want 200", ob["down_mbps"])
	}
	obfs, _ := ob["obfs"].(map[string]any)
	if obfs == nil || obfs["type"] != "salamander" || obfs["password"] != "obfs-secret" {
		t.Errorf("obfs block wrong: %v", obfs)
	}
}

func TestMapClashHysteria2_UpAsString(t *testing.T) {
	in := map[string]any{
		"server":   "h",
		"port":     443,
		"password": "p",
		"up":       "50 Mbps",
		"down":     "100",
	}
	got, err := mapClashHysteria2(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	var ob map[string]any
	if err := json.Unmarshal(got.Outbound, &ob); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if upN, _ := asInt(ob["up_mbps"]); upN != 50 {
		t.Errorf("up_mbps=%v want 50", ob["up_mbps"])
	}
	if downN, _ := asInt(ob["down_mbps"]); downN != 100 {
		t.Errorf("down_mbps=%v want 100", ob["down_mbps"])
	}
}

func TestMapClashHysteria2_MissingPassword(t *testing.T) {
	_, err := mapClashHysteria2(map[string]any{
		"server": "h",
		"port":   443,
	})
	if err == nil || !strings.Contains(err.Error(), "password") {
		t.Errorf("want password error, got %v", err)
	}
}

func TestMapClashHysteria2_ObfsWithoutPassword(t *testing.T) {
	_, err := mapClashHysteria2(map[string]any{
		"server":   "h",
		"port":     443,
		"password": "p",
		"obfs":     "salamander",
	})
	if err == nil || !strings.Contains(err.Error(), "obfs requires obfs-password") {
		t.Errorf("want 'obfs requires obfs-password' error, got %v", err)
	}
}
