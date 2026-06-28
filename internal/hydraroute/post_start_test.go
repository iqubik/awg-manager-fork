package hydraroute

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hoaxisr/awg-manager/internal/logging"
	ndmsquery "github.com/hoaxisr/awg-manager/internal/ndms/query"
)

type fakeHydraAppLogger struct {
	mu      sync.Mutex
	entries []fakeHydraLogEntry
}

type fakeHydraLogEntry struct {
	level    logging.Level
	group    string
	subgroup string
	action   string
	target   string
	message  string
}

func (f *fakeHydraAppLogger) AppLog(level logging.Level, group, subgroup, action, target, message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entries = append(f.entries, fakeHydraLogEntry{
		level:    level,
		group:    group,
		subgroup: subgroup,
		action:   action,
		target:   target,
		message:  message,
	})
}

func (f *fakeHydraAppLogger) has(action string, level logging.Level, contains string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, entry := range f.entries {
		if entry.action != action || entry.level != level {
			continue
		}
		if contains == "" || strings.Contains(entry.message, contains) {
			return true
		}
	}
	return false
}

func (f *fakeHydraAppLogger) hasScoped(action string, level logging.Level, group string, subgroup string, target string, contains string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, entry := range f.entries {
		if entry.action != action || entry.level != level {
			continue
		}
		if entry.group != group || entry.subgroup != subgroup {
			continue
		}
		if target != "" && entry.target != target {
			continue
		}
		if contains == "" || strings.Contains(entry.message, contains) {
			return true
		}
	}
	return false
}

func (f *fakeHydraAppLogger) count(action string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, entry := range f.entries {
		if entry.action == action {
			n++
		}
	}
	return n
}

func postStartPendingLen(s *Service) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.postStartPending)
}

func postStartRunning(s *Service) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.postStartRunning
}

func postStartTimerArmed(s *Service) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.postStartTimer != nil
}

func TestScheduleRestartAfterInterfaceOnline_SkipsWithoutRelevantRules(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	tunePostStartForTest(svc)

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")

	if stopPostStartTimer(svc) {
		t.Fatalf("post-start timer scheduled without relevant HR rules")
	}
	if got := postStartPendingLen(svc); got != 0 {
		t.Fatalf("pending interfaces should stay empty, got %d", got)
	}
}

func TestScheduleRestartAfterInterfaceOnline_SkipsDisabledRule(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	tunePostStartForTest(svc)
	if _, err := svc.CreateRule(HRRule{Name: "Youtube", Domains: []string{"youtube.com"}, Target: "nwg0", Disabled: true}); err != nil {
		t.Fatalf("CreateRule: %v", err)
	}
	clearRuleWriteRestart(svc)

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")

	if stopPostStartTimer(svc) {
		t.Fatalf("post-start timer scheduled for disabled rule")
	}
}

func TestScheduleRestartAfterInterfaceOnline_DebouncesToSingleRestart(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	createEnabledRule(t, svc, "Youtube", "nwg0")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)
	svc.postStartReadyProbe = func(_ context.Context, ndmsIface string) (bool, string, error) {
		return ndmsIface == "Wireguard0", "connected=yes conf=running link=up state=up", nil
	}

	var restarts atomic.Int32
	done := make(chan struct{}, 1)
	svc.postStartRestart = func(_ context.Context) error {
		if restarts.Add(1) == 1 {
			done <- struct{}{}
		}
		return nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "wan-up-start", "Wireguard0", "nwg0")

	waitForSignal(t, done, "timed out waiting for debounced post-start restart")
	if got := restarts.Load(); got != 1 {
		t.Fatalf("expected 1 debounced restart, got %d", got)
	}
}

func TestScheduleRestartAfterInterfaceOnline_CancelledCtxDoesNotCancelBackgroundWait(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	createEnabledRule(t, svc, "Youtube", "nwg0")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)

	var ready atomic.Bool
	svc.postStartReadyProbe = func(_ context.Context, _ string) (bool, string, error) {
		return ready.Load(), "connected=yes conf=running link=up state=up", nil
	}

	done := make(chan struct{}, 1)
	svc.postStartRestart = func(_ context.Context) error {
		done <- struct{}{}
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc.ScheduleRestartAfterInterfaceOnline(ctx, "manual-start", "Wireguard0", "nwg0")
	time.Sleep(10 * time.Millisecond)
	ready.Store(true)

	waitForSignal(t, done, "cancelled ctx should not cancel post-start restart")
}

func TestScheduleRestartAfterInterfaceOnline_WaitsForAllPendingInterfaces(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	createEnabledRule(t, svc, "Rule0", "nwg0")
	createEnabledRule(t, svc, "Rule1", "nwg1")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)

	var mu sync.Mutex
	ready := map[string]bool{
		"Wireguard0": true,
		"Wireguard1": false,
	}
	restartTimes := make(chan time.Time, 1)
	svc.postStartReadyProbe = func(_ context.Context, ndmsIface string) (bool, string, error) {
		mu.Lock()
		defer mu.Unlock()
		return ready[ndmsIface], "connected=yes conf=running link=up state=up", nil
	}
	svc.postStartRestart = func(_ context.Context) error {
		restartTimes <- time.Now()
		return nil
	}

	start := time.Now()
	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard1", "nwg1")

	time.Sleep(30 * time.Millisecond)
	select {
	case <-restartTimes:
		t.Fatal("restart fired before all pending interfaces were ready")
	default:
	}

	mu.Lock()
	ready["Wireguard1"] = true
	mu.Unlock()

	select {
	case ts := <-restartTimes:
		if ts.Sub(start) < 30*time.Millisecond {
			t.Fatalf("restart happened too early after %s", ts.Sub(start))
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("timed out waiting for multi-interface restart")
	}
}

func TestScheduleRestartAfterInterfaceOnline_TimeoutStopsWithoutRestart(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	createEnabledRule(t, svc, "Youtube", "nwg0")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)

	var restarts atomic.Int32
	done := make(chan struct{}, 1)
	svc.postStartReadyProbe = func(_ context.Context, _ string) (bool, string, error) {
		return false, "not-ready", nil
	}
	svc.postStartRestart = func(_ context.Context) error {
		restarts.Add(1)
		done <- struct{}{}
		return nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	time.Sleep(120 * time.Millisecond)

	select {
	case <-done:
		t.Fatal("restart should not run on timeout when no interface became ready")
	default:
	}
	if got := restarts.Load(); got != 0 {
		t.Fatalf("expected 0 restarts on timeout, got %d", got)
	}
	if postStartRunning(svc) {
		t.Fatal("post-start cycle should stop after timeout")
	}
	if got := postStartPendingLen(svc); got != 0 {
		t.Fatalf("pending interfaces should be cleared after timeout, got %d", got)
	}
}

func TestScheduleRestartAfterInterfaceOnline_PolicyFallbackWithoutQueriesRestarts(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	createEnabledRule(t, svc, "PolicyRule", "HydraRoute")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)
	svc.SetQueries(&ndmsquery.Queries{})

	done := make(chan struct{}, 1)
	svc.postStartReadyProbe = func(_ context.Context, _ string) (bool, string, error) {
		return true, "connected=yes conf=running link=up state=up", nil
	}
	svc.postStartRestart = func(_ context.Context) error {
		done <- struct{}{}
		return nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	waitForSignal(t, done, "policy fallback without queries should still restart")
}

func TestScheduleRestartAfterInterfaceOnline_PolicyQueryErrorFallsBackToRestart(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	createEnabledRule(t, svc, "PolicyRule", "HydraRoute")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)

	fg := ndmsquery.NewFakeGetter()
	fg.SetDefaultError(errors.New("policy list failed"))
	svc.SetQueries(&ndmsquery.Queries{
		Policies: ndmsquery.NewPolicyStore(fg, ndmsquery.NopLogger()),
	})

	done := make(chan struct{}, 1)
	svc.postStartReadyProbe = func(_ context.Context, _ string) (bool, string, error) {
		return true, "connected=yes conf=running link=up state=up", nil
	}
	svc.postStartRestart = func(_ context.Context) error {
		done <- struct{}{}
		return nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	waitForSignal(t, done, "policy query error should fall back to safe restart")
}

func TestScheduleRestartAfterInterfaceOnline_CooldownIsSeparateFromTimeout(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	createEnabledRule(t, svc, "Youtube", "nwg0")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)
	svc.postStartCooldown = 40 * time.Millisecond
	svc.postStartReadyProbe = func(_ context.Context, _ string) (bool, string, error) {
		return true, "connected=yes conf=running link=up state=up", nil
	}

	var restarts atomic.Int32
	done := make(chan struct{}, 3)
	svc.postStartRestart = func(_ context.Context) error {
		restarts.Add(1)
		done <- struct{}{}
		return nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	waitForSignal(t, done, "first restart did not happen")

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "wan-up-start", "Wireguard0", "nwg0")
	time.Sleep(20 * time.Millisecond)
	if got := restarts.Load(); got != 1 {
		t.Fatalf("restart should be suppressed during cooldown, got %d", got)
	}

	time.Sleep(50 * time.Millisecond)
	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "wan-up-start", "Wireguard0", "nwg0")
	waitForSignal(t, done, "second restart after cooldown did not happen")
	if got := restarts.Load(); got != 2 {
		t.Fatalf("expected 2 restarts after cooldown expiry, got %d", got)
	}
}

func TestScheduleRestartAfterInterfaceOnline_TimeoutWithAnyReadyUsesFreshRestartContext(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	createEnabledRule(t, svc, "Rule0", "nwg0")
	createEnabledRule(t, svc, "Rule1", "nwg1")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)
	svc.postStartTimeout = 40 * time.Millisecond

	svc.postStartReadyProbe = func(_ context.Context, ndmsIface string) (bool, string, error) {
		return ndmsIface == "Wireguard0", "connected=yes conf=running link=up state=up", nil
	}

	done := make(chan struct{}, 1)
	svc.postStartRestart = func(ctx context.Context) error {
		if err := ctx.Err(); err != nil {
			t.Fatalf("restart got cancelled ctx: %v", err)
		}
		done <- struct{}{}
		return nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard1", "nwg1")

	waitForSignal(t, done, "timeout best-effort restart did not happen")
}

func TestScheduleRestartAfterInterfaceOnline_NewPendingDuringReadyDelayIsNotDropped(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	createEnabledRule(t, svc, "Rule0", "nwg0")
	createEnabledRule(t, svc, "Rule1", "nwg1")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)
	svc.postStartReadyDelay = 40 * time.Millisecond

	var mu sync.Mutex
	ready := map[string]bool{
		"Wireguard0": true,
		"Wireguard1": true,
	}
	svc.postStartReadyProbe = func(_ context.Context, ndmsIface string) (bool, string, error) {
		mu.Lock()
		defer mu.Unlock()
		return ready[ndmsIface], "connected=yes conf=running link=up state=up", nil
	}

	var restarts atomic.Int32
	done := make(chan struct{}, 2)
	svc.postStartRestart = func(_ context.Context) error {
		restarts.Add(1)
		done <- struct{}{}
		return nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	ready["Wireguard1"] = false
	mu.Unlock()
	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard1", "nwg1")

	waitForSignal(t, done, "first restart did not happen")
	if postStartRunning(svc) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := restarts.Load(); got != 1 {
		t.Fatalf("expected first restart only, got %d", got)
	}
	if postStartPendingLen(svc) == 0 {
		t.Fatal("new pending interface was dropped during readyDelay")
	}

	mu.Lock()
	ready["Wireguard1"] = true
	mu.Unlock()
	waitForSignal(t, done, "second restart for carried pending interface did not happen")
	if got := restarts.Load(); got != 2 {
		t.Fatalf("expected 2 restarts after carrying pending interface, got %d", got)
	}
}

func TestHasRelevantPostStartRules_LowercasePolicyTargetFallsBackSafely(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	createEnabledRule(t, svc, "PolicyRule", "vpnpolicy")
	clearRuleWriteRestart(svc)
	svc.SetQueries(&ndmsquery.Queries{})

	relevant, err := svc.hasRelevantPostStartRules(context.Background(), "Wireguard0", "nwg0")
	if err != nil {
		t.Fatalf("hasRelevantPostStartRules: %v", err)
	}
	if !relevant {
		t.Fatal("lowercase policy target should fall back to safe restart")
	}
}

func TestScheduleRestartAfterInterfaceOnline_CooldownQueuesInsteadOfDropping(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	createEnabledRule(t, svc, "Youtube", "nwg0")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)
	svc.postStartCooldown = 40 * time.Millisecond
	svc.postStartReadyProbe = func(_ context.Context, _ string) (bool, string, error) {
		return true, "connected=yes conf=running link=up state=up", nil
	}

	var restarts atomic.Int32
	done := make(chan struct{}, 2)
	svc.postStartRestart = func(_ context.Context) error {
		restarts.Add(1)
		done <- struct{}{}
		return nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	waitForSignal(t, done, "first restart did not happen")

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "wan-up-start", "Wireguard0", "nwg0")
	time.Sleep(20 * time.Millisecond)
	if got := restarts.Load(); got != 1 {
		t.Fatalf("restart should still be queued during cooldown, got %d", got)
	}
	if postStartPendingLen(svc) == 0 {
		t.Fatal("cooldown path dropped pending event instead of queuing it")
	}

	waitForSignal(t, done, "queued cooldown restart did not happen")
	if got := restarts.Load(); got != 2 {
		t.Fatalf("expected queued second restart after cooldown, got %d", got)
	}
}

func TestScheduleRestartAfterInterfaceOnline_ReadyDelayTimeoutDoesNotLeaveStalePending(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	createEnabledRule(t, svc, "Youtube", "nwg0")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)
	svc.postStartTimeout = 30 * time.Millisecond
	svc.postStartReadyDelay = 50 * time.Millisecond
	svc.postStartReadyProbe = func(_ context.Context, _ string) (bool, string, error) {
		return true, "connected=yes conf=running link=up state=up", nil
	}

	done := make(chan struct{}, 1)
	svc.postStartRestart = func(_ context.Context) error {
		done <- struct{}{}
		return nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	waitForSignal(t, done, "best-effort restart after readyDelay timeout did not happen")

	time.Sleep(20 * time.Millisecond)
	if postStartRunning(svc) {
		t.Fatal("post-start cycle should not stay running after readyDelay timeout handling")
	}
	if postStartTimerArmed(svc) {
		t.Fatal("post-start timer should not remain armed after readyDelay timeout handling")
	}
	if got := postStartPendingLen(svc); got != 0 {
		t.Fatalf("pending interfaces should not remain stale after readyDelay timeout, got %d", got)
	}
}

func TestScheduleRestartAfterInterfaceOnline_PendingQueuedWhileRunningCooldownIsScheduledOnFinish(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	tunePostStartForTest(svc)
	svc.postStartPending = map[string]string{"Wireguard0": "nwg0"}
	svc.postStartRunning = true
	svc.postStartLastRestartAt = time.Now()
	svc.postStartCooldown = 40 * time.Millisecond
	svc.postStartDebounce = 5 * time.Millisecond
	svc.postStartTimer = nil

	svc.finishPostStartCycle()

	if postStartRunning(svc) {
		t.Fatal("finishPostStartCycle should clear running flag")
	}
	if !postStartTimerArmed(svc) {
		t.Fatal("finishPostStartCycle should schedule next timer for queued pending")
	}
	if got := postStartPendingLen(svc); got != 1 {
		t.Fatalf("pending should stay queued, got %d", got)
	}
	stopPostStartTimer(svc)
}

func TestScheduleRestartAfterInterfaceOnline_RemainingPendingSchedulesAfterRunningCleared(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	tunePostStartForTest(svc)
	svc.postStartDebounce = 0
	svc.postStartRestart = func(_ context.Context) error { return nil }
	svc.postStartPending = map[string]string{
		"Wireguard0": "nwg0",
		"Wireguard1": "nwg1",
	}
	svc.postStartRunning = true
	svc.postStartTimer = nil
	svc.postStartLastRestartAt = time.Now()

	svc.executePostStartRestart([]postStartStatus{{NDMS: "Wireguard0", Kernel: "nwg0", Ready: true}})
	if postStartTimerArmed(svc) {
		t.Fatal("executePostStartRestart should not schedule timer before running flag is cleared")
	}
	if !postStartRunning(svc) {
		t.Fatal("test setup expects running to stay true until finishPostStartCycle")
	}

	svc.finishPostStartCycle()

	if postStartRunning(svc) {
		t.Fatal("finishPostStartCycle should clear running flag")
	}
	if !postStartTimerArmed(svc) {
		t.Fatal("remaining pending should schedule next cycle after running is cleared")
	}
	if !postStartPendingContains(svc, "Wireguard1") {
		t.Fatalf("remaining pending interface lost")
	}
	stopPostStartTimer(svc)
}

func TestScheduleRestartAfterInterfaceOnline_DoesNotRestartBeforeOnline(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	createEnabledRule(t, svc, "Youtube", "nwg0")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)
	svc.postStartReadyProbe = func(_ context.Context, _ string) (bool, string, error) {
		return false, "connected=yes summaryConnected=true online=false conf=running link=up state=up", nil
	}

	var restarts atomic.Int32
	svc.postStartRestart = func(_ context.Context) error {
		restarts.Add(1)
		return nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	time.Sleep(120 * time.Millisecond)

	if got := restarts.Load(); got != 0 {
		t.Fatalf("restart should not happen before online=yes, got %d", got)
	}
}

func TestScheduleRestartAfterInterfaceOnline_RestartsWhenOnlineAndLayersReady(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	createEnabledRule(t, svc, "Youtube", "nwg0")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)

	done := make(chan struct{}, 1)
	svc.postStartReadyProbe = func(_ context.Context, _ string) (bool, string, error) {
		return true, "connected=yes summaryConnected=true online=true conf=running link=up state=up", nil
	}
	svc.postStartRestart = func(_ context.Context) error {
		done <- struct{}{}
		return nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	waitForSignal(t, done, "restart should happen when online=yes and all layers are ready")
}

func TestScheduleRestartAfterInterfaceOnline_TimeoutRestartsOnlyReadyStatuses(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	createEnabledRule(t, svc, "Rule0", "nwg0")
	createEnabledRule(t, svc, "Rule1", "nwg1")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)
	svc.postStartTimeout = 40 * time.Millisecond

	svc.postStartReadyProbe = func(_ context.Context, ndmsIface string) (bool, string, error) {
		if ndmsIface == "Wireguard0" {
			return true, "connected=yes summaryConnected=true online=true conf=running link=up state=up", nil
		}
		return false, "connected=yes summaryConnected=true online=false conf=running link=up state=up", nil
	}

	var mu sync.Mutex
	var restarted []string
	done := make(chan struct{}, 1)
	svc.postStartRestart = func(_ context.Context) error {
		mu.Lock()
		restarted = append(restarted, "restart")
		mu.Unlock()
		done <- struct{}{}
		return nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard1", "nwg1")

	waitForSignal(t, done, "timeout partial restart did not happen")
	time.Sleep(20 * time.Millisecond)

	mu.Lock()
	gotRestarts := len(restarted)
	mu.Unlock()
	if gotRestarts != 1 {
		t.Fatalf("expected exactly one partial timeout restart, got %d", gotRestarts)
	}
	if !postStartPendingContains(svc, "Wireguard1") {
		t.Fatal("not-ready pending interface should stay queued after partial timeout restart")
	}
	if postStartPendingContains(svc, "Wireguard0") {
		t.Fatal("ready interface should be removed from pending after partial timeout restart")
	}
}

func TestScheduleRestartAfterInterfaceOnline_IsInterfaceReadyRequiresOnline(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	fg := ndmsquery.NewFakeGetter()
	fg.SetJSON("/show/interface/", `{
		"Wireguard0": {"id":"Wireguard0","interface-name":"nwg0","type":"Wireguard","state":"up","link":"up","connected":"yes","summary":{"layer":{"ipv4":"running","conf":"running"}}}
	}`)
	fg.SetPostInterface("Wireguard0", `{"show":{"interface":{
		"id":"Wireguard0","state":"up","link":"up","conf-layer":"running","connected":"yes","online":"no",
		"summary":{"layer":{"conf":"running","link":"running","ctrl":"running"}}
	}}}`)
	svc.SetQueries(&ndmsquery.Queries{
		Interfaces: ndmsquery.NewInterfaceStore(fg, ndmsquery.NopLogger()),
	})

	ready, detail, err := svc.isInterfaceReady(context.Background(), "Wireguard0")
	if err != nil {
		t.Fatalf("isInterfaceReady: %v", err)
	}
	if ready {
		t.Fatalf("interface should not be ready without online=yes, detail=%s", detail)
	}
}

func TestScheduleRestartAfterInterfaceOnline_IsInterfaceReadyAcceptsOnline(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	fg := ndmsquery.NewFakeGetter()
	fg.SetJSON("/show/interface/", `{
		"Wireguard0": {"id":"Wireguard0","interface-name":"nwg0","type":"Wireguard","state":"up","link":"up","connected":"yes","summary":{"layer":{"ipv4":"running","conf":"running"}}}
	}`)
	fg.SetPostInterface("Wireguard0", `{"show":{"interface":{
		"id":"Wireguard0","state":"up","link":"up","conf-layer":"running","connected":"yes","online":"yes",
		"summary":{"layer":{"conf":"running","link":"running","ctrl":"running"}}
	}}}`)
	svc.SetQueries(&ndmsquery.Queries{
		Interfaces: ndmsquery.NewInterfaceStore(fg, ndmsquery.NopLogger()),
	})

	ready, detail, err := svc.isInterfaceReady(context.Background(), "Wireguard0")
	if err != nil {
		t.Fatalf("isInterfaceReady: %v", err)
	}
	if !ready {
		t.Fatalf("interface should be ready with online=yes, detail=%s", detail)
	}
}

func TestScheduleRestartAfterInterfaceOnline_LogsSkipNotInstalled(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	app := &fakeHydraAppLogger{}
	svc.appLog = logging.NewScopedLogger(app, logging.GroupRouting, logging.SubHrNeo)
	svc.SetStatusForTest(false)

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")

	if !app.has("post-start-skip-not-installed", logging.LevelDebug, "reason=boot-start") {
		t.Fatalf("missing post-start-skip-not-installed log: %+v", app.entries)
	}
}

func TestScheduleRestartAfterInterfaceOnline_LogsSkipNoRules(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	app := &fakeHydraAppLogger{}
	svc.appLog = logging.NewScopedLogger(app, logging.GroupRouting, logging.SubHrNeo)

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")

	if !app.hasScoped("post-start-skip-no-rules", logging.LevelInfo, logging.GroupRouting, logging.SubHrNeo, "nwg0", "reason=boot-start") {
		t.Fatalf("missing scoped post-start-skip-no-rules log: %+v", app.entries)
	}
	if !app.hasScoped("post-start-skip-no-rules", logging.LevelInfo, logging.GroupRouting, logging.SubHrNeo, "nwg0", "ndms=Wireguard0") {
		t.Fatalf("missing ndms in post-start-skip-no-rules log: %+v", app.entries)
	}
}

func TestScheduleRestartAfterInterfaceOnline_LogsInvalidInterfaceSkip(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	app := &fakeHydraAppLogger{}
	svc.appLog = logging.NewScopedLogger(app, logging.GroupRouting, logging.SubHrNeo)

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "", "nwg0")

	if !app.has("post-start-skip-invalid-interface", logging.LevelDebug, "reason=boot-start") {
		t.Fatalf("missing post-start-skip-invalid-interface log: %+v", app.entries)
	}
}

func TestScheduleRestartAfterInterfaceOnline_LogsScheduledWaitAndCycleBounds(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	app := &fakeHydraAppLogger{}
	svc.appLog = logging.NewScopedLogger(app, logging.GroupRouting, logging.SubHrNeo)
	createEnabledRule(t, svc, "Youtube", "nwg0")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)
	done := make(chan struct{}, 1)
	svc.postStartReadyProbe = func(_ context.Context, _ string) (bool, string, error) {
		return true, "connected=yes summaryConnected=true online=true conf=running link=up state=up", nil
	}
	svc.postStartRestart = func(_ context.Context) error {
		done <- struct{}{}
		return nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	waitForSignal(t, done, "restart should happen")
	time.Sleep(20 * time.Millisecond)

	if !app.has("post-start-wait", logging.LevelInfo, "scheduled:") {
		t.Fatalf("missing scheduled wait log: %+v", app.entries)
	}
	if !app.has("post-start-cycle-start", logging.LevelDebug, "") {
		t.Fatalf("missing cycle start log: %+v", app.entries)
	}
	if !app.has("post-start-cycle-finish", logging.LevelDebug, "") {
		t.Fatalf("missing cycle finish log: %+v", app.entries)
	}
}

func TestScheduleRestartAfterInterfaceOnline_LogsJoinedActiveWait(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	app := &fakeHydraAppLogger{}
	svc.appLog = logging.NewScopedLogger(app, logging.GroupRouting, logging.SubHrNeo)
	createEnabledRule(t, svc, "Rule1", "nwg1")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)
	svc.postStartRunning = true
	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard1", "nwg1")

	if !app.has("post-start-wait", logging.LevelInfo, "joined active wait:") {
		t.Fatalf("missing joined active wait log: %+v", app.entries)
	}
}

func TestScheduleRestartAfterInterfaceOnline_LogsCooldownQueued(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	app := &fakeHydraAppLogger{}
	svc.appLog = logging.NewScopedLogger(app, logging.GroupRouting, logging.SubHrNeo)
	createEnabledRule(t, svc, "Youtube", "nwg0")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)
	svc.postStartCooldown = 40 * time.Millisecond
	svc.postStartReadyProbe = func(_ context.Context, _ string) (bool, string, error) {
		return true, "connected=yes summaryConnected=true online=true conf=running link=up state=up", nil
	}
	done := make(chan struct{}, 2)
	svc.postStartRestart = func(_ context.Context) error {
		done <- struct{}{}
		return nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	waitForSignal(t, done, "first restart should happen")
	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "wan-up-start", "Wireguard0", "nwg0")

	if !app.has("post-start-wait", logging.LevelDebug, "queued during cooldown:") {
		t.Fatalf("missing cooldown queued log: %+v", app.entries)
	}
}

func TestScheduleRestartAfterInterfaceOnline_LogsRestartCompleted(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	app := &fakeHydraAppLogger{}
	svc.appLog = logging.NewScopedLogger(app, logging.GroupRouting, logging.SubHrNeo)
	createEnabledRule(t, svc, "Youtube", "nwg0")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)
	done := make(chan struct{}, 1)
	svc.postStartReadyProbe = func(_ context.Context, _ string) (bool, string, error) {
		return true, "connected=yes summaryConnected=true online=true conf=running link=up state=up", nil
	}
	svc.postStartRestart = func(_ context.Context) error {
		done <- struct{}{}
		return nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	waitForSignal(t, done, "restart should happen")
	time.Sleep(20 * time.Millisecond)

	if !app.has("post-start-restart", logging.LevelInfo, "ndms=Wireguard0") {
		t.Fatalf("missing restart log: %+v", app.entries)
	}
	if !app.has("post-start-restart-completed", logging.LevelInfo, "restarted for 1 interface") {
		t.Fatalf("missing restart completed log: %+v", app.entries)
	}
}

func TestScheduleRestartAfterInterfaceOnline_LogsReady(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	app := &fakeHydraAppLogger{}
	svc.appLog = logging.NewScopedLogger(app, logging.GroupRouting, logging.SubHrNeo)
	createEnabledRule(t, svc, "Youtube", "nwg0")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)
	done := make(chan struct{}, 1)
	svc.postStartReadyProbe = func(_ context.Context, _ string) (bool, string, error) {
		return true, "connected=yes summaryConnected=true online=true conf=running link=up state=up", nil
	}
	svc.postStartRestart = func(_ context.Context) error {
		done <- struct{}{}
		return nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	waitForSignal(t, done, "restart should happen")
	time.Sleep(20 * time.Millisecond)

	if !app.hasScoped("post-start-ready", logging.LevelInfo, logging.GroupRouting, logging.SubHrNeo, "nwg0", "ndms=Wireguard0") {
		t.Fatalf("missing scoped post-start-ready log: %+v", app.entries)
	}
	if !app.hasScoped("post-start-ready", logging.LevelInfo, logging.GroupRouting, logging.SubHrNeo, "nwg0", "online=true") {
		t.Fatalf("missing readiness detail in post-start-ready log: %+v", app.entries)
	}
}

func TestScheduleRestartAfterInterfaceOnline_LogsRestartFailure(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	app := &fakeHydraAppLogger{}
	svc.appLog = logging.NewScopedLogger(app, logging.GroupRouting, logging.SubHrNeo)
	createEnabledRule(t, svc, "Youtube", "nwg0")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)
	done := make(chan struct{}, 1)
	svc.postStartReadyProbe = func(_ context.Context, _ string) (bool, string, error) {
		return true, "connected=yes summaryConnected=true online=true conf=running link=up state=up", nil
	}
	svc.postStartRestart = func(_ context.Context) error {
		done <- struct{}{}
		return errors.New("boom")
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	waitForSignal(t, done, "restart attempt should happen")
	time.Sleep(20 * time.Millisecond)

	if !app.has("post-start-restart", logging.LevelWarn, "boom") {
		t.Fatalf("missing restart failure log: %+v", app.entries)
	}
}

func TestScheduleRestartAfterInterfaceOnline_LogsTimeoutDrop(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	app := &fakeHydraAppLogger{}
	svc.appLog = logging.NewScopedLogger(app, logging.GroupRouting, logging.SubHrNeo)
	createEnabledRule(t, svc, "Youtube", "nwg0")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)
	svc.postStartReadyProbe = func(_ context.Context, _ string) (bool, string, error) {
		return false, "not-ready", nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	time.Sleep(120 * time.Millisecond)

	if !app.has("post-start-timeout", logging.LevelWarn, "pending:") {
		t.Fatalf("missing timeout log: %+v", app.entries)
	}
	if !app.has("post-start-timeout-drop", logging.LevelWarn, "dropping pending interfaces") {
		t.Fatalf("missing timeout drop log: %+v", app.entries)
	}
}

func TestScheduleRestartAfterInterfaceOnline_LogsPartialTimeout(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	app := &fakeHydraAppLogger{}
	svc.appLog = logging.NewScopedLogger(app, logging.GroupRouting, logging.SubHrNeo)
	createEnabledRule(t, svc, "Rule0", "nwg0")
	createEnabledRule(t, svc, "Rule1", "nwg1")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)
	svc.postStartTimeout = 40 * time.Millisecond
	svc.postStartReadyProbe = func(_ context.Context, ndmsIface string) (bool, string, error) {
		if ndmsIface == "Wireguard0" {
			return true, "connected=yes summaryConnected=true online=true conf=running link=up state=up", nil
		}
		return false, "connected=no summaryConnected=false online=false conf=running link=up state=up", nil
	}
	done := make(chan struct{}, 1)
	svc.postStartRestart = func(_ context.Context) error {
		done <- struct{}{}
		return nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard1", "nwg1")
	waitForSignal(t, done, "partial timeout restart should happen")
	time.Sleep(20 * time.Millisecond)

	if !app.has("post-start-timeout-partial", logging.LevelInfo, "Wireguard1") {
		t.Fatalf("missing partial timeout log: %+v", app.entries)
	}
}

func TestScheduleRestartAfterInterfaceOnline_LogsRescheduleForRemainingPending(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	app := &fakeHydraAppLogger{}
	svc.appLog = logging.NewScopedLogger(app, logging.GroupRouting, logging.SubHrNeo)
	tunePostStartForTest(svc)
	svc.postStartDebounce = 0
	svc.postStartRestart = func(_ context.Context) error { return nil }
	svc.postStartPending = map[string]string{
		"Wireguard0": "nwg0",
		"Wireguard1": "nwg1",
	}
	svc.postStartRunning = true
	svc.postStartTimer = nil
	svc.postStartLastRestartAt = time.Now()

	svc.executePostStartRestart([]postStartStatus{{NDMS: "Wireguard0", Kernel: "nwg0", Ready: true}})
	svc.finishPostStartCycle()
	stopPostStartTimer(svc)

	if !app.has("post-start-reschedule", logging.LevelDebug, "pending=1") {
		t.Fatalf("missing reschedule log: %+v", app.entries)
	}
}

func TestScheduleRestartAfterInterfaceOnline_LogsPolicyCheckWarningFallback(t *testing.T) {
	svc, _, _ := setupRuleFiles(t)
	app := &fakeHydraAppLogger{}
	svc.appLog = logging.NewScopedLogger(app, logging.GroupRouting, logging.SubHrNeo)
	createEnabledRule(t, svc, "PolicyRule", "HydraRoute")
	clearRuleWriteRestart(svc)
	tunePostStartForTest(svc)

	fg := ndmsquery.NewFakeGetter()
	fg.SetDefaultError(errors.New("policy list failed"))
	svc.SetQueries(&ndmsquery.Queries{
		Policies: ndmsquery.NewPolicyStore(fg, ndmsquery.NopLogger()),
	})
	done := make(chan struct{}, 1)
	svc.postStartReadyProbe = func(_ context.Context, _ string) (bool, string, error) {
		return true, "connected=yes summaryConnected=true online=true conf=running link=up state=up", nil
	}
	svc.postStartRestart = func(_ context.Context) error {
		done <- struct{}{}
		return nil
	}

	svc.ScheduleRestartAfterInterfaceOnline(context.Background(), "boot-start", "Wireguard0", "nwg0")
	waitForSignal(t, done, "fallback restart should happen")

	if !app.has("post-start-policy-check", logging.LevelWarn, "policy list failed") {
		t.Fatalf("missing policy-check warn log: %+v", app.entries)
	}
}

func tunePostStartForTest(svc *Service) {
	svc.postStartDebounce = 5 * time.Millisecond
	svc.postStartCooldown = 5 * time.Millisecond
	svc.postStartTimeout = 60 * time.Millisecond
	svc.postStartReadyDelay = 0
	svc.postStartPollInterval = 5 * time.Millisecond
}

func createEnabledRule(t *testing.T, svc *Service, name string, target string) {
	t.Helper()
	if _, err := svc.CreateRule(HRRule{Name: name, Domains: []string{name + ".example"}, Target: target}); err != nil {
		t.Fatalf("CreateRule(%s): %v", name, err)
	}
}

func clearRuleWriteRestart(svc *Service) {
	if svc.restartTimer != nil {
		svc.restartTimer.Stop()
		svc.restartTimer = nil
	}
}

func waitForSignal(t *testing.T, ch <-chan struct{}, msg string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(300 * time.Millisecond):
		t.Fatal(msg)
	}
}

func stopPostStartTimer(s *Service) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.postStartTimer == nil {
		return false
	}
	s.postStartTimer.Stop()
	s.postStartTimer = nil
	return true
}

func postStartPendingContains(s *Service, ndmsIface string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.postStartPending[ndmsIface]
	return ok
}
