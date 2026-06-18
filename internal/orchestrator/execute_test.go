package orchestrator

import (
	"context"
	"strings"
	"testing"

	"github.com/hoaxisr/awg-manager/internal/logging"
)

type fakeHydraRoutePostStartExecutor struct {
	calls []struct {
		reason string
		ndms   string
		iface  string
	}
}

func (f *fakeHydraRoutePostStartExecutor) ScheduleRestartAfterInterfaceOnline(_ context.Context, reason, ndmsIface, kernelIface string) {
	f.calls = append(f.calls, struct {
		reason string
		ndms   string
		iface  string
	}{reason: reason, ndms: ndmsIface, iface: kernelIface})
}

type fakeAppLogger struct {
	entries []fakeAppLogEntry
}

type fakeAppLogEntry struct {
	level    logging.Level
	group    string
	subgroup string
	action   string
	target   string
	message  string
}

func (f *fakeAppLogger) AppLog(level logging.Level, group, subgroup, action, target, message string) {
	f.entries = append(f.entries, fakeAppLogEntry{
		level:    level,
		group:    group,
		subgroup: subgroup,
		action:   action,
		target:   target,
		message:  message,
	})
}

func TestExecuteOne_LogsActionStartAndDone(t *testing.T) {
	app := &fakeAppLogger{}
	o := &Orchestrator{appLog: logging.NewScopedLogger(app, logging.GroupTunnel, logging.SubOrchestrator)}

	err := o.executeOne(context.Background(), Action{
		Type:   ActionHydraRoutePostStart,
		Tunnel: "awg0",
		Reason: "boot-start",
		NDMS:   "OpkgTun0",
		Iface:  "opkgtun0",
	})
	if err != nil {
		t.Fatalf("executeOne returned error: %v", err)
	}
	if len(app.entries) != 3 {
		t.Fatalf("expected 3 log entries (start/skip/done), got %d", len(app.entries))
	}
	assertAppEntry(t, app.entries[0], logging.LevelFull, "action-start", "awg0", "ActionHydraRoutePostStart")
	assertAppEntry(t, app.entries[2], logging.LevelFull, "action-done", "awg0", "ActionHydraRoutePostStart")
}

func TestExecuteOne_LogsActionError(t *testing.T) {
	app := &fakeAppLogger{}
	o := &Orchestrator{appLog: logging.NewScopedLogger(app, logging.GroupTunnel, logging.SubOrchestrator)}

	err := o.executeOne(context.Background(), Action{
		Type:   ActionStartNativeWG,
		Tunnel: "awg0",
	})
	if err == nil {
		t.Fatal("expected error from ActionStartNativeWG without nwg executor")
	}
	if len(app.entries) != 2 {
		t.Fatalf("expected 2 log entries (start/error), got %d", len(app.entries))
	}
	assertAppEntry(t, app.entries[0], logging.LevelFull, "action-start", "awg0", "ActionStartNativeWG")
	assertAppEntry(t, app.entries[1], logging.LevelWarn, "action-error", "awg0", "ActionStartNativeWG")
	if !strings.Contains(app.entries[1].message, "NativeWG backend not available") {
		t.Fatalf("error log missing backend message: %q", app.entries[1].message)
	}
}

func TestExecuteOne_HydraRoutePostStartLogsPayload(t *testing.T) {
	app := &fakeAppLogger{}
	fake := &fakeHydraRoutePostStartExecutor{}
	o := &Orchestrator{
		appLog:     logging.NewScopedLogger(app, logging.GroupTunnel, logging.SubOrchestrator),
		hydraRoute: fake,
	}

	err := o.executeOne(context.Background(), Action{
		Type:   ActionHydraRoutePostStart,
		Tunnel: "awg0",
		Reason: "boot-start",
		NDMS:   "Wireguard0",
		Iface:  "nwg0",
	})
	if err != nil {
		t.Fatalf("executeOne returned error: %v", err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("expected 1 executor call, got %d", len(fake.calls))
	}
	if len(app.entries) != 2 {
		t.Fatalf("expected 2 log entries (start/done), got %d", len(app.entries))
	}
	for _, entry := range app.entries {
		assertAppEntry(t, entry, entry.level, entry.action, "awg0", "ActionHydraRoutePostStart")
		if !strings.Contains(entry.message, "reason=boot-start") ||
			!strings.Contains(entry.message, "ndms=Wireguard0") ||
			!strings.Contains(entry.message, "iface=nwg0") {
			t.Fatalf("payload log missing HR post-start fields: %+v", entry)
		}
	}
}

func TestExecuteOne_HydraRoutePostStartLogsSkipWhenExecutorNil(t *testing.T) {
	app := &fakeAppLogger{}
	o := &Orchestrator{appLog: logging.NewScopedLogger(app, logging.GroupTunnel, logging.SubOrchestrator)}

	err := o.executeOne(context.Background(), Action{
		Type:   ActionHydraRoutePostStart,
		Tunnel: "awg0",
		Reason: "boot-start",
		NDMS:   "Wireguard0",
		Iface:  "nwg0",
	})
	if err != nil {
		t.Fatalf("executeOne returned error: %v", err)
	}
	if len(app.entries) != 3 {
		t.Fatalf("expected 3 log entries (start/skip/done), got %d", len(app.entries))
	}
	assertAppEntry(t, app.entries[0], logging.LevelFull, "action-start", "awg0", "ActionHydraRoutePostStart")
	assertAppEntry(t, app.entries[1], logging.LevelDebug, "action-skip", "awg0", "ActionHydraRoutePostStart")
	assertAppEntry(t, app.entries[2], logging.LevelFull, "action-done", "awg0", "ActionHydraRoutePostStart")
	if !strings.Contains(app.entries[1].message, "skip=hydraroute executor is not wired") {
		t.Fatalf("skip log missing reason: %q", app.entries[1].message)
	}
}

func assertAppEntry(t *testing.T, entry fakeAppLogEntry, level logging.Level, action, target, contains string) {
	t.Helper()
	if entry.level != level {
		t.Fatalf("level = %s, want %s", entry.level, level)
	}
	if entry.group != logging.GroupTunnel {
		t.Fatalf("group = %s, want %s", entry.group, logging.GroupTunnel)
	}
	if entry.subgroup != logging.SubOrchestrator {
		t.Fatalf("subgroup = %s, want %s", entry.subgroup, logging.SubOrchestrator)
	}
	if entry.action != action {
		t.Fatalf("action = %s, want %s", entry.action, action)
	}
	if entry.target != target {
		t.Fatalf("target = %s, want %s", entry.target, target)
	}
	if !strings.Contains(entry.message, contains) {
		t.Fatalf("message %q does not contain %q", entry.message, contains)
	}
}
