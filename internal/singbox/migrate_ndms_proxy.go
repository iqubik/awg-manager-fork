package singbox

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/hoaxisr/awg-manager/internal/sys/ndmsinfo"
)

// SettingsToggler — минимальный subset SettingsStore, нужный мигратору.
// Декаплинг изолирует юнит-тесты мигратора от полного SettingsStore.
type SettingsToggler interface {
	SetSingboxCreateNDMSProxy(v bool) error
	IsSingboxNDMSProxyEnabled() bool
}

// Migrator переводит sing-box между режимами NDMS Proxy on/off.
// Не делает dependency-check: реальные NDMS-policies живут в прошивке
// роутера (не в awg-manager storages), пользователю показывается
// предупреждение в UI перед выключением.
type Migrator struct {
	op       *Operator
	settings SettingsToggler
	log      *slog.Logger
}

func NewMigrator(op *Operator, settings SettingsToggler) *Migrator {
	log := op.log
	if log == nil {
		log = slog.Default()
	}
	return &Migrator{op: op, settings: settings, log: log}
}

// MigrateOff выключает создание ProxyN. Sequence:
//  1. Settings.flag := false (single-writer pattern). Делаем первым,
//     чтобы при обрыве в шаге 2 next-start подобрал orphan-cleanup.
//  2. Для каждого туннеля с ненулевым ProxyInterface — RemoveProxy(idx).
//     Best-effort, ошибки только в лог.
//  3. MarkNeedsOrphanCleanup — Reconcile дочистит остатки на следующем тике.
//  4. SSE invalidate.
//
// config.json не правится: ProxyInterface/KernelInterface — derived в
// Tunnels() парсере из listen_port, не stored. ListTunnels (T9) очищает
// их в выдаче на верх в disabled-режиме.
func (m *Migrator) MigrateOff(ctx context.Context) error {
	m.op.migrationMu.Lock()
	defer m.op.migrationMu.Unlock()

	if err := m.settings.SetSingboxCreateNDMSProxy(false); err != nil {
		return fmt.Errorf("flip setting: %w", err)
	}

	cfg, err := m.op.loadConfig()
	if err == nil && cfg != nil {
		for _, t := range cfg.Tunnels() {
			if t.ProxyInterface == "" {
				continue
			}
			idx, perr := parseProxyIdx(t.ProxyInterface)
			if perr != nil || idx < 0 {
				continue
			}
			if rerr := m.op.proxyMgr.RemoveProxy(ctx, idx); rerr != nil {
				m.log.Warn("MigrateOff: RemoveProxy failed",
					"tag", t.Tag, "iface", t.ProxyInterface, "err", rerr)
			}
		}
	}

	// Subscription composite proxies are a separate managed set, invisible to
	// Tunnels() — remove them explicitly so disabling NDMS Proxy also tears
	// down the ProxyN behind selector/urltest subscriptions.
	for _, sp := range m.op.subscriptionProxies() {
		if rerr := m.op.proxyMgr.RemoveProxy(ctx, sp.Index); rerr != nil {
			m.log.Warn("MigrateOff: RemoveProxy (subscription) failed",
				"label", sp.Label, "idx", sp.Index, "err", rerr)
		}
	}

	m.op.MarkNeedsOrphanCleanup()
	if m.op.bus != nil {
		m.op.bus.Publish("resource:invalidated", map[string]any{"resource": "singbox.status"})
		m.op.bus.Publish("resource:invalidated", map[string]any{"resource": "singbox.tunnels"})
	}
	return nil
}

// MigrateOn включает создание ProxyN. Precondition (NDMS-компонент
// 'proxy' установлен) проверяется ДО изменения settings, чтобы не
// оставить флаг включённым без рабочей инфраструктуры.
func (m *Migrator) MigrateOn(ctx context.Context) error {
	m.op.migrationMu.Lock()
	defer m.op.migrationMu.Unlock()

	if !ndmsinfo.HasProxyComponent() {
		return ErrProxyComponentMissing
	}

	if err := m.settings.SetSingboxCreateNDMSProxy(true); err != nil {
		return fmt.Errorf("flip setting: %w", err)
	}

	cfg, err := m.op.loadConfig()
	if err == nil && cfg != nil {
		// SyncProxies идемпотентен — создаст недостающие ProxyN.
		// Tunnels() заполнит ProxyInterface "Proxy<slot>" из listen_port.
		if serr := m.op.proxyMgr.SyncProxies(ctx, cfg.Tunnels()); serr != nil {
			m.log.Warn("MigrateOn: SyncProxies failed", "err", serr)
		}
	}

	// Recreate subscription composite proxies symmetrically (SyncProxies only
	// covers Tunnels()). EnsureProxy is idempotent.
	for _, sp := range m.op.subscriptionProxies() {
		if eerr := m.op.proxyMgr.EnsureProxy(ctx, sp.Index, sp.Port, sp.Label); eerr != nil {
			m.log.Warn("MigrateOn: EnsureProxy (subscription) failed",
				"label", sp.Label, "idx", sp.Index, "err", eerr)
		}
	}

	if m.op.bus != nil {
		m.op.bus.Publish("resource:invalidated", map[string]any{"resource": "singbox.status"})
		m.op.bus.Publish("resource:invalidated", map[string]any{"resource": "singbox.tunnels"})
	}
	return nil
}
