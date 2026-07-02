package singboxwatchdog

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/logging"
	"github.com/hoaxisr/awg-manager/internal/singbox"
	"github.com/hoaxisr/awg-manager/internal/singbox/subscription"
)

type fakeOp struct {
	status          singbox.Status
	tunnels         []singbox.TunnelInfo
	clash           *fakeClash
	restarts        int
	activeBySel     map[string]string
	selectorCalls   int
	manuallyStopped bool
	controlErr      error
	controlBlock    chan struct{}
}

func (f *fakeOp) GetStatus(ctx context.Context) singbox.Status { return f.status }
func (f *fakeOp) ListTunnels(ctx context.Context) ([]singbox.TunnelInfo, error) {
	return f.tunnels, nil
}
func (f *fakeOp) Control(ctx context.Context, action string) error {
	if f.controlBlock != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-f.controlBlock:
		}
	}
	if f.controlErr != nil {
		return f.controlErr
	}
	if action == "restart" {
		f.restarts++
		f.status.Running = true
	}
	return nil
}
func (f *fakeOp) Clash() *singbox.ClashClient { return nil }
func (f *fakeOp) GetSelectorActive(ctx context.Context, selectorTag string) (string, error) {
	f.selectorCalls++
	return f.activeBySel[selectorTag], nil
}
func (f *fakeOp) IsSingboxManuallyStopped() bool { return f.manuallyStopped }

type fakeClash struct {
	delays      map[string]int
	setCalls    [][2]string
	testedNames []string
	setErr      error
}

func (f *fakeClash) TestDelay(name, url string, timeout time.Duration) (int, error) {
	f.testedNames = append(f.testedNames, name)
	if d, ok := f.delays[name]; ok && d > 0 {
		return d, nil
	}
	return 0, errors.New("timeout")
}
func (f *fakeClash) SetSelector(selectorTag, memberTag string) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.setCalls = append(f.setCalls, [2]string{selectorTag, memberTag})
	return nil
}
func (f *fakeClash) SelectorActive(selectorTag string) (string, error) { return "", nil }

type fakeSubs struct{ list []subscription.Subscription }

func (f *fakeSubs) List() []subscription.Subscription { return f.list }

type recordedLog struct {
	level    logging.Level
	group    string
	subgroup string
	action   string
	target   string
	message  string
}

type recordingAppLogger struct {
	mu   sync.Mutex
	logs []recordedLog
}

func (r *recordingAppLogger) AppLog(level logging.Level, group, subgroup, action, target, message string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, recordedLog{
		level:    level,
		group:    group,
		subgroup: subgroup,
		action:   action,
		target:   target,
		message:  message,
	})
}

func (r *recordingAppLogger) count(action string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, entry := range r.logs {
		if entry.action == action {
			n++
		}
	}
	return n
}

func (r *recordingAppLogger) has(action, target string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, entry := range r.logs {
		if entry.action == action && entry.target == target {
			return true
		}
	}
	return false
}

func (r *recordingAppLogger) latest() recordedLog {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.logs) == 0 {
		return recordedLog{}
	}
	return r.logs[len(r.logs)-1]
}

func newInstalledOp() *fakeOp {
	return &fakeOp{
		status: singbox.Status{Installed: true, Running: true},
	}
}

func TestStore_LoadSaveUpsertAndEnable(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "singbox_watchdog.json"))
	if err := store.Upsert(TargetConfig{
		ID:            "tunnel:sb-main",
		Kind:          TargetTunnel,
		Ref:           "sb-main",
		Enabled:       true,
		Interval:      0,
		FailThreshold: 0,
		Timeout:       0,
	}); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	cfg, ok := store.Get("tunnel:sb-main")
	if !ok {
		t.Fatal("expected config in store")
	}
	if cfg.Interval != 30 || cfg.FailThreshold != 3 || cfg.Timeout != 5 {
		t.Fatalf("normalized config mismatch: %+v", cfg)
	}
	if cfg.RecoveryMode != RecoveryOff {
		t.Fatalf("raw tunnel default recovery = %q, want %q", cfg.RecoveryMode, RecoveryOff)
	}
	if err := store.SetEnabled("tunnel:sb-main", false); err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	reloaded := NewStore(store.path)
	if err := reloaded.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg, ok = reloaded.Get("tunnel:sb-main")
	if !ok || cfg.Enabled {
		t.Fatalf("reloaded config mismatch: %+v ok=%v", cfg, ok)
	}
}

func TestCheckTarget_Stopped(t *testing.T) {
	res := CheckTarget(context.Background(), &fakeClash{}, RuntimeTarget{Running: false}, 2*time.Second)
	if res.Success || res.Error != "target stopped" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestService_GetStatus_ResolvesSubscriptionAndHidesOwnedRawTunnel(t *testing.T) {
	op := newInstalledOp()
	op.tunnels = []singbox.TunnelInfo{
		{Tag: "iq0", Running: true},
		{Tag: "raw-1", Running: true, Protocol: "vless"},
	}
	op.activeBySel = map[string]string{"iq0": "member-a"}
	svc := NewService(NewStore(filepath.Join(t.TempDir(), "store.json")), op, &fakeSubs{
		list: []subscription.Subscription{{
			ID:           "sub-1",
			Label:        "Pool",
			SelectorTag:  "iq0",
			ActiveMember: "member-a",
			Enabled:      true,
			MemberTags:   []string{"member-a", "member-b"},
			Members: []subscription.MemberInfo{{
				Tag: "member-a", Protocol: "vless", Security: "reality", Transport: "tcp",
			}},
		}},
	}, nil)
	statuses := svc.GetStatus(context.Background())
	if len(statuses) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(statuses))
	}
	if statuses[0].ID != "subscription:sub-1" || statuses[1].ID != "tunnel:raw-1" {
		t.Fatalf("unexpected targets: %+v", statuses)
	}
}

func TestService_Configure_RejectsUnknownTarget(t *testing.T) {
	svc := NewService(NewStore(filepath.Join(t.TempDir(), "store.json")), newInstalledOp(), &fakeSubs{}, nil)
	err := svc.Configure(TargetConfig{
		ID:            "tunnel:missing",
		Kind:          TargetTunnel,
		Ref:           "missing",
		Enabled:       true,
		Interval:      30,
		FailThreshold: 3,
		Timeout:       5,
	})
	if !errors.Is(err, ErrTargetNotFound) {
		t.Fatalf("Configure error = %v, want ErrTargetNotFound", err)
	}
}

func TestService_Configure_WritesProjectLog(t *testing.T) {
	rec := &recordingAppLogger{}
	op := newInstalledOp()
	op.tunnels = []singbox.TunnelInfo{{Tag: "sb-main", Running: true}}
	svc := NewService(NewStore(filepath.Join(t.TempDir(), "store.json")), op, &fakeSubs{}, rec)
	err := svc.Configure(TargetConfig{
		ID:            "tunnel:sb-main",
		Kind:          TargetTunnel,
		Ref:           "sb-main",
		Enabled:       true,
		Interval:      30,
		FailThreshold: 3,
		Timeout:       5,
		RecoveryMode:  RecoveryOff,
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if !rec.has("configure", "tunnel:sb-main") {
		t.Fatal("expected configure project log entry")
	}
	entry := rec.latest()
	if entry.group != logging.GroupSingbox || entry.subgroup != logging.SubSBWatchdog {
		t.Fatalf("unexpected log scope: %+v", entry)
	}
}

func TestService_ManualStop_SkipsRawRecovery(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "store.json"))
	cfg := TargetConfig{
		ID:            "tunnel:sb-main",
		Kind:          TargetTunnel,
		Ref:           "sb-main",
		Enabled:       true,
		Interval:      30,
		FailThreshold: 1,
		Timeout:       1,
		RecoveryMode:  RecoveryRestartSingbox,
	}
	if err := store.Upsert(cfg); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	rec := &recordingAppLogger{}
	op := newInstalledOp()
	op.manuallyStopped = true
	op.tunnels = []singbox.TunnelInfo{{Tag: "sb-main", Running: true}}
	clash := &fakeClash{}
	svc := NewService(store, op, &fakeSubs{}, rec)
	svc.SetManualStopReader(op)
	svc.clash = clash
	target, ok := svc.resolveTargetByConfig(context.Background(), cfg)
	if !ok {
		t.Fatal("target not resolved")
	}
	svc.recoverRawTunnel(context.Background(), target, cfg)
	if op.restarts != 0 {
		t.Fatalf("restart count = %d, want 0", op.restarts)
	}
	if !rec.has("recovery-skipped", "tunnel:sb-main") {
		t.Fatal("expected recovery-skipped project log")
	}
	logs := svc.GetLogs("tunnel:sb-main")
	if len(logs) == 0 || logs[len(logs)-1].StateChange != "recovery_skipped" {
		t.Fatalf("expected recovery_skipped domain log, got %+v", logs)
	}
}

func TestService_GetStatus_CachesSelectorActiveWithinTTL(t *testing.T) {
	op := newInstalledOp()
	op.tunnels = []singbox.TunnelInfo{{Tag: "iq0", Running: true}}
	op.activeBySel = map[string]string{"iq0": "member-a"}
	svc := NewService(NewStore(filepath.Join(t.TempDir(), "store.json")), op, &fakeSubs{
		list: []subscription.Subscription{{
			ID:           "sub-1",
			Label:        "Pool",
			SelectorTag:  "iq0",
			ActiveMember: "member-fallback",
			Enabled:      true,
			MemberTags:   []string{"member-a", "member-b"},
		}},
	}, nil)

	first := svc.GetStatus(context.Background())
	second := svc.GetStatus(context.Background())

	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("unexpected status count: first=%d second=%d", len(first), len(second))
	}
	if first[0].ActiveMemberTag != "member-a" || second[0].ActiveMemberTag != "member-a" {
		t.Fatalf("unexpected active member tags: first=%q second=%q", first[0].ActiveMemberTag, second[0].ActiveMemberTag)
	}
	if op.selectorCalls != 1 {
		t.Fatalf("GetSelectorActive calls = %d, want 1 within TTL", op.selectorCalls)
	}
}

func TestService_RecoverRawTunnel_SingleFlight(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "store.json"))
	cfg := TargetConfig{
		ID:            "tunnel:sb-main",
		Kind:          TargetTunnel,
		Ref:           "sb-main",
		Enabled:       true,
		Interval:      30,
		FailThreshold: 1,
		Timeout:       1,
		RecoveryMode:  RecoveryRestartSingbox,
	}
	if err := store.Upsert(cfg); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	op := newInstalledOp()
	op.tunnels = []singbox.TunnelInfo{{Tag: "sb-main", Running: true}}
	op.controlBlock = make(chan struct{})
	svc := NewService(store, op, &fakeSubs{}, nil)
	svc.clash = &fakeClash{delays: map[string]int{"sb-main": 10}}
	target, ok := svc.resolveTargetByConfig(context.Background(), cfg)
	if !ok {
		t.Fatal("target not resolved")
	}

	done := make(chan struct{})
	go func() {
		svc.recoverRawTunnel(context.Background(), target, cfg)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	svc.recoverRawTunnel(context.Background(), target, cfg)
	close(op.controlBlock)
	<-done

	if op.restarts != 1 {
		t.Fatalf("restart count = %d, want 1", op.restarts)
	}
}

func TestService_Reconcile_StopsMonitorWhenTargetDisappears(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "store.json"))
	cfg := TargetConfig{
		ID:            "tunnel:sb-main",
		Kind:          TargetTunnel,
		Ref:           "sb-main",
		Enabled:       true,
		Interval:      30,
		FailThreshold: 3,
		Timeout:       5,
		RecoveryMode:  RecoveryOff,
	}
	if err := store.Upsert(cfg); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	op := newInstalledOp()
	op.tunnels = []singbox.TunnelInfo{{Tag: "sb-main", Running: true}}
	svc := NewService(store, op, &fakeSubs{}, nil)
	svc.Start(context.Background())
	defer svc.Stop()

	time.Sleep(50 * time.Millisecond)
	svc.mu.RLock()
	_, runningBefore := svc.monitors["tunnel:sb-main"]
	svc.mu.RUnlock()
	if !runningBefore {
		t.Fatal("expected monitor to start")
	}

	op.tunnels = nil
	svc.Reconcile()
	time.Sleep(50 * time.Millisecond)

	svc.mu.RLock()
	_, runningAfter := svc.monitors["tunnel:sb-main"]
	svc.mu.RUnlock()
	if runningAfter {
		t.Fatal("expected monitor to stop after target disappeared")
	}
}

func TestService_RecoverSubscription_SwitchesMember(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "store.json"))
	cfg := TargetConfig{
		ID:            "subscription:sub-1",
		Kind:          TargetSubscription,
		Ref:           "sub-1",
		Enabled:       true,
		Interval:      30,
		FailThreshold: 1,
		Timeout:       1,
		RecoveryMode:  RecoverySwitchMember,
	}
	if err := store.Upsert(cfg); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	clash := &fakeClash{delays: map[string]int{
		"member-b": 45,
	}}
	op := newInstalledOp()
	op.clash = clash
	op.activeBySel = map[string]string{"iq0": "member-a"}
	svc := NewService(store, op, &fakeSubs{list: []subscription.Subscription{{
		ID:           "sub-1",
		Label:        "Pool",
		SelectorTag:  "iq0",
		ActiveMember: "member-a",
		Enabled:      true,
		ListenPort:   1080,
		MemberTags:   []string{"member-a", "member-b"},
		Members: []subscription.MemberInfo{
			{Tag: "member-a", Protocol: "vless"},
			{Tag: "member-b", Protocol: "vless"},
		},
	}}}, nil)
	svc.clash = clash
	target, ok := svc.resolveTargetByConfig(context.Background(), cfg)
	if !ok {
		t.Fatal("target not resolved")
	}
	svc.recoverSubscription(context.Background(), target, cfg)
	if len(clash.setCalls) != 1 || clash.setCalls[0] != [2]string{"iq0", "member-b"} {
		t.Fatalf("unexpected selector calls: %+v", clash.setCalls)
	}
}

func TestService_RecoverSubscription_DoesNotTestSelectorTagAsMember(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "store.json"))
	cfg := TargetConfig{
		ID:            "subscription:sub-1",
		Kind:          TargetSubscription,
		Ref:           "sub-1",
		Enabled:       true,
		Interval:      30,
		FailThreshold: 1,
		Timeout:       1,
		RecoveryMode:  RecoverySwitchMember,
	}
	_ = store.Upsert(cfg)
	clash := &fakeClash{delays: map[string]int{
		"member-b": 45,
	}}
	op := newInstalledOp()
	op.activeBySel = map[string]string{"iq0": "member-a"}
	svc := NewService(store, op, &fakeSubs{list: []subscription.Subscription{{
		ID:           "sub-1",
		SelectorTag:  "iq0",
		ActiveMember: "member-a",
		Enabled:      true,
		ListenPort:   1080,
		MemberTags:   []string{"iq0", "member-a", "member-b"},
	}}}, nil)
	svc.clash = clash
	target, _ := svc.resolveTargetByConfig(context.Background(), cfg)
	svc.recoverSubscription(context.Background(), target, cfg)
	for _, name := range clash.testedNames[:len(clash.testedNames)-1] {
		if name == "iq0" {
			t.Fatalf("selectorTag was tested as member: %+v", clash.testedNames)
		}
	}
}

func TestService_RecoverSubscription_SkipsReservedServiceTags(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "store.json"))
	cfg := TargetConfig{
		ID:            "subscription:sub-1",
		Kind:          TargetSubscription,
		Ref:           "sub-1",
		Enabled:       true,
		Interval:      30,
		FailThreshold: 1,
		Timeout:       1,
		RecoveryMode:  RecoverySwitchMember,
	}
	_ = store.Upsert(cfg)
	clash := &fakeClash{delays: map[string]int{
		"member-b": 45,
	}}
	op := newInstalledOp()
	op.activeBySel = map[string]string{"iq0": "member-a"}
	svc := NewService(store, op, &fakeSubs{list: []subscription.Subscription{{
		ID:           "sub-1",
		SelectorTag:  "iq0",
		ActiveMember: "member-a",
		Enabled:      true,
		ListenPort:   1080,
		MemberTags:   []string{"direct", "block", "final", "member-a", "member-b"},
	}}}, nil)
	svc.clash = clash
	target, _ := svc.resolveTargetByConfig(context.Background(), cfg)
	svc.recoverSubscription(context.Background(), target, cfg)
	for _, name := range clash.testedNames[:len(clash.testedNames)-1] {
		switch name {
		case "direct", "block", "final":
			t.Fatalf("reserved service tag was tested as member: %+v", clash.testedNames)
		}
	}
}

func TestService_RecoverSubscription_LimitsMemberChecks(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "store.json"))
	cfg := TargetConfig{
		ID:            "subscription:sub-1",
		Kind:          TargetSubscription,
		Ref:           "sub-1",
		Enabled:       true,
		Interval:      30,
		FailThreshold: 1,
		Timeout:       1,
		RecoveryMode:  RecoverySwitchMember,
	}
	_ = store.Upsert(cfg)
	memberTags := make([]string, 0, 32)
	delays := make(map[string]int)
	for i := 0; i < 32; i++ {
		tag := "member-" + time.Now().Add(time.Duration(i)).Format("150405.000000000")
		memberTags = append(memberTags, tag)
		if i == 11 {
			delays[tag] = 90
		}
	}
	clash := &fakeClash{delays: delays}
	op := newInstalledOp()
	op.activeBySel = map[string]string{"iq0": "member-current"}
	svc := NewService(store, op, &fakeSubs{list: []subscription.Subscription{{
		ID:           "sub-1",
		SelectorTag:  "iq0",
		ActiveMember: "member-current",
		Enabled:      true,
		ListenPort:   1080,
		MemberTags:   memberTags,
	}}}, nil)
	svc.clash = clash
	target, _ := svc.resolveTargetByConfig(context.Background(), cfg)
	svc.recoverSubscription(context.Background(), target, cfg)
	memberProbeCount := len(clash.testedNames)
	if memberProbeCount > 0 && clash.testedNames[len(clash.testedNames)-1] == "iq0" {
		memberProbeCount--
	}
	if memberProbeCount > maxMemberChecks {
		t.Fatalf("tested %d members, want <= %d", memberProbeCount, maxMemberChecks)
	}
}

func TestService_RecoverSubscription_SetSelectorErrorDoesNotRestart(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "store.json"))
	cfg := TargetConfig{
		ID:            "subscription:sub-1",
		Kind:          TargetSubscription,
		Ref:           "sub-1",
		Enabled:       true,
		Interval:      30,
		FailThreshold: 1,
		Timeout:       1,
		RecoveryMode:  RecoverySwitchMember,
	}
	_ = store.Upsert(cfg)
	clash := &fakeClash{
		delays: map[string]int{"member-b": 45},
		setErr: errors.New("unsupported"),
	}
	op := newInstalledOp()
	op.activeBySel = map[string]string{"iq0": "member-a"}
	svc := NewService(store, op, &fakeSubs{list: []subscription.Subscription{{
		ID:           "sub-1",
		SelectorTag:  "iq0",
		ActiveMember: "member-a",
		Enabled:      true,
		ListenPort:   1080,
		MemberTags:   []string{"member-a", "member-b"},
	}}}, nil)
	svc.clash = clash
	target, _ := svc.resolveTargetByConfig(context.Background(), cfg)
	svc.recoverSubscription(context.Background(), target, cfg)
	if op.restarts != 0 {
		t.Fatalf("restart count = %d, want 0", op.restarts)
	}
}

func TestService_RecoverSubscription_FailsWhenClashUnavailable(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "store.json"))
	cfg := TargetConfig{
		ID:            "subscription:sub-1",
		Kind:          TargetSubscription,
		Ref:           "sub-1",
		Enabled:       true,
		Interval:      30,
		FailThreshold: 1,
		Timeout:       1,
		RecoveryMode:  RecoverySwitchMember,
	}
	_ = store.Upsert(cfg)
	op := newInstalledOp()
	op.activeBySel = map[string]string{"iq0": "member-current"}
	svc := NewService(store, op, &fakeSubs{list: []subscription.Subscription{{
		ID:           "sub-1",
		SelectorTag:  "iq0",
		ActiveMember: "member-current",
		Enabled:      true,
		ListenPort:   1080,
		MemberTags:   []string{"member-current", "member-b"},
	}}}, nil)
	svc.clash = nil
	target, _ := svc.resolveTargetByConfig(context.Background(), cfg)
	svc.recoverSubscription(context.Background(), target, cfg)

	logs := svc.GetLogs("subscription:sub-1")
	if len(logs) == 0 || logs[len(logs)-1].Error != "clash api unavailable" {
		t.Fatalf("expected clash api unavailable log, got %+v", logs)
	}
}

func TestService_CheckSuccessDoesNotSpamProjectLog(t *testing.T) {
	rec := &recordingAppLogger{}
	svc := NewService(NewStore(filepath.Join(t.TempDir(), "store.json")), newInstalledOp(), &fakeSubs{}, rec)
	target := RuntimeTarget{ID: "tunnel:sb-main", Kind: TargetTunnel, Name: "sb-main", CheckTag: "sb-main", Running: true}
	cfg := normalizeConfig(TargetConfig{ID: target.ID, Kind: TargetTunnel, Ref: "sb-main", Enabled: true})
	svc.recordSuccess(target, cfg, CheckResult{Success: true, Latency: 42}, time.Now())
	if rec.count("recovered") != 0 {
		t.Fatalf("unexpected recovered log spam: %+v", rec.logs)
	}
}

func TestService_CheckFailureWritesWarnProjectLog(t *testing.T) {
	rec := &recordingAppLogger{}
	svc := NewService(NewStore(filepath.Join(t.TempDir(), "store.json")), newInstalledOp(), &fakeSubs{}, rec)
	target := RuntimeTarget{ID: "tunnel:sb-main", Kind: TargetTunnel, Name: "sb-main", CheckTag: "sb-main", Running: true}
	cfg := normalizeConfig(TargetConfig{ID: target.ID, Kind: TargetTunnel, Ref: "sb-main", Enabled: true})
	svc.recordFailure(target, cfg, CheckResult{Error: "timeout"}, time.Now())
	if !rec.has("check-failed", "tunnel:sb-main") {
		t.Fatal("expected check-failed project log")
	}
	if len(svc.GetLogs(target.ID)) == 0 {
		t.Fatal("expected domain log for failed check")
	}
}

func TestService_CheckOne_LocalProxyNotReadyOnFirstCheckStaysWarmingWithoutWarn(t *testing.T) {
	rec := &recordingAppLogger{}
	store := NewStore(filepath.Join(t.TempDir(), "store.json"))
	cfg := TargetConfig{
		ID:            "tunnel:sb-main",
		Kind:          TargetTunnel,
		Ref:           "sb-main",
		Enabled:       true,
		Interval:      30,
		FailThreshold: 3,
		Timeout:       1,
		RecoveryMode:  RecoveryRestartSingbox,
	}
	if err := store.Upsert(cfg); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	op := newInstalledOp()
	op.tunnels = []singbox.TunnelInfo{{
		Tag:        "sb-main",
		Running:    true,
		ListenPort: 1,
	}}
	svc := NewService(store, op, &fakeSubs{}, rec)
	svc.clash = nil

	svc.checkOne(context.Background(), cfg)

	if rec.has("check-failed", "tunnel:sb-main") {
		t.Fatalf("unexpected startup warning log: %+v", rec.logs)
	}
	statuses := svc.GetStatus(context.Background())
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].Status != StatusWarming {
		t.Fatalf("status = %q, want warming", statuses[0].Status)
	}
	if statuses[0].FailCount != 0 {
		t.Fatalf("failCount = %d, want 0", statuses[0].FailCount)
	}
	if statuses[0].LastError != "local proxy not ready" {
		t.Fatalf("lastError = %q, want local proxy not ready", statuses[0].LastError)
	}
	if statuses[0].LastCheck == nil {
		t.Fatal("lastCheck should be set after transient warming")
	}
	logs := svc.GetLogs("tunnel:sb-main")
	if len(logs) == 0 {
		t.Fatal("expected domain log entry")
	}
	if logs[len(logs)-1].FailCount != 0 || logs[len(logs)-1].Error != "local proxy not ready" {
		t.Fatalf("unexpected startup domain log: %+v", logs[len(logs)-1])
	}
}

func TestService_CheckOne_LocalProxyNotReadyAfterSuccessCountsAsRealFailure(t *testing.T) {
	rec := &recordingAppLogger{}
	store := NewStore(filepath.Join(t.TempDir(), "store.json"))
	cfg := TargetConfig{
		ID:            "tunnel:sb-main",
		Kind:          TargetTunnel,
		Ref:           "sb-main",
		Enabled:       true,
		Interval:      30,
		FailThreshold: 3,
		Timeout:       1,
		RecoveryMode:  RecoveryRestartSingbox,
	}
	if err := store.Upsert(cfg); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	op := newInstalledOp()
	op.tunnels = []singbox.TunnelInfo{{
		Tag:        "sb-main",
		Running:    true,
		ListenPort: 1,
	}}
	svc := NewService(store, op, &fakeSubs{}, rec)
	svc.clash = &fakeClash{delays: map[string]int{"sb-main": 42}}

	svc.checkOne(context.Background(), cfg)
	svc.clash = nil
	svc.checkOne(context.Background(), cfg)

	if !rec.has("check-failed", "tunnel:sb-main") {
		t.Fatalf("expected real failure warning after a prior success, logs=%+v", rec.logs)
	}
	statuses := svc.GetStatus(context.Background())
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].FailCount != 1 {
		t.Fatalf("failCount = %d, want 1", statuses[0].FailCount)
	}
	if statuses[0].LastCheck == nil {
		t.Fatal("expected lastCheck to be set after real failure")
	}
}
