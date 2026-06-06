package command

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/ndms"
	"github.com/hoaxisr/awg-manager/internal/ndms/query"
)

type bindFailPoster struct{ calls int }

func (p *bindFailPoster) Post(_ context.Context, _ any) (json.RawMessage, error) {
	p.calls++
	if p.calls == 5 {
		return nil, errors.New("bind-fail")
	}
	return json.RawMessage(`{}`), nil
}

func newTestPingCheckCommands(_ *testing.T) (*PingCheckCommands, *fakePoster) {
	poster := &fakePoster{}
	pub := &fakePublisher{}
	sc := NewSaveCoordinator(poster, pub, 500*time.Millisecond, 5*time.Second, 0, nil)
	q := query.NewQueries(query.Deps{Getter: query.NewFakeGetter(), Logger: query.NopLogger()})
	return NewPingCheckCommands(poster, sc, q), poster
}

func TestPingCheckCommands_ConfigureProfile_PostSequence(t *testing.T) {
	cmds, poster := newTestPingCheckCommands(t)
	err := cmds.ConfigureProfile(context.Background(), "myprofile", "Wireguard0", ndms.PingCheckConfig{
		Host:           "8.8.8.8",
		Mode:           "ip",
		UpdateInterval: 60,
		Timeout:        1,
		MaxFails:       3,
		MinSuccess:     1,
		Restart:        true,
	})
	if err != nil {
		t.Fatalf("ConfigureProfile: %v", err)
	}
	if len(poster.Payloads()) != 5 {
		t.Fatalf("POST count: want 5, got %d", len(poster.Payloads()))
	}

	p4 := poster.Payloads()[3].(map[string]any)
	profile := p4["ping-check"].(map[string]any)["profile"].(map[string]any)["myprofile"].(map[string]any)
	if profile["host"] != "8.8.8.8" || profile["mode"] != "ip" {
		t.Errorf("profile: %#v", profile)
	}
	ui := profile["update-interval"].(map[string]any)
	if ui["seconds"] != 60 {
		t.Errorf("update-interval: %#v", ui)
	}

	p5 := poster.Payloads()[4].(map[string]any)
	pc := p5["interface"].(map[string]any)["Wireguard0"].(map[string]any)["ping-check"].(map[string]any)
	if pc["profile"] != "myprofile" || pc["restart"] != true {
		t.Errorf("bind: %#v", pc)
	}
}

func TestPingCheckCommands_ConfigureProfile_PortOmittedForIPMode(t *testing.T) {
	cmds, poster := newTestPingCheckCommands(t)
	_ = cmds.ConfigureProfile(context.Background(), "p", "W0", ndms.PingCheckConfig{
		Host: "8.8.8.8", Mode: "ip", UpdateInterval: 60, Timeout: 1, Port: 443,
	})
	profile := poster.Payloads()[3].(map[string]any)["ping-check"].(map[string]any)["profile"].(map[string]any)["p"].(map[string]any)
	if _, ok := profile["port"]; ok {
		t.Errorf("port must be omitted for ip mode, got %v", profile["port"])
	}
}

func TestPingCheckCommands_ConfigureProfile_PortIncludedForConnectMode(t *testing.T) {
	cmds, poster := newTestPingCheckCommands(t)
	_ = cmds.ConfigureProfile(context.Background(), "p", "W0", ndms.PingCheckConfig{
		Host: "example.com", Mode: "connect", UpdateInterval: 60, Timeout: 1, Port: 443,
	})
	profile := poster.Payloads()[3].(map[string]any)["ping-check"].(map[string]any)["profile"].(map[string]any)["p"].(map[string]any)
	if profile["port"] != 443 {
		t.Errorf("port: %v", profile["port"])
	}
}

func TestPingCheckCommands_ConfigureProfile_PortIncludedForTLSMode(t *testing.T) {
	cmds, poster := newTestPingCheckCommands(t)
	_ = cmds.ConfigureProfile(context.Background(), "p", "W0", ndms.PingCheckConfig{
		Host: "dns.example", Mode: "tls", UpdateInterval: 60, Timeout: 1, Port: 853,
	})
	profile := poster.Payloads()[3].(map[string]any)["ping-check"].(map[string]any)["profile"].(map[string]any)["p"].(map[string]any)
	if profile["port"] != 853 {
		t.Errorf("port: %v", profile["port"])
	}
}

func TestPingCheckCommands_ConfigureProfile_CreateError(t *testing.T) {
	cmds, poster := newTestPingCheckCommands(t)
	poster.SetError(errors.New("boom"))
	err := cmds.ConfigureProfile(context.Background(), "p", "W0", ndms.PingCheckConfig{
		Host: "1.1.1.1", Mode: "icmp", UpdateInterval: 45, Timeout: 5, Restart: true,
	})
	if err == nil || !strings.Contains(err.Error(), "create ping-check profile") {
		t.Fatalf("err = %v, want contains create ping-check profile", err)
	}
}

func TestPingCheckCommands_ConfigureProfile_BindError(t *testing.T) {
	poster := &bindFailPoster{}
	pub := &fakePublisher{}
	sc := NewSaveCoordinator(poster, pub, 500*time.Millisecond, 5*time.Second, 0, nil)
	q := query.NewQueries(query.Deps{Getter: query.NewFakeGetter(), Logger: query.NopLogger()})
	cmds := NewPingCheckCommands(poster, sc, q)
	err := cmds.ConfigureProfile(context.Background(), "p", "W0", ndms.PingCheckConfig{
		Host: "1.1.1.1", Mode: "icmp", UpdateInterval: 45, Timeout: 5, Restart: true,
	})
	if err == nil || !strings.Contains(err.Error(), "bind ping-check profile") {
		t.Fatalf("err = %v, want contains bind ping-check profile", err)
	}
}

func TestPingCheckCommands_RemoveProfile_3PostsIgnoreErrors(t *testing.T) {
	cmds, poster := newTestPingCheckCommands(t)
	poster.SetError(nil)
	if err := cmds.RemoveProfile(context.Background(), "myprofile", "Wireguard0"); err != nil {
		t.Fatalf("RemoveProfile: %v", err)
	}
	if len(poster.Payloads()) != 3 {
		t.Errorf("POST count: want 3, got %d", len(poster.Payloads()))
	}
}
