package singboxwatchdog

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/hoaxisr/awg-manager/internal/events"
	"github.com/hoaxisr/awg-manager/internal/logging"
	"github.com/hoaxisr/awg-manager/internal/singbox"
	"github.com/hoaxisr/awg-manager/internal/singbox/subscription"
)

// defaultRestartCooldown is a global sing-box restart backoff shared by all
// raw tunnel watchdog targets. We keep it hardcoded in the first release so a
// user cannot accidentally set an unsafe value and create a restart loop.
const defaultRestartCooldown = 2 * time.Minute
const reconcileInterval = 30 * time.Second
const maxMemberChecks = 12
const selectorActiveCacheTTL = 5 * time.Second

var (
	ErrTargetNotFound = errors.New("watchdog target not found")
	ErrTargetMismatch = errors.New("watchdog target mismatch")
)

type SingboxOperator interface {
	GetStatus(ctx context.Context) singbox.Status
	ListTunnels(ctx context.Context) ([]singbox.TunnelInfo, error)
	Control(ctx context.Context, action string) error
	Clash() *singbox.ClashClient
	GetSelectorActive(ctx context.Context, selectorTag string) (string, error)
}

type SubscriptionProvider interface {
	List() []subscription.Subscription
}

type SubscriptionSwitcher interface {
	SetActiveMember(ctx context.Context, id, memberTag string) error
}

type ManualStopReader interface {
	IsSingboxManuallyStopped() bool
}

type targetRuntimeState struct {
	failCount         int
	restartCount      int
	switchCount       int
	lastCheck         *time.Time
	lastLatency       int
	lastError         string
	status            StatusKind
	lastRecovery      string
	recovering        bool
	recoveryStartedAt time.Time
}

type monitor struct {
	id     string
	stopCh chan struct{}
	wg     sync.WaitGroup
}

type selectorActiveCacheEntry struct {
	value     string
	expiresAt time.Time
}

type Service struct {
	store      *Store
	op         SingboxOperator
	subs       SubscriptionProvider
	subSwitch  SubscriptionSwitcher
	clash      Clash
	bus        *events.Bus
	log        *logging.ScopedLogger
	manualStop ManualStopReader

	mu                  sync.RWMutex
	monitors            map[string]*monitor
	state               map[string]*targetRuntimeState
	logs                *LogBuffer
	selectorActiveCache map[string]selectorActiveCacheEntry

	ctx     context.Context
	cancel  context.CancelFunc
	running bool

	restartMu       sync.Mutex
	lastRestartAt   time.Time
	restartCooldown time.Duration
}

func NewService(store *Store, op SingboxOperator, subs SubscriptionProvider, appLogger logging.AppLogger) *Service {
	var clash Clash
	if op != nil {
		if rawClash := op.Clash(); rawClash != nil {
			clash = rawClash
		}
	}
	var subSwitch SubscriptionSwitcher
	if subs != nil {
		if switcher, ok := subs.(SubscriptionSwitcher); ok {
			subSwitch = switcher
		}
	}
	return &Service{
		store:               store,
		op:                  op,
		subs:                subs,
		subSwitch:           subSwitch,
		clash:               clash,
		log:                 logging.NewScopedLogger(appLogger, logging.GroupSingbox, logging.SubSBWatchdog),
		monitors:            make(map[string]*monitor),
		state:               make(map[string]*targetRuntimeState),
		logs:                NewLogBuffer(500),
		selectorActiveCache: make(map[string]selectorActiveCacheEntry),
		restartCooldown:     defaultRestartCooldown,
	}
}

func (s *Service) SetEventBus(bus *events.Bus) { s.bus = bus }

func (s *Service) SetManualStopReader(reader ManualStopReader) { s.manualStop = reader }

func (s *Service) Start(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return
	}
	if ctx == nil {
		ctx = context.Background()
	}
	s.ctx, s.cancel = context.WithCancel(ctx)
	s.running = true
	s.logInfo("start", "", "Sing-box Watchdog service started")
	go s.Reconcile()
	go s.reconcileLoop()
}

func (s *Service) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	if s.cancel != nil {
		s.cancel()
	}
	monitors := make([]*monitor, 0, len(s.monitors))
	for _, m := range s.monitors {
		close(m.stopCh)
		monitors = append(monitors, m)
	}
	s.monitors = make(map[string]*monitor)
	s.mu.Unlock()
	for _, m := range monitors {
		m.wg.Wait()
	}
	s.logInfo("stop", "", "Sing-box Watchdog service stopped")
}

func (s *Service) Reconcile() {
	if s == nil || s.store == nil {
		return
	}
	cfgs := s.store.List()
	runtimeTargets := s.ResolveTargets(context.Background())
	available := make(map[string]bool, len(runtimeTargets))
	for _, target := range runtimeTargets {
		available[target.ID] = true
	}
	cfgByID := make(map[string]TargetConfig, len(cfgs))
	for _, cfg := range cfgs {
		cfgByID[cfg.ID] = normalizeConfig(cfg)
	}

	s.mu.Lock()
	for id, m := range s.monitors {
		cfg, ok := cfgByID[id]
		// Intentionally keep orphan configs in the store when a runtime target
		// disappears. Cleanup should be an explicit user action, not automatic GC.
		if !ok || !cfg.Enabled || !available[id] {
			close(m.stopCh)
			delete(s.monitors, id)
		}
	}
	for _, cfg := range cfgs {
		cfg = normalizeConfig(cfg)
		if !cfg.Enabled || !available[cfg.ID] {
			continue
		}
		if _, ok := s.monitors[cfg.ID]; ok {
			continue
		}
		m := &monitor{id: cfg.ID, stopCh: make(chan struct{})}
		m.wg.Add(1)
		s.monitors[cfg.ID] = m
		go s.runMonitorLoop(m)
	}
	s.mu.Unlock()
}

func (s *Service) reconcileLoop() {
	ticker := time.NewTicker(reconcileInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.Reconcile()
		}
	}
}

func (s *Service) GetStatus(ctx context.Context) []TargetStatus {
	targets := s.ResolveTargets(ctx)
	cfgs := s.store.List()
	cfgByID := make(map[string]TargetConfig, len(cfgs))
	for _, cfg := range cfgs {
		cfgByID[cfg.ID] = normalizeConfig(cfg)
	}

	out := make([]TargetStatus, 0, len(targets))
	s.mu.RLock()
	for _, target := range targets {
		state := s.state[target.ID]
		cfg, configured := cfgByID[target.ID]
		out = append(out, s.statusForTarget(target, state, cfg, configured))
	}
	s.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Service) statusForTarget(target RuntimeTarget, state *targetRuntimeState, cfg TargetConfig, configured bool) TargetStatus {
	status := TargetStatus{
		ID:              target.ID,
		Kind:            target.Kind,
		Ref:             target.Ref,
		Name:            target.Name,
		CheckTag:        target.CheckTag,
		TrafficTag:      target.TrafficTag,
		SelectorTag:     target.SelectorTag,
		ActiveMemberTag: target.ActiveMemberTag,
		Protocol:        target.Protocol,
		Security:        target.Security,
		Transport:       target.Transport,
		ProxyInterface:  target.ProxyInterface,
		KernelInterface: target.KernelInterface,
		Running:         target.Running,
		Configured:      configured,
		Enabled:         configured && cfg.Enabled,
		Status:          StatusDisabled,
		Interval:        cfg.Interval,
		Timeout:         cfg.Timeout,
		FailThreshold:   cfg.FailThreshold,
		RecoveryMode:    cfg.RecoveryMode,
		PersistSwitch:   cfg.PersistSwitch,
	}
	if !configured {
		return status
	}
	if !cfg.Enabled {
		status.Status = StatusDisabled
		return status
	}
	if !target.Running {
		status.Status = StatusStopped
	}
	if state != nil {
		status.LastCheck = state.lastCheck
		status.LastLatency = state.lastLatency
		status.FailCount = state.failCount
		status.RestartCount = state.restartCount
		status.SwitchCount = state.switchCount
		status.LastError = state.lastError
		status.LastRecovery = state.lastRecovery
		switch {
		case state.recovering:
			status.Status = StatusRecovering
		case !target.Running:
			status.Status = StatusStopped
		case state.lastCheck == nil:
			status.Status = StatusWarming
		case state.failCount >= cfg.FailThreshold:
			status.Status = StatusDead
		case state.lastError != "":
			status.Status = StatusWarming
		default:
			status.Status = StatusAlive
		}
	} else if target.Running {
		status.Status = StatusWarming
	}
	if configured && cfg.Enabled && target.Running && status.Status == StatusDisabled {
		status.Status = StatusWarming
	}
	return status
}

func (s *Service) GetLogs(targetID string) []LogEntry {
	if targetID == "" {
		return s.logs.GetAll()
	}
	return s.logs.GetByTarget(targetID)
}

func (s *Service) ClearLogs() {
	s.logs.Clear()
	s.publishSnapshot("logs-cleared")
}

func (s *Service) Configure(cfg TargetConfig) error {
	if s.store == nil {
		return errors.New("watchdog store not configured")
	}
	target, ok := s.resolveTargetByID(context.Background(), cfg.ID)
	if !ok {
		return ErrTargetNotFound
	}
	if target.Kind != cfg.Kind || target.Ref != cfg.Ref {
		return ErrTargetMismatch
	}
	if err := s.store.Upsert(cfg); err != nil {
		return err
	}
	s.Reconcile()
	normalized := normalizeConfig(cfg)
	s.logInfo("configure", cfg.ID, fmt.Sprintf(
		"configured target kind=%s ref=%s interval=%ds threshold=%d timeout=%ds recovery=%s",
		normalized.Kind, normalized.Ref, normalized.Interval, normalized.FailThreshold, normalized.Timeout, normalized.RecoveryMode,
	))
	s.publishSnapshot("configured")
	return nil
}

func (s *Service) Enable(id string) error {
	if err := s.store.SetEnabled(id, true); err != nil {
		return err
	}
	s.Reconcile()
	s.logInfo("enable", id, "watchdog enabled")
	s.publishSnapshot("enabled")
	return nil
}

func (s *Service) Disable(id string) error {
	if err := s.store.SetEnabled(id, false); err != nil {
		return err
	}
	s.mu.Lock()
	if st := s.state[id]; st != nil {
		st.recovering = false
	}
	s.mu.Unlock()
	s.Reconcile()
	s.logInfo("disable", id, "watchdog disabled")
	s.publishSnapshot("disabled")
	return nil
}

func (s *Service) CheckNow(_ context.Context, targetID string) error {
	if targetID == "" {
		s.CheckAllNow(nil)
		return nil
	}
	cfg, ok := s.store.Get(targetID)
	if !ok {
		return ErrTargetNotFound
	}
	runCtx := contextOrBackground(s.ctx)
	go s.checkOne(runCtx, normalizeConfig(cfg))
	return nil
}

func (s *Service) CheckAllNow(_ context.Context) {
	runCtx := contextOrBackground(s.ctx)
	for _, cfg := range s.store.List() {
		cfg = normalizeConfig(cfg)
		if !cfg.Enabled {
			continue
		}
		go s.checkOne(runCtx, cfg)
	}
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func (s *Service) ResolveTargets(ctx context.Context) []RuntimeTarget {
	if s.op == nil {
		return nil
	}
	tunnels, err := s.op.ListTunnels(ctx)
	if err != nil {
		return nil
	}
	cfgs := s.store.List()
	cfgByID := make(map[string]TargetConfig, len(cfgs))
	for _, cfg := range cfgs {
		cfgByID[cfg.ID] = normalizeConfig(cfg)
	}

	subs := []subscription.Subscription{}
	if s.subs != nil {
		subs = s.subs.List()
	}
	ownedTags := make(map[string]struct{})
	targets := make([]RuntimeTarget, 0, len(tunnels)+len(subs))
	sbStatus := s.op.GetStatus(ctx)
	for _, sub := range subs {
		memberTags := append([]string(nil), sub.MemberTags...)
		active := strings.TrimSpace(sub.ActiveMember)
		if sub.SelectorTag != "" {
			active = s.resolveSelectorActive(ctx, sub.SelectorTag, active)
		}
		if active == "" && len(sub.MemberTags) > 0 {
			active = sub.MemberTags[0]
		}
		var activeMember *subscription.MemberInfo
		for i := range sub.Members {
			if sub.Members[i].Tag == active {
				activeMember = &sub.Members[i]
				break
			}
		}
		name := strings.TrimSpace(sub.Label)
		if name == "" {
			switch {
			case sub.SelectorTag != "":
				name = sub.SelectorTag
			case activeMember != nil && strings.TrimSpace(activeMember.Label) != "":
				name = strings.TrimSpace(activeMember.Label)
			case active != "":
				name = active
			default:
				name = sub.ID
			}
		}
		checkTag := strings.TrimSpace(sub.SelectorTag)
		if checkTag == "" {
			checkTag = active
		}
		trafficTag := active
		proxyInterface := ""
		kernelInterface := ""
		if sub.ProxyIndex >= 0 {
			proxyInterface = fmt.Sprintf("Proxy%d", sub.ProxyIndex)
			kernelInterface = fmt.Sprintf("t2s%d", sub.ProxyIndex)
		}
		for _, tag := range memberTags {
			if tag != "" {
				ownedTags[tag] = struct{}{}
			}
		}
		if sub.SelectorTag != "" {
			ownedTags[sub.SelectorTag] = struct{}{}
		}
		target := RuntimeTarget{
			ID:              "subscription:" + sub.ID,
			Kind:            TargetSubscription,
			Ref:             sub.ID,
			Name:            name,
			CheckTag:        checkTag,
			TrafficTag:      trafficTag,
			SelectorTag:     strings.TrimSpace(sub.SelectorTag),
			ActiveMemberTag: active,
			MemberTags:      memberTags,
			ListenPort:      int(sub.ListenPort),
			ProxyInterface:  proxyInterface,
			KernelInterface: kernelInterface,
			Running:         sbStatus.Running && sub.Enabled,
		}
		if activeMember != nil {
			target.Protocol = activeMember.Protocol
			target.Security = activeMember.Security
			target.Transport = activeMember.Transport
		}
		if cfg, ok := cfgByID[target.ID]; ok {
			cfgCopy := cfg
			target.Config = &cfgCopy
		}
		targets = append(targets, target)
	}

	for _, tunnel := range tunnels {
		if _, hidden := ownedTags[tunnel.Tag]; hidden {
			continue
		}
		target := RuntimeTarget{
			ID:              "tunnel:" + tunnel.Tag,
			Kind:            TargetTunnel,
			Ref:             tunnel.Tag,
			Name:            tunnel.Tag,
			CheckTag:        tunnel.Tag,
			TrafficTag:      tunnel.Tag,
			ListenPort:      tunnel.ListenPort,
			Protocol:        tunnel.Protocol,
			Security:        tunnel.Security,
			Transport:       tunnel.Transport,
			ProxyInterface:  tunnel.ProxyInterface,
			KernelInterface: tunnel.KernelInterface,
			Running:         tunnel.Running,
		}
		if cfg, ok := cfgByID[target.ID]; ok {
			cfgCopy := cfg
			target.Config = &cfgCopy
		}
		targets = append(targets, target)
	}

	sort.Slice(targets, func(i, j int) bool { return targets[i].ID < targets[j].ID })
	return targets
}

func (s *Service) resolveTargetByConfig(ctx context.Context, cfg TargetConfig) (RuntimeTarget, bool) {
	return s.resolveTargetByID(ctx, cfg.ID)
}

func (s *Service) resolveTargetByID(ctx context.Context, id string) (RuntimeTarget, bool) {
	for _, target := range s.ResolveTargets(ctx) {
		if target.ID == id {
			return target, true
		}
	}
	return RuntimeTarget{}, false
}

func (s *Service) resolveSelectorActive(ctx context.Context, selectorTag, fallback string) string {
	selectorTag = strings.TrimSpace(selectorTag)
	fallback = strings.TrimSpace(fallback)
	if selectorTag == "" || s.op == nil {
		return fallback
	}

	now := time.Now()
	s.mu.RLock()
	cached, ok := s.selectorActiveCache[selectorTag]
	s.mu.RUnlock()
	if ok && now.Before(cached.expiresAt) && strings.TrimSpace(cached.value) != "" {
		return cached.value
	}

	live, err := s.op.GetSelectorActive(ctx, selectorTag)
	live = strings.TrimSpace(live)
	if err == nil && live != "" {
		s.setSelectorActiveCache(selectorTag, live)
		return live
	}

	if ok && strings.TrimSpace(cached.value) != "" {
		return cached.value
	}
	return fallback
}

func (s *Service) setSelectorActiveCache(selectorTag, active string) {
	selectorTag = strings.TrimSpace(selectorTag)
	active = strings.TrimSpace(active)
	if selectorTag == "" || active == "" {
		return
	}
	s.mu.Lock()
	s.selectorActiveCache[selectorTag] = selectorActiveCacheEntry{
		value:     active,
		expiresAt: time.Now().Add(selectorActiveCacheTTL),
	}
	s.mu.Unlock()
}

func (s *Service) runMonitorLoop(m *monitor) {
	defer m.wg.Done()
	ctx := contextOrBackground(s.ctx)
	done := ctx.Done()
	for {
		cfg, ok := s.store.Get(m.id)
		if !ok || !cfg.Enabled {
			return
		}
		cfg = normalizeConfig(cfg)
		s.checkOne(ctx, cfg)
		timer := time.NewTimer(time.Duration(cfg.Interval) * time.Second)
		select {
		case <-timer.C:
		case <-m.stopCh:
			timer.Stop()
			return
		case <-done:
			timer.Stop()
			return
		}
	}
}

func (s *Service) checkOne(ctx context.Context, cfg TargetConfig) {
	target, ok := s.resolveTargetByConfig(ctx, cfg)
	if !ok {
		s.recordMissing(cfg)
		return
	}

	res := CheckTarget(ctx, s.clash, target, time.Duration(cfg.Timeout)*time.Second)
	now := time.Now()
	if res.Success {
		s.recordSuccess(target, cfg, res, now)
		return
	}
	if s.shouldTreatAsStartupWarming(target.ID, res.Error) {
		s.recordTransientWarming(target, cfg, now)
		return
	}

	failCount, shouldRecover := s.recordFailure(target, cfg, res, now)
	if !shouldRecover || failCount < cfg.FailThreshold {
		return
	}

	switch {
	case cfg.Kind == TargetTunnel && cfg.RecoveryMode == RecoveryRestartSingbox:
		go s.recoverRawTunnel(contextOrBackground(s.ctx), target, cfg)
	case cfg.Kind == TargetSubscription && cfg.RecoveryMode == RecoverySwitchMember:
		go s.recoverSubscription(contextOrBackground(s.ctx), target, cfg)
	}
}

func isLocalProxyNotReadyError(errMsg string) bool {
	errMsg = strings.ToLower(strings.TrimSpace(errMsg))
	return strings.Contains(errMsg, "proxyconnect tcp: dial tcp 127.0.0.1:") &&
		strings.Contains(errMsg, "connect: connection refused")
}

func (s *Service) shouldTreatAsStartupWarming(targetID, errMsg string) bool {
	if !isLocalProxyNotReadyError(errMsg) {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	state := s.state[targetID]
	return state == nil || state.lastCheck == nil
}

func (s *Service) recordTransientWarming(target RuntimeTarget, cfg TargetConfig, now time.Time) {
	s.mu.Lock()
	state := s.ensureStateLocked(target.ID)
	state.lastCheck = &now
	state.lastLatency = 0
	state.lastError = "local proxy not ready"
	state.status = StatusWarming
	s.mu.Unlock()

	s.addLogEntry(LogEntry{
		Timestamp:  now,
		TargetID:   target.ID,
		TargetName: target.Name,
		Kind:       target.Kind,
		CheckTag:   target.CheckTag,
		Success:    false,
		Error:      "local proxy not ready",
		FailCount:  0,
		Threshold:  cfg.FailThreshold,
	})
}

func (s *Service) recordMissing(cfg TargetConfig) {
	now := time.Now()
	failCount := 0
	s.mu.Lock()
	state := s.ensureStateLocked(cfg.ID)
	state.lastCheck = &now
	state.lastError = "target not found"
	state.failCount++
	failCount = state.failCount
	s.mu.Unlock()
	s.addLogEntry(LogEntry{
		Timestamp:  now,
		TargetID:   cfg.ID,
		TargetName: cfg.Ref,
		Kind:       cfg.Kind,
		Success:    false,
		Error:      "target not found",
		FailCount:  failCount,
		Threshold:  cfg.FailThreshold,
	})
	if failCount == 1 {
		s.logWarn("check-failed", cfg.ID, "check failed target not found")
	}
}

func (s *Service) recordSuccess(target RuntimeTarget, cfg TargetConfig, res CheckResult, now time.Time) {
	var stateChange string
	var previousFailCount int
	s.mu.Lock()
	state := s.ensureStateLocked(target.ID)
	previousFailCount = state.failCount
	if state.recovering || state.failCount >= cfg.FailThreshold {
		stateChange = "recovered"
		state.lastRecovery = "recovered"
	}
	state.recovering = false
	state.failCount = 0
	state.lastCheck = &now
	state.lastLatency = res.Latency
	state.lastError = ""
	state.status = StatusAlive
	s.mu.Unlock()

	s.addLogEntry(LogEntry{
		Timestamp:   now,
		TargetID:    target.ID,
		TargetName:  target.Name,
		Kind:        target.Kind,
		CheckTag:    target.CheckTag,
		Success:     true,
		Latency:     res.Latency,
		Threshold:   cfg.FailThreshold,
		StateChange: stateChange,
	})
	if previousFailCount > 0 {
		s.logInfo("recovered", target.ID, fmt.Sprintf(
			"target recovered checkTag=%s latency=%dms",
			target.CheckTag, res.Latency,
		))
	}
}

func (s *Service) recordFailure(target RuntimeTarget, cfg TargetConfig, res CheckResult, now time.Time) (int, bool) {
	s.mu.Lock()
	state := s.ensureStateLocked(target.ID)
	state.failCount++
	state.lastCheck = &now
	state.lastLatency = res.Latency
	state.lastError = res.Error
	failCount := state.failCount
	alreadyRecovering := state.recovering
	if !alreadyRecovering && failCount >= cfg.FailThreshold {
		state.recovering = true
		state.status = StatusRecovering
	} else if failCount >= cfg.FailThreshold {
		state.status = StatusDead
	}
	s.mu.Unlock()

	s.addLogEntry(LogEntry{
		Timestamp:  now,
		TargetID:   target.ID,
		TargetName: target.Name,
		Kind:       target.Kind,
		CheckTag:   target.CheckTag,
		Success:    false,
		Latency:    res.Latency,
		Error:      res.Error,
		FailCount:  failCount,
		Threshold:  cfg.FailThreshold,
	})
	s.logWarn("check-failed", target.ID, fmt.Sprintf(
		"check failed checkTag=%s fail=%d/%d error=%s",
		target.CheckTag, failCount, cfg.FailThreshold, res.Error,
	))
	if failCount >= cfg.FailThreshold {
		s.logWarn("threshold-reached", target.ID, fmt.Sprintf(
			"threshold reached checkTag=%s fail=%d/%d",
			target.CheckTag, failCount, cfg.FailThreshold,
		))
	}
	return failCount, !alreadyRecovering
}

func (s *Service) canRestartSingbox() bool {
	s.restartMu.Lock()
	defer s.restartMu.Unlock()
	if time.Since(s.lastRestartAt) < s.restartCooldown {
		return false
	}
	s.lastRestartAt = time.Now()
	return true
}

func (s *Service) recoverRawTunnel(ctx context.Context, target RuntimeTarget, cfg TargetConfig) {
	if cfg.RecoveryMode != RecoveryRestartSingbox {
		s.markRecoverySkipped(target, cfg, "recovery disabled", StatusDead)
		return
	}
	if !s.beginRecovery(target.ID) {
		return
	}
	defer s.endRecovery(target.ID)
	if s.shouldSkipRecovery(ctx, target, cfg) {
		return
	}
	if !s.canRestartSingbox() {
		s.logWarn("restart-skipped", target.ID, "restart cooldown active")
		s.markRecoverySkipped(target, cfg, "restart cooldown active", StatusDead)
		return
	}
	s.logWarn("restart-singbox", target.ID, fmt.Sprintf(
		"threshold reached for raw target checkTag=%s; restarting sing-box",
		target.CheckTag,
	))
	s.addLogEntry(LogEntry{
		Timestamp:   time.Now(),
		TargetID:    target.ID,
		TargetName:  target.Name,
		Kind:        target.Kind,
		CheckTag:    target.CheckTag,
		Success:     false,
		FailCount:   cfg.FailThreshold,
		Threshold:   cfg.FailThreshold,
		StateChange: "restart",
		Error:       "restarting sing-box after watchdog threshold",
	})
	if err := s.op.Control(ctx, "restart"); err != nil {
		s.logWarn("restart-failed", target.ID, "sing-box restart did not recover target: "+err.Error())
		s.markRecoveryFailed(target, cfg, err.Error())
		return
	}
	if !s.waitSingboxRunning(ctx, 20*time.Second) {
		s.logWarn("restart-failed", target.ID, "sing-box restart did not recover target: sing-box did not become running")
		s.markRecoveryFailed(target, cfg, "sing-box did not become running")
		return
	}
	if !sleepContext(ctx, 1500*time.Millisecond) {
		return
	}
	verifyTarget, ok := s.resolveTargetByConfig(ctx, cfg)
	if !ok {
		s.markRecoveryFailed(target, cfg, "target disappeared after restart")
		return
	}
	res := CheckTarget(ctx, s.clash, verifyTarget, time.Duration(cfg.Timeout)*time.Second)
	if res.Success {
		s.mu.Lock()
		state := s.ensureStateLocked(target.ID)
		state.restartCount++
		s.mu.Unlock()
		s.recordSuccess(verifyTarget, cfg, res, time.Now())
		s.logInfo("restart-recovered", target.ID, "sing-box restart recovered target")
		s.publishSnapshot("recovery")
		return
	}
	s.logWarn("restart-failed", target.ID, "sing-box restart did not recover target: "+res.Error)
	s.markRecoveryFailed(verifyTarget, cfg, res.Error)
}

func (s *Service) recoverSubscription(ctx context.Context, target RuntimeTarget, cfg TargetConfig) {
	if cfg.RecoveryMode != RecoverySwitchMember {
		s.markRecoverySkipped(target, cfg, "recovery disabled", StatusDead)
		return
	}
	if !s.beginRecovery(target.ID) {
		return
	}
	defer s.endRecovery(target.ID)
	if s.shouldSkipRecovery(ctx, target, cfg) {
		return
	}
	if target.SelectorTag == "" {
		s.markRecoveryFailed(target, cfg, "subscription has no selectorTag")
		return
	}
	if s.clash == nil {
		s.markRecoveryFailed(target, cfg, "clash api unavailable")
		return
	}

	current := target.ActiveMemberTag
	if active, err := s.clash.SelectorActive(target.SelectorTag); err == nil && strings.TrimSpace(active) != "" {
		current = strings.TrimSpace(active)
		s.setSelectorActiveCache(target.SelectorTag, current)
	}
	type candidate struct {
		tag   string
		delay int
	}
	var best *candidate
	tested := 0
	for _, tag := range target.MemberTags {
		tag = strings.TrimSpace(tag)
		if !isProbeableSubscriptionMemberTag(tag, current, target.SelectorTag) {
			continue
		}
		if tested >= maxMemberChecks {
			break
		}
		tested++
		delay, err := s.clash.TestDelay(tag, defaultTestURL, time.Duration(cfg.Timeout)*time.Second)
		if err != nil || delay <= 0 {
			continue
		}
		if best == nil || delay < best.delay {
			best = &candidate{tag: tag, delay: delay}
		}
	}
	if best == nil {
		s.logWarn("member-switch-failed", target.ID, fmt.Sprintf(
			"no healthy subscription members selector=%s tested=%d",
			target.SelectorTag, tested,
		))
		s.markRecoveryFailed(target, cfg, "no healthy subscription members")
		return
	}
	if err := s.clash.SetSelector(target.SelectorTag, best.tag); err != nil {
		s.logWarn("member-switch-unsupported", target.ID, "selector switch unsupported: "+err.Error())
		s.markRecoveryFailed(target, cfg, "selector switch unsupported: "+err.Error())
		return
	}
	s.setSelectorActiveCache(target.SelectorTag, best.tag)
	s.logWarn("member-switch", target.ID, fmt.Sprintf(
		"switching subscription selector=%s from=%s to=%s",
		target.SelectorTag, current, best.tag,
	))
	if !sleepContext(ctx, 500*time.Millisecond) {
		return
	}
	verifyTarget, ok := s.resolveTargetByConfig(ctx, cfg)
	if !ok {
		s.markRecoveryFailed(target, cfg, "target disappeared after selector switch")
		return
	}
	verifyTarget.ActiveMemberTag = best.tag
	verifyTarget.TrafficTag = best.tag
	res := CheckTarget(ctx, s.clash, verifyTarget, time.Duration(cfg.Timeout)*time.Second)
	if !res.Success {
		s.markRecoveryFailed(verifyTarget, cfg, res.Error)
		return
	}
	if cfg.PersistSwitch {
		if s.subSwitch == nil {
			s.markRecoveryFailed(verifyTarget, cfg, "subscription persistence unavailable")
			return
		}
		if err := s.subSwitch.SetActiveMember(ctx, target.Ref, best.tag); err != nil {
			s.markRecoveryFailed(verifyTarget, cfg, "persist selector switch: "+err.Error())
			return
		}
	}
	s.mu.Lock()
	state := s.ensureStateLocked(target.ID)
	state.switchCount++
	state.recovering = false
	state.failCount = 0
	state.lastCheck = timePtr(time.Now())
	state.lastLatency = res.Latency
	state.lastError = ""
	state.lastRecovery = "member_switch"
	state.status = StatusAlive
	s.mu.Unlock()
	s.addLogEntry(LogEntry{
		Timestamp:   time.Now(),
		TargetID:    verifyTarget.ID,
		TargetName:  verifyTarget.Name,
		Kind:        verifyTarget.Kind,
		CheckTag:    verifyTarget.CheckTag,
		Success:     true,
		Latency:     res.Latency,
		FailCount:   0,
		Threshold:   cfg.FailThreshold,
		StateChange: "member_switch",
		MemberFrom:  current,
		MemberTo:    best.tag,
	})
	s.logInfo("member-switch-recovered", target.ID, fmt.Sprintf(
		"subscription recovered selector=%s from=%s to=%s latency=%dms",
		target.SelectorTag, current, best.tag, res.Latency,
	))
	s.publishSnapshot("recovery")
}

func isProbeableSubscriptionMemberTag(tag, current, selectorTag string) bool {
	tag = strings.TrimSpace(tag)
	if tag == "" || tag == current || tag == selectorTag {
		return false
	}

	switch strings.ToLower(tag) {
	case "direct", "block", "final":
		return false
	default:
		return true
	}
}

func timePtr(t time.Time) *time.Time { return &t }

func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (s *Service) waitSingboxRunning(ctx context.Context, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		if s.op.GetStatus(ctx).Running {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
		}
	}
}

func (s *Service) markRecoveryFailed(target RuntimeTarget, cfg TargetConfig, errMsg string) {
	now := time.Now()
	s.mu.Lock()
	state := s.ensureStateLocked(target.ID)
	state.recovering = false
	state.lastCheck = &now
	state.lastError = errMsg
	state.lastRecovery = "recovery_failed"
	if state.failCount < cfg.FailThreshold {
		state.failCount = cfg.FailThreshold
	}
	state.status = StatusDead
	s.mu.Unlock()
	s.addLogEntry(LogEntry{
		Timestamp:   now,
		TargetID:    target.ID,
		TargetName:  target.Name,
		Kind:        target.Kind,
		CheckTag:    target.CheckTag,
		Success:     false,
		Error:       errMsg,
		FailCount:   cfg.FailThreshold,
		Threshold:   cfg.FailThreshold,
		StateChange: "recovery_failed",
	})
	s.logWarn("recovery-failed", target.ID, errMsg)
	s.publishSnapshot("recovery")
}

func (s *Service) ensureStateLocked(id string) *targetRuntimeState {
	state, ok := s.state[id]
	if !ok {
		state = &targetRuntimeState{}
		s.state[id] = state
	}
	return state
}

func (s *Service) addLogEntry(entry LogEntry) {
	s.logs.Add(entry)
	if s.bus == nil {
		return
	}
	s.bus.Publish("singbox:watchdog-log", entry)
}

func (s *Service) publishSnapshot(reason string) {
	if s.bus == nil {
		return
	}
	s.bus.Publish("resource:invalidated", events.ResourceInvalidatedEvent{
		Resource: "singbox.watchdog",
		Reason:   reason,
	})
}

func (s *Service) beginRecovery(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.ensureStateLocked(id)
	if state.recovering && !state.recoveryStartedAt.IsZero() {
		return false
	}
	state.recovering = true
	state.recoveryStartedAt = time.Now()
	state.status = StatusRecovering
	return true
}

func (s *Service) endRecovery(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if state := s.state[id]; state != nil {
		state.recovering = false
		state.recoveryStartedAt = time.Time{}
	}
}

func (s *Service) shouldSkipRecovery(ctx context.Context, target RuntimeTarget, cfg TargetConfig) bool {
	status := s.op.GetStatus(ctx)
	if !status.Installed {
		s.logWarn("recovery-skipped", target.ID, "sing-box not installed; watchdog recovery skipped")
		s.markRecoverySkipped(target, cfg, "sing-box not installed; watchdog recovery skipped", StatusStopped)
		return true
	}
	if s.manualStop != nil && s.manualStop.IsSingboxManuallyStopped() {
		s.logWarn("recovery-skipped", target.ID, "sing-box manually stopped; watchdog recovery skipped")
		s.markRecoverySkipped(target, cfg, "sing-box manually stopped; watchdog recovery skipped", StatusStopped)
		return true
	}
	return false
}

func (s *Service) markRecoverySkipped(target RuntimeTarget, cfg TargetConfig, errMsg string, status StatusKind) {
	now := time.Now()
	s.mu.Lock()
	state := s.ensureStateLocked(target.ID)
	state.lastCheck = &now
	state.lastError = errMsg
	state.lastRecovery = "recovery_skipped"
	if state.failCount < cfg.FailThreshold {
		state.failCount = cfg.FailThreshold
	}
	state.status = status
	s.mu.Unlock()
	s.addLogEntry(LogEntry{
		Timestamp:   now,
		TargetID:    target.ID,
		TargetName:  target.Name,
		Kind:        target.Kind,
		CheckTag:    target.CheckTag,
		Success:     false,
		Error:       errMsg,
		FailCount:   cfg.FailThreshold,
		Threshold:   cfg.FailThreshold,
		StateChange: "recovery_skipped",
	})
	s.publishSnapshot("recovery")
}

func (s *Service) logInfo(action, target, message string) {
	if s.log != nil {
		s.log.Info(action, target, message)
	}
}

func (s *Service) logWarn(action, target, message string) {
	if s.log != nil {
		s.log.Warn(action, target, message)
	}
}
