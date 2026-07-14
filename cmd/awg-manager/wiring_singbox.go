package main

import (
	"context"
	"path/filepath"

	"log/slog"
	"runtime"

	"github.com/hoaxisr/awg-manager/internal/api"
	"github.com/hoaxisr/awg-manager/internal/events"
	"github.com/hoaxisr/awg-manager/internal/integrationbackup"
	"github.com/hoaxisr/awg-manager/internal/logging"
	"github.com/hoaxisr/awg-manager/internal/singbox"
	"github.com/hoaxisr/awg-manager/internal/singbox/installer"
	singboxorch "github.com/hoaxisr/awg-manager/internal/singbox/orchestrator"
	"github.com/hoaxisr/awg-manager/internal/singbox/router"
	"github.com/hoaxisr/awg-manager/internal/singbox/subscription"
	"github.com/hoaxisr/awg-manager/internal/storage"
)

// setupSingbox builds the sing-box operator, the config.d slot
// orchestrator, the subscription service, the managed-binary installer and
// the background workers (watchdog, traffic, delay, log forwarder).
func (a *app) setupSingbox() {
	var err error
	// Sing-box integration
	a.singboxOp = singbox.NewOperator(singbox.OperatorDeps{
		Log:             slog.Default().With("component", "singbox"),
		Queries:         a.ndmsQueries,
		Commands:        a.ndmsCommands,
		AppLogger:       a.loggingService,
		SingboxLogLevel: a.settingsStore.GetSingboxLogLevel,
		// Seed the sticky-stop flag from disk so the watchdog respects
		// a user-pressed Stop across awgm restarts. SetManuallyStopped
		// writes the new intent back through a single-field updater so
		// concurrent writers on other Settings fields (e.g. router
		// service toggling SingboxRouter) cannot silently overwrite it.
		InitialManuallyStopped: a.settings.SingboxManuallyStopped,
		SetManuallyStopped:     a.settingsStore.SetSingboxManuallyStopped,
		IsNDMSProxyEnabled:     a.settingsStore.IsSingboxNDMSProxyEnabled,
	})
	// Если на старте флаг disabled — orphan-cleanup (после возможного
	// обрыва прошлой MigrateOff в любой момент). Reconcile подберёт
	// сигнал на первом тике watchdog'а.
	if !a.settingsStore.IsSingboxNDMSProxyEnabled() {
		a.singboxOp.MarkNeedsOrphanCleanup()
	}

	// config.d orchestrator — the single writer of slot files (00-base /
	// 10-tunnels / 15-awg / 20-router / 30-deviceproxy). Producers route
	// their writes through Save / SetEnabled so a "disabled" domain
	// actually moves the file out of sing-box's view (config.d/disabled/)
	// instead of leaving stale content behind.
	singboxConfigDir := a.singboxOp.ConfigDir()
	if err := singbox.MigrateDeviceProxyOutOfTunnels(singboxConfigDir); err != nil {
		a.bootLog.Warn("deviceproxy-migration", "", err.Error())
	}
	a.sbOrch = singboxorch.New(singboxConfigDir, a.singboxOp.Process())
	a.sbOrch.SetLogger(func(level, msg string) {
		switch level {
		case "warn":
			a.loggingService.AppLog(logging.LevelWarn, logging.GroupSingbox, logging.SubSBProcess, "orchestrator", "", msg)
		case "error":
			a.loggingService.AppLog(logging.LevelError, logging.GroupSingbox, logging.SubSBProcess, "orchestrator", "", msg)
		default:
			a.loggingService.AppLog(logging.LevelInfo, logging.GroupSingbox, logging.SubSBProcess, "orchestrator", "", msg)
		}
	})
	a.sbOrch.SetValidator(&orchValidatorAdapter{v: singbox.NewValidator(installer.DefaultBinaryPath)})
	// Propagate the sticky-stop intent so reload-triggered cold-starts
	// (slot-file writes from router/deviceproxy/subscriptions) respect a
	// user-pressed Stop in the same way the watchdog does.
	a.sbOrch.SetShouldRun(func() bool { return !a.singboxOp.IsManuallyStopped() })
	for _, meta := range singboxorch.KnownSlots() {
		// SlotTunnels is AlwaysOn but only counts as "active work" when
		// the user has defined sing-box tunnels — wire HasContent so
		// the daemon stops running for an empty 10-tunnels.json.
		if meta.Slot == singboxorch.SlotTunnels {
			meta.HasContent = func() bool {
				return a.singboxOp.HasUserTunnels()
			}
		}
		if err := a.sbOrch.Register(meta); err != nil {
			a.bootLog.Error("singbox-orchestrator", string(meta.Slot), "register failed: "+err.Error())
		}
	}
	if err := a.sbOrch.Bootstrap(); err != nil {
		a.bootLog.Error("singbox-orchestrator", "bootstrap", err.Error())
	}
	// Legacy download-proxy slot (35-download-proxy.json) is no longer used
	// by the downloader, but disable it on boot in case a previous awgm
	// process crashed with the slot still enabled.
	if err := a.sbOrch.SetEnabledSilent(singboxorch.SlotDownloadProxy, false); err != nil {
		a.bootLog.Warn("singbox-orchestrator", "downloadproxy-disable", err.Error())
	}
	// Reflect Settings into orchestrator slot enabled-state. router /
	// deviceproxy / subscriptions are content-driven; tunnels / awg
	// are AlwaysOn (registered as such above) and cannot be toggled
	// here — Register already marked them enabled. deviceproxy is
	// reflected after deviceProxySvc is constructed below.
	if curSettings, err := a.settingsStore.Load(); err == nil && curSettings != nil {
		mode := curSettings.SingboxRouter.RoutingMode
		_ = a.sbOrch.SetEnabled(router.RouterSlotForMode(mode), curSettings.SingboxRouter.Enabled)
		_ = a.sbOrch.SetEnabled(router.OtherRouterSlot(mode), false)
	}

	// Subscription service — owns 40-subscriptions.json in config.d.
	// NewOperatorAdapter registers the slot into sbOrch (must happen before
	// Bootstrap so Bootstrap can scan the file). LoadFromDisk reads any
	// existing 40-subscriptions.json so the in-memory state is consistent.
	subStorePath := filepath.Join(a.dataDir, "subscriptions.json")
	a.subStore, err = subscription.NewStore(subStorePath)
	if err != nil {
		a.bootLog.Error("subscription-store", "", err.Error())
	}
	subProxyMgr := singbox.NewProxyManager(a.ndmsQueries, a.ndmsCommands)
	a.subAdapter = subscription.NewOperatorAdapter(a.sbOrch, subProxyMgr, a.singboxOp.Clash())
	if err := a.subAdapter.LoadFromDisk(singboxConfigDir); err != nil {
		a.bootLog.Warn("subscription-adapter", "load-from-disk", err.Error())
	}
	a.subSvc = subscription.NewService(a.subStore, a.subAdapter)
	a.subSvc.SetAppLogger(a.loggingService)
	// Gate subscription ProxyN creation on the global toggle (same flag the
	// Operator uses for tunnels) so disabling it stops subscriptions from
	// creating NDMS Proxy interfaces too.
	a.subSvc.SetNDMSProxyEnabled(a.settingsStore.IsSingboxNDMSProxyEnabled)

	// Сводные группы (#372) — отдельный JSON-файл рядом с subscriptions.json.
	subGroupStorePath := filepath.Join(a.dataDir, "subscription-groups.json")
	a.subGroupStore, err = subscription.NewGroupStore(subGroupStorePath)
	if err != nil {
		// Битый файл групп: карантиним (<path>.corrupt — данные сохраняются
		// для ручного восстановления, как у других store) и пересоздаём
		// пустой store. Иначе функциональность групп молча выключалась бы,
		// а их ProxyN оставались бы без владельца до конца аптайма.
		a.bootLog.Error("subscription-group-store", "", err.Error()+" — quarantining and recreating empty store")
		storage.QuarantineCorrupt(subGroupStorePath, err)
		a.subGroupStore, err = subscription.NewGroupStore(subGroupStorePath)
		if err != nil {
			a.bootLog.Error("subscription-group-store", "", "recreate after quarantine failed: "+err.Error())
		}
	}
	if a.subGroupStore != nil {
		a.subSvc.SetGroupStore(a.subGroupStore)
	}

	// Let NDMS-proxy enable/disable + orphan cleanup manage subscription
	// composite proxies (a set separate from Tunnels()).
	a.singboxOp.SetSubscriptionProxySet(subProxySet{store: a.subStore, groups: a.subGroupStore})
	// On MigrateOn, reconcile subscription proxies through the service so
	// subscriptions created while the toggle was off get a freshly allocated
	// ProxyN (not just the already-indexed ones).
	a.singboxOp.SetSubscriptionProxySync(a.subSvc.SyncProxies)

	// Wire orchestrator into Operator so ApplyConfig writes 10-tunnels.json
	// through SlotTunnels rather than an in-place write that bypasses
	// the orchestrator's validate / debounced reload.
	a.singboxOp.SetOrch(a.sbOrch)
	// Предикат перезапуска для watchdog/Reconcile (#456): упавший sing-box
	// поднимается и когда легаси-туннелей нет, но активны orchestrator-слоты
	// (router / deviceproxy / subscriptions / пользовательские туннели).
	a.singboxOp.SetActiveWorkFn(a.sbOrch.HasActiveWork)

	// Wire managed-binary installer into Operator. The installer is keyed
	// by the build-time arch string (e.g. "mipsel-3.4") so it can resolve
	// the correct download URL and SHA256 from EmbeddedBinaries.
	arch := detectArch()
	if arch == "" {
		a.bootLog.Warn("managed-singbox", runtime.GOARCH, "could not derive arch — install/update disabled")
	} else {
		spec, ok := installer.EmbeddedBinaries[arch]
		if !ok {
			a.bootLog.Warn("managed-singbox", arch, "no embedded BinarySpec — install/update disabled")
		} else {
			a.singboxInstaller = installer.New(installer.DefaultBinaryPath, arch, spec, a.loggingService)
			a.singboxOp.SetInstaller(a.singboxInstaller)

			// Stream sing-box install/update lifecycle over SSE so the UI
			// can render a live progress bar instead of a blocking spinner.
			a.singboxOp.SetInstallProgressReporter(func(op, phase string, downloaded, total int64, errMsg string) {
				a.eventBus.Publish("singbox:install-progress", events.SingboxInstallProgressEvent{
					Op:         op,
					Phase:      phase,
					Downloaded: downloaded,
					Total:      total,
					Error:      errMsg,
				})
			})

		}
	}

	delayChecker := singbox.NewDelayChecker(
		a.singboxOp.Clash(),
		&singboxAndSubLister{op: a.singboxOp, sub: a.subSvc},
		a.eventBus,
	)
	a.singboxHandler = api.NewSingboxHandler(a.singboxOp, a.eventBus, delayChecker, a.testService, a.loggingService)
	singboxMigrator := singbox.NewMigrator(a.singboxOp, a.settingsStore, a.loggingService)
	a.singboxHandler.SetNDMSProxyMigrator(singboxMigrator, a.settingsStore)
	a.clashProxy = api.NewClashProxy(a.singboxOp)
	a.singboxConnsHandler = api.NewSingboxConnectionsHandler(a.ndmsQueries.Hotspot)
	// Managed WG-server peer names for the connections monitor (issue
	// #435): in-memory store read, no NDMS round-trip. The system-server
	// source is wired in server.go once ServersHandler exists.
	if a.managedService != nil {
		a.singboxConnsHandler.SetManagedServers(a.managedService)
	}

	// Watchdog: runs an immediate reconcile (replacing the old one-shot
	// startup reconcile) and keeps checking every 30s. If sing-box crashes
	// while awgm is running, the next tick detects the dead PID and
	// restarts it; the UI is notified via resource:invalidated SSE hints
	// only when the running state actually flips.
	watchdogCtx, watchdogCancel := context.WithCancel(context.Background())
	a.deferOnExit(watchdogCancel)
	go singbox.NewWatchdog(a.singboxOp, a.eventBus, slog.Default().With("component", "singbox-watchdog")).Run(watchdogCtx)

	trafficCtx, trafficCancel := context.WithCancel(context.Background())
	a.deferOnExit(trafficCancel)
	go singbox.NewTrafficAggregator(a.singboxOp.Clash().Address(), a.eventBus, a.trafficHistory).Run(trafficCtx)

	delayCtx, delayCancel := context.WithCancel(context.Background())
	a.deferOnExit(delayCancel)
	go delayChecker.Run(delayCtx)

	// Forward sing-box runtime logs from clash_api /logs into the app's
	// UI log view (replaces the old file-based log; see process.go).
	logFwdCtx, logFwdCancel := context.WithCancel(context.Background())
	a.deferOnExit(logFwdCancel)

	// Integration backup/restore for sing-box and HydraRoute is created only
	// after both services exist. The HTTP layer receives it via server.Deps.
	a.integrationBackup = integrationbackup.NewService(a.dataDir, a.settingsStore, newIntegrationBackupSingbox(a.singboxOp), a.hydraService)
	go singbox.NewLogForwarder(a.singboxOp.Clash().Address(), a.loggingService).Run(logFwdCtx)

}
