package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/storage"
	"github.com/hoaxisr/awg-manager/internal/tunnel"
	servicesvc "github.com/hoaxisr/awg-manager/internal/tunnel/service"
	"github.com/hoaxisr/awg-manager/internal/tunnel/wan"
)

type replaceTunnelServiceStub struct {
	state         tunnel.StateInfo
	tunnel        *servicesvc.TunnelWithStatus
	stopCalls     int
	startCalls    int
	replaceCalls  int
	lastReplaceID string
}

func (s *replaceTunnelServiceStub) List(ctx context.Context) ([]servicesvc.TunnelWithStatus, error) {
	return nil, nil
}

func (s *replaceTunnelServiceStub) Get(ctx context.Context, tunnelID string) (*servicesvc.TunnelWithStatus, error) {
	return s.tunnel, nil
}

func (s *replaceTunnelServiceStub) Create(ctx context.Context, tunnelID, name string, cfg tunnel.Config, stored *storage.AWGTunnel) error {
	return nil
}

func (s *replaceTunnelServiceStub) Update(ctx context.Context, oldStored, newStored *storage.AWGTunnel) error {
	return nil
}

func (s *replaceTunnelServiceStub) Delete(ctx context.Context, tunnelID string) error {
	return nil
}

func (s *replaceTunnelServiceStub) Start(ctx context.Context, tunnelID string) error {
	s.startCalls++
	return nil
}

func (s *replaceTunnelServiceStub) Stop(ctx context.Context, tunnelID string) error {
	s.stopCalls++
	return nil
}

func (s *replaceTunnelServiceStub) Restart(ctx context.Context, tunnelID string) error {
	return nil
}

func (s *replaceTunnelServiceStub) CheckAddressConflicts(ctx context.Context, tunnelID string) []string {
	return nil
}

func (s *replaceTunnelServiceStub) GetState(ctx context.Context, tunnelID string) tunnel.StateInfo {
	return s.state
}

func (s *replaceTunnelServiceStub) SetEnabled(ctx context.Context, tunnelID string, enabled bool) error {
	return nil
}

func (s *replaceTunnelServiceStub) SetDefaultRoute(ctx context.Context, tunnelID string, enabled bool) error {
	return nil
}

func (s *replaceTunnelServiceStub) Import(ctx context.Context, confContent, name, backend string) (*servicesvc.TunnelWithStatus, error) {
	return nil, nil
}

func (s *replaceTunnelServiceStub) ReplaceConfig(ctx context.Context, tunnelID, confContent, newName string) error {
	s.replaceCalls++
	s.lastReplaceID = tunnelID
	return nil
}

func (s *replaceTunnelServiceStub) WANModel() *wan.Model {
	return nil
}

func (s *replaceTunnelServiceStub) GetResolvedISP(tunnelID string) string {
	return ""
}

func (s *replaceTunnelServiceStub) SetSelfCreateGate(g tunnel.SelfCreateGater) {}

func newReplaceHandlerTestStore(t *testing.T) *storage.AWGTunnelStore {
	t.Helper()

	store := storage.NewAWGTunnelStoreWithLockDir(t.TempDir(), t.TempDir())
	if err := store.Save(&storage.AWGTunnel{
		ID:      "awg12",
		Name:    "Test tunnel",
		Backend: "kernel",
		Enabled: true,
		Interface: storage.AWGInterface{
			Address: "10.8.1.75/32",
			MTU:     1420,
		},
		Peer: storage.AWGPeer{
			PublicKey:  "pub",
			Endpoint:   "vpn.example.com:51820",
			AllowedIPs: []string{"0.0.0.0/0"},
		},
	}); err != nil {
		t.Fatalf("save tunnel: %v", err)
	}

	return store
}

func newReplaceRequest(t *testing.T) *http.Request {
	t.Helper()

	body, err := json.Marshal(map[string]string{
		"content": "[Interface]\nPrivateKey = key\nAddress = 10.8.1.76/32\n\n[Peer]\nPublicKey = peer\nAllowedIPs = 0.0.0.0/0\nEndpoint = vpn.example.com:51820\n",
		"name":    "impvpn-RU",
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	return httptest.NewRequest(http.MethodPost, "/api/tunnels/replace?id=awg12", bytes.NewReader(body))
}

func TestReplaceNeedsStop_DirtyKernelNeedsStop(t *testing.T) {
	info := tunnel.StateInfo{
		State:          tunnel.StateNeedsStop,
		BackendType:    "kernel",
		ProcessRunning: true,
		OpkgTunExists:  true,
	}

	if !replaceNeedsStop(info) {
		t.Fatal("replaceNeedsStop() = false, want true for dirty kernel needs_stop")
	}
	if replaceNeedsRestart(info) {
		t.Fatal("replaceNeedsRestart() = true, want false for needs_stop cleanup-only state")
	}
}

func TestReplaceConf_StopsDirtyKernelRuntimeBeforeReplace(t *testing.T) {
	store := newReplaceHandlerTestStore(t)
	svc := &replaceTunnelServiceStub{
		state: tunnel.StateInfo{
			State:          tunnel.StateNeedsStop,
			BackendType:    "kernel",
			ProcessRunning: true,
			OpkgTunExists:  true,
		},
		tunnel: &servicesvc.TunnelWithStatus{
			ID:            "awg12",
			Name:          "Test tunnel",
			Enabled:       true,
			DefaultRoute:  true,
			InterfaceName: "opkgtun12",
			StateInfo: tunnel.StateInfo{
				State:       tunnel.StateStopped,
				BackendType: "kernel",
			},
		},
	}

	handler := NewTunnelsHandler(svc, store, nil)
	rr := httptest.NewRecorder()

	handler.ReplaceConf(rr, newReplaceRequest(t))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if svc.stopCalls != 1 {
		t.Fatalf("stopCalls = %d, want 1", svc.stopCalls)
	}
	if svc.replaceCalls != 1 {
		t.Fatalf("replaceCalls = %d, want 1", svc.replaceCalls)
	}
	if svc.startCalls != 0 {
		t.Fatalf("startCalls = %d, want 0 for cleanup-only needs_stop state", svc.startCalls)
	}
}

func TestReplaceConf_RestartsRunningTunnelAfterReplace(t *testing.T) {
	store := newReplaceHandlerTestStore(t)
	svc := &replaceTunnelServiceStub{
		state: tunnel.StateInfo{
			State:       tunnel.StateRunning,
			BackendType: "kernel",
		},
		tunnel: &servicesvc.TunnelWithStatus{
			ID:            "awg12",
			Name:          "Test tunnel",
			Enabled:       true,
			DefaultRoute:  true,
			InterfaceName: "opkgtun12",
			StateInfo: tunnel.StateInfo{
				State:       tunnel.StateRunning,
				BackendType: "kernel",
			},
		},
	}

	handler := NewTunnelsHandler(svc, store, nil)
	rr := httptest.NewRecorder()

	handler.ReplaceConf(rr, newReplaceRequest(t))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if svc.stopCalls != 1 {
		t.Fatalf("stopCalls = %d, want 1", svc.stopCalls)
	}
	if svc.replaceCalls != 1 {
		t.Fatalf("replaceCalls = %d, want 1", svc.replaceCalls)
	}
	if svc.startCalls != 1 {
		t.Fatalf("startCalls = %d, want 1 for running tunnel", svc.startCalls)
	}
}

func TestReplaceConf_StopsStoppedKernelWithDirtyRuntime(t *testing.T) {
	store := newReplaceHandlerTestStore(t)
	svc := &replaceTunnelServiceStub{
		state: tunnel.StateInfo{
			State:          tunnel.StateStopped,
			BackendType:    "kernel",
			ProcessRunning: true,
			OpkgTunExists:  true,
		},
		tunnel: &servicesvc.TunnelWithStatus{
			ID:            "awg12",
			Name:          "Test tunnel",
			Enabled:       true,
			DefaultRoute:  true,
			InterfaceName: "opkgtun12",
			StateInfo: tunnel.StateInfo{
				State:       tunnel.StateStopped,
				BackendType: "kernel",
			},
		},
	}

	handler := NewTunnelsHandler(svc, store, nil)
	rr := httptest.NewRecorder()
	handler.ReplaceConf(rr, newReplaceRequest(t))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if svc.stopCalls != 1 || svc.startCalls != 0 {
		t.Fatalf("stop/start = %d/%d, want 1/0", svc.stopCalls, svc.startCalls)
	}
}

func TestReplaceConf_DoesNotStopCleanStoppedKernel(t *testing.T) {
	store := newReplaceHandlerTestStore(t)
	svc := &replaceTunnelServiceStub{
		state: tunnel.StateInfo{
			State:       tunnel.StateStopped,
			BackendType: "kernel",
		},
		tunnel: &servicesvc.TunnelWithStatus{
			ID:            "awg12",
			Name:          "Test tunnel",
			Enabled:       true,
			DefaultRoute:  true,
			InterfaceName: "opkgtun12",
			StateInfo: tunnel.StateInfo{
				State:       tunnel.StateStopped,
				BackendType: "kernel",
			},
		},
	}

	handler := NewTunnelsHandler(svc, store, nil)
	rr := httptest.NewRecorder()
	handler.ReplaceConf(rr, newReplaceRequest(t))

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}
	if svc.stopCalls != 0 || svc.startCalls != 0 {
		t.Fatalf("stop/start = %d/%d, want 0/0", svc.stopCalls, svc.startCalls)
	}
}
