package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/deviceproxy"
	singboxorch "github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
)

type fakeOutboundsProvider struct {
	items []deviceproxy.Outbound
}

func (f *fakeOutboundsProvider) ListOutbounds(context.Context) []deviceproxy.Outbound {
	out := make([]deviceproxy.Outbound, len(f.items))
	copy(out, f.items)
	return out
}

type fakeDownloadSingbox struct {
	running bool

	selectorCalls []string
	selectorErrs  []error

	activeNow  string
	activeErrs []error
}

func (f *fakeDownloadSingbox) IsRunning() (bool, int) {
	if f.running {
		return true, 123
	}
	return false, 0
}

func (f *fakeDownloadSingbox) SetSelectorDefault(_ context.Context, selectorTag, memberTag string) error {
	f.selectorCalls = append(f.selectorCalls, selectorTag+"="+memberTag)
	f.activeNow = memberTag
	if len(f.selectorErrs) == 0 {
		return nil
	}
	err := f.selectorErrs[0]
	f.selectorErrs = f.selectorErrs[1:]
	return err
}

func (f *fakeDownloadSingbox) GetSelectorActive(_ context.Context, _ string) (string, error) {
	if len(f.activeErrs) > 0 {
		err := f.activeErrs[0]
		f.activeErrs = f.activeErrs[1:]
		if err != nil {
			return "", err
		}
	}
	return f.activeNow, nil
}

type fakeDownloadOrch struct {
	saveCalls   int
	enableCalls []bool
	reloadCalls int

	saveErr   error
	enableErr error
	reloadErr error

	lastSlot singboxorch.Slot
	lastJSON string
}

func (f *fakeDownloadOrch) SaveSilent(slot singboxorch.Slot, b []byte) error {
	f.saveCalls++
	f.lastSlot = slot
	f.lastJSON = string(b)
	return f.saveErr
}

func (f *fakeDownloadOrch) SetEnabledSilent(slot singboxorch.Slot, enabled bool) error {
	f.lastSlot = slot
	f.enableCalls = append(f.enableCalls, enabled)
	return f.enableErr
}

func (f *fakeDownloadOrch) Reload() error {
	f.reloadCalls++
	return f.reloadErr
}

func TestResolveDownloadClient_Direct(t *testing.T) {
	h := &HydraRouteHandler{}

	client, restore, err := h.resolveDownloadClient(context.Background(), nil)
	if err != nil {
		t.Fatalf("resolve direct nil route: %v", err)
	}
	if client != nil {
		t.Fatalf("direct route should use nil client override")
	}
	if restore != nil {
		t.Fatalf("direct route should not return restore callback")
	}

	client, restore, err = h.resolveDownloadClient(context.Background(), &DownloadRouteDTO{Tag: "direct"})
	if err != nil {
		t.Fatalf("resolve direct tag: %v", err)
	}
	if client != nil {
		t.Fatalf("direct tag should use nil client override")
	}
	if restore != nil {
		t.Fatalf("direct tag should not return restore callback")
	}
}

func TestResolveDownloadClient_NonDirectWithoutSingbox(t *testing.T) {
	h := &HydraRouteHandler{}
	_, _, err := h.resolveDownloadClient(context.Background(), &DownloadRouteDTO{Tag: "awg-a"})
	if err == nil {
		t.Fatal("expected error for non-direct route without sing-box operator")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveDownloadClient_RoutedUnavailable_DoesNotWriteSlot(t *testing.T) {
	sb := &fakeDownloadSingbox{running: true}
	orch := &fakeDownloadOrch{}
	prov := &fakeOutboundsProvider{
		items: []deviceproxy.Outbound{
			{Tag: "direct", Kind: "direct", Label: "Direct (WAN)"},
			{Tag: "awg-a", Kind: "awg", Label: "AWG A"},
		},
	}
	h := &HydraRouteHandler{
		singboxOp:      sb,
		downloadOrch:   orch,
		deviceProxySvc: prov,
	}

	_, _, err := h.resolveDownloadClient(context.Background(), &DownloadRouteDTO{Tag: "missing"})
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("expected unavailable error, got %v", err)
	}
	if orch.saveCalls != 0 {
		t.Fatalf("SaveSilent must not be called, got %d", orch.saveCalls)
	}
	if len(orch.enableCalls) != 0 {
		t.Fatalf("SetEnabledSilent must not be called, got %v", orch.enableCalls)
	}
	if orch.reloadCalls != 0 {
		t.Fatalf("Reload must not be called, got %d", orch.reloadCalls)
	}
}

func TestResolveDownloadClient_Routed_UsesOrchestratorSlotAndCleanup(t *testing.T) {
	sb := &fakeDownloadSingbox{running: true}
	orch := &fakeDownloadOrch{}
	prov := &fakeOutboundsProvider{
		items: []deviceproxy.Outbound{
			{Tag: "direct", Kind: "direct", Label: "Direct (WAN)"},
			{Tag: "awg-a", Kind: "awg", Label: "AWG A"},
		},
	}
	h := &HydraRouteHandler{
		singboxOp:      sb,
		downloadOrch:   orch,
		deviceProxySvc: prov,
	}

	client, restore, err := h.resolveDownloadClient(context.Background(), &DownloadRouteDTO{Tag: "awg-a"})
	if err != nil {
		t.Fatalf("resolve routed client: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil routed client")
	}
	if restore == nil {
		t.Fatal("expected restore callback")
	}
	if orch.saveCalls != 1 {
		t.Fatalf("SaveSilent calls: got %d want 1", orch.saveCalls)
	}
	if orch.lastSlot != singboxorch.SlotDownloadProxy {
		t.Fatalf("last slot: got %q want %q", orch.lastSlot, singboxorch.SlotDownloadProxy)
	}
	if !strings.Contains(orch.lastJSON, "\"tag\": \"awgm-download-selector\"") {
		t.Fatalf("slot json missing selector tag: %s", orch.lastJSON)
	}
	if !strings.Contains(orch.lastJSON, "\"default\": \"awg-a\"") {
		t.Fatalf("slot json missing selected default: %s", orch.lastJSON)
	}
	if !strings.Contains(orch.lastJSON, "\"listen_port\": 11998") {
		t.Fatalf("slot json missing listen_port: %s", orch.lastJSON)
	}
	if len(orch.enableCalls) < 1 || orch.enableCalls[0] != true {
		t.Fatalf("expected first enable call true, got %v", orch.enableCalls)
	}
	if orch.reloadCalls < 1 {
		t.Fatalf("expected reload on enable, got %d", orch.reloadCalls)
	}
	if len(sb.selectorCalls) == 0 || sb.selectorCalls[0] != "awgm-download-selector=awg-a" {
		t.Fatalf("expected selector set to awg-a, got %v", sb.selectorCalls)
	}

	restore()

	if len(sb.selectorCalls) < 2 || sb.selectorCalls[len(sb.selectorCalls)-1] != "awgm-download-selector=direct" {
		t.Fatalf("expected selector restore to direct, got %v", sb.selectorCalls)
	}
	if len(orch.enableCalls) < 2 || orch.enableCalls[len(orch.enableCalls)-1] != false {
		t.Fatalf("expected disable call on cleanup, got %v", orch.enableCalls)
	}
	if orch.reloadCalls < 2 {
		t.Fatalf("expected second reload on cleanup, got %d", orch.reloadCalls)
	}

	_, restore2, err := h.resolveDownloadClient(context.Background(), &DownloadRouteDTO{Tag: "awg-a"})
	if err != nil {
		t.Fatalf("second resolve should not deadlock: %v", err)
	}
	if restore2 != nil {
		restore2()
	}
}

func TestResolveDownloadClient_Routed_ReloadFailureDisablesSlotAndUnlocks(t *testing.T) {
	sb := &fakeDownloadSingbox{running: true}
	orch := &fakeDownloadOrch{reloadErr: errors.New("reload failed")}
	prov := &fakeOutboundsProvider{
		items: []deviceproxy.Outbound{
			{Tag: "direct", Kind: "direct", Label: "Direct (WAN)"},
			{Tag: "awg-a", Kind: "awg", Label: "AWG A"},
		},
	}
	h := &HydraRouteHandler{
		singboxOp:      sb,
		downloadOrch:   orch,
		deviceProxySvc: prov,
	}

	_, _, err := h.resolveDownloadClient(context.Background(), &DownloadRouteDTO{Tag: "awg-a"})
	if err == nil || !strings.Contains(err.Error(), "reload sing-box with download transport slot") {
		t.Fatalf("expected reload error, got %v", err)
	}
	if len(orch.enableCalls) < 2 || orch.enableCalls[len(orch.enableCalls)-1] != false {
		t.Fatalf("expected disable call after reload failure, got %v", orch.enableCalls)
	}

	orch.reloadErr = nil
	_, restore2, err := h.resolveDownloadClient(context.Background(), &DownloadRouteDTO{Tag: "awg-a"})
	if err != nil {
		t.Fatalf("second resolve should not deadlock: %v", err)
	}
	if restore2 != nil {
		restore2()
	}
}

func TestListDownloadOutbounds_NoDeviceProxy(t *testing.T) {
	h := &HydraRouteHandler{}
	req := httptest.NewRequest(http.MethodGet, "/download/outbounds", nil)
	rr := httptest.NewRecorder()

	h.ListDownloadOutbounds(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, `"tag":"direct"`) {
		t.Fatalf("expected direct tag in response: %s", body)
	}
	if !strings.Contains(body, `"available":true`) {
		t.Fatalf("expected direct available=true in response: %s", body)
	}
}

func TestSelectedTagForSlot(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "", want: "direct"},
		{in: "   ", want: "direct"},
		{in: "sub-1", want: "sub-1"},
	}
	for _, tc := range cases {
		if got := selectedTagForSlot(tc.in); got != tc.want {
			t.Fatalf("selectedTagForSlot(%q): got %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestSelectDownloadOutboundWithRetry_Retries(t *testing.T) {
	sb := &fakeDownloadSingbox{
		running: true,
		selectorErrs: []error{
			errors.New("not ready"),
			errors.New("not ready"),
			nil,
		},
	}
	h := &HydraRouteHandler{singboxOp: sb}

	if err := h.selectDownloadOutboundWithRetry(context.Background(), "awg-a"); err != nil {
		t.Fatalf("select with retry: %v", err)
	}
	if len(sb.selectorCalls) != 3 {
		t.Fatalf("expected 3 attempts, got %d", len(sb.selectorCalls))
	}
}

func TestReadSelectorActiveWithRetry_Retries(t *testing.T) {
	sb := &fakeDownloadSingbox{
		activeErrs: []error{
			errors.New("not ready"),
			errors.New("not ready"),
			nil,
		},
		activeNow: "awg-a",
	}
	h := &HydraRouteHandler{singboxOp: sb}
	active, err := h.readSelectorActiveWithRetry(context.Background(), downloadProxySelectorTag)
	if err != nil {
		t.Fatalf("read active with retry: %v", err)
	}
	if active != "awg-a" {
		t.Fatalf("active: got %q want %q", active, "awg-a")
	}
}

func TestApplyDownloadProxySlotLocked_JSONHasSelectedDefault(t *testing.T) {
	orch := &fakeDownloadOrch{}
	h := &HydraRouteHandler{downloadOrch: orch}
	err := h.applyDownloadProxySlotLocked([]string{"direct", "awg-a", "awg-a"}, "awg-a")
	if err != nil {
		t.Fatalf("apply slot: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(orch.lastJSON), &parsed); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	inboundsAny, ok := parsed["inbounds"].([]interface{})
	if !ok || len(inboundsAny) != 1 {
		t.Fatalf("inbounds: got %T %v", parsed["inbounds"], parsed["inbounds"])
	}
	inbound, ok := inboundsAny[0].(map[string]interface{})
	if !ok {
		t.Fatalf("inbound item type: %T", inboundsAny[0])
	}
	if inbound["tag"] != downloadProxyInboundTag {
		t.Fatalf("inbound tag = %v, want %s", inbound["tag"], downloadProxyInboundTag)
	}
	if inbound["listen"] != downloadProxyListenHost {
		t.Fatalf("inbound listen = %v, want %s", inbound["listen"], downloadProxyListenHost)
	}
	if int(inbound["listen_port"].(float64)) != downloadProxyListenPort {
		t.Fatalf("inbound listen_port = %v, want %d", inbound["listen_port"], downloadProxyListenPort)
	}

	outboundsAny, ok := parsed["outbounds"].([]interface{})
	if !ok || len(outboundsAny) != 1 {
		t.Fatalf("outbounds: got %T %v", parsed["outbounds"], parsed["outbounds"])
	}
	selector, ok := outboundsAny[0].(map[string]interface{})
	if !ok {
		t.Fatalf("selector item type: %T", outboundsAny[0])
	}
	if selector["tag"] != downloadProxySelectorTag {
		t.Fatalf("selector tag = %v, want %s", selector["tag"], downloadProxySelectorTag)
	}
	if selector["default"] != "awg-a" {
		t.Fatalf("selector default = %v, want awg-a", selector["default"])
	}
	membersAny, ok := selector["outbounds"].([]interface{})
	if !ok {
		t.Fatalf("selector outbounds type: %T", selector["outbounds"])
	}
	members := make([]string, 0, len(membersAny))
	for _, m := range membersAny {
		members = append(members, m.(string))
	}
	if len(members) != 2 || members[0] != "direct" || members[1] != "awg-a" {
		t.Fatalf("selector members = %v, want [direct awg-a]", members)
	}
}
