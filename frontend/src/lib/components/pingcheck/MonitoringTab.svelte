<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { onDestroy, onMount } from 'svelte';
	import { Settings2 } from 'lucide-svelte';
	import { api } from '$lib/api/client';
	import { clearHistoryCache, monitoringStore } from '$lib/stores/monitoring';
	import { settings as globalSettings, setSettings as setGlobalSettings } from '$lib/stores/settings';
	import { Button, SegmentedControl } from '$lib/components/ui';
	import { LoadingSpinner, EmptyState } from '$lib/components/layout';
	import { SideDrawer } from '$lib/components/ui';
	import { MatrixDrillDown, MatrixGrid, MatrixStatusStrip } from '$lib/components/monitoring';
	import { KernelPingCheckModal, NativeWGPingCheckModal } from '$lib/components/pingcheck';
	import WatchdogMonitoringMode from './WatchdogMonitoringMode.svelte';
	import { notifications } from '$lib/stores/notifications';
	import {
		DEFAULT_MONITORING_SETTINGS,
		getMonitoringHistoryCapacity,
		getMonitoringHistoryCapacityRaw,
		normalizeMonitoringSettings,
		validateMonitoringSettings,
	} from '$lib/constants/monitoring';
	import type { SegmentedOption } from '$lib/components/ui';
	import type {
		MonitoringSettings,
		MonitoringTarget,
		MonitoringTunnel,
		AWGTunnel,
		NativePingCheckStatus,
		Settings,
	} from '$lib/types';

	const MONITORING_MODE_STORAGE_KEY = 'awgm.monitoring.mode';

	function readInitialMode(): 'matrix' | 'watchdog' {
		if (typeof localStorage === 'undefined') return 'matrix';
		const raw = localStorage.getItem(MONITORING_MODE_STORAGE_KEY);
		return raw === 'watchdog' ? 'watchdog' : 'matrix';
	}

	let drawerOpen = $state(false);
	let planningOpen = $state(false);
	let drawerTarget = $state<MonitoringTarget | null>(null);
	let drawerTunnel = $state<MonitoringTunnel | null>(null);
	let refreshing = $state(false);
	let nowTs = $state(Date.now());
	let lastRefreshTs = $state(0);
	let lastFetchedAtTs = $state(0);
	let nextAutoRefreshTs = $state(0);
	let autoRefreshTimeout: ReturnType<typeof setTimeout> | null = null;
	let progressTimer: ReturnType<typeof setInterval> | null = null;
	let autoPressResetTimer: ReturnType<typeof setTimeout> | null = null;
	let autoPressActive = $state(false);
	let mode = $state<'matrix' | 'watchdog'>(readInitialMode());
	let settings = $state<Settings | null>(null);
	let monitoringDraft = $state<MonitoringSettings>({ ...DEFAULT_MONITORING_SETTINGS });
	let saving = $state(false);
	let excludedTunnelIds = $state<Set<string>>(new Set());
	let excludedTunnelNames = $state<Record<string, string>>({});
	let excludedNamesReady = $state(false);
	const excludedTunnelList = $derived.by(() => [...excludedTunnelIds].sort((a, b) => a.localeCompare(b)));
	const excludedTunnelLabels = $derived.by(() =>
		excludedTunnelList
			.map((id) => ({
				id,
				name: (excludedTunnelNames[id]?.trim() || (excludedNamesReady ? id : '')).trim(),
			}))
			.filter((item) => item.name !== ''),
	);
	const updatedTimeLabel = $derived.by(() => {
		if (lastFetchedAtTs <= 0) return '';
		const d = new Date(lastFetchedAtTs);
		if (Number.isNaN(d.getTime())) return '';
		return d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
	});
	const refreshProgress = $derived.by(() => {
		if (lastRefreshTs <= 0 || nextAutoRefreshTs <= lastRefreshTs) return 0;
		const elapsed = Math.max(0, nowTs - lastRefreshTs);
		const cycle = nextAutoRefreshTs - lastRefreshTs;
		return Math.min(1, elapsed / cycle);
	});
	const monitoringSettings = $derived(normalizeMonitoringSettings(($globalSettings ?? settings)?.monitoring));
	const matrixRefreshDelayMs = $derived(monitoringSettings.matrixRefreshIntervalSec * 1000);
	const historyCapacity = $derived(getMonitoringHistoryCapacity(monitoringSettings));
	const currentMonitoringDraft = $derived.by(() => ({
		historyHours: Number(monitoringDraft.historyHours),
		sampleIntervalSec: Number(monitoringDraft.sampleIntervalSec),
		matrixRefreshIntervalSec: Number(monitoringDraft.matrixRefreshIntervalSec),
	}));
	const monitoringErrors = $derived.by(() => validateMonitoringSettings(currentMonitoringDraft));
	const modeOptions: SegmentedOption<'matrix' | 'watchdog'>[] = [
		{ value: 'matrix', label: 'Матрица' },
		{ value: 'watchdog', label: 'Вотчдог' },
	];

	// Pingcheck drawer state — backend determines which form is shown.
	let pingTunnelId = $state('');
	let pingTunnelName = $state('');
	let pingBackend = $state<'kernel' | 'nativewg' | ''>('');
	let pingNativeStatus = $state<NativePingCheckStatus | null>(null);
	let pingOpenKernel = $state(false);
	let pingOpenNative = $state(false);

	function normalizeExcludedTunnelId(raw: string): string {
		let id = (raw ?? '').trim();
		let prev = '';
		while (id !== prev) {
			prev = id;
			id = id
				.trim()
				.replace(/^[\s\["',]+/g, '')
				.replace(/[\s\]"',]+$/g, '');
		}
		return id;
	}

	function normalizeExcludedTunnelIds(raw: string[] | undefined): Set<string> {
		const out = new Set<string>();
		for (const item of raw ?? []) {
			const trimmed = item.trim();
			if (!trimmed) continue;
			if (trimmed.startsWith('[') && trimmed.endsWith(']')) {
				try {
					const parsed = JSON.parse(trimmed);
					if (Array.isArray(parsed)) {
						for (const v of parsed) {
							const normalized = normalizeExcludedTunnelId(String(v));
							if (normalized) out.add(normalized);
						}
						continue;
					}
				} catch {
					// Fall through to plain sanitizer below.
				}
			}
			const normalized = normalizeExcludedTunnelId(trimmed);
			if (normalized) out.add(normalized);
		}
		return out;
	}

	function triggerRefresh(force = false): void {
		void refresh(force);
	}

	function formatDraftHistoryCapacity(value: MonitoringSettings): string {
		if (
			!Number.isFinite(value.historyHours) ||
			!Number.isFinite(value.sampleIntervalSec) ||
			value.sampleIntervalSec <= 0
		) {
			return '—';
		}
		return `${getMonitoringHistoryCapacityRaw(value)} points`;
	}

	function triggerAutoRefresh(): void {
		autoPressActive = true;
		if (autoPressResetTimer) clearTimeout(autoPressResetTimer);
		autoPressResetTimer = setTimeout(() => {
			autoPressActive = false;
			autoPressResetTimer = null;
		}, 220);
		triggerRefresh(false);
	}

	async function refresh(force = false) {
		if (refreshing) {
			if (autoRefreshTimeout) clearTimeout(autoRefreshTimeout);
			autoRefreshTimeout = setTimeout(() => {
				triggerAutoRefresh();
			}, matrixRefreshDelayMs);
			return;
		}
		refreshing = true;
		try {
			const snap = await api.getMonitoringMatrix({ force });
			monitoringStore.setSnapshot(snap);
			const names = { ...excludedTunnelNames };
			for (const t of snap.tunnels) {
				names[t.id] = t.name || t.id;
			}
			excludedTunnelNames = names;
			void hydrateExcludedTunnelNames();
			lastFetchedAtTs = Date.now();
		} catch {
			if (!$monitoringStore.snapshot) {
				notifications.error('Не удалось загрузить матрицу мониторинга');
			}
		} finally {
			lastRefreshTs = Date.now();
			nextAutoRefreshTs = lastRefreshTs + matrixRefreshDelayMs;
			if (autoRefreshTimeout) clearTimeout(autoRefreshTimeout);
			autoRefreshTimeout = setTimeout(() => {
				triggerAutoRefresh();
			}, matrixRefreshDelayMs);
			refreshing = false;
		}
	}

	async function loadSettings() {
		try {
			settings = await api.getSettings();
			monitoringDraft = normalizeMonitoringSettings(settings.monitoring);
			excludedNamesReady = false;
			excludedTunnelIds = normalizeExcludedTunnelIds(settings.monitoringExcludedTunnels);
			const names: Record<string, string> = {};
			for (const t of $monitoringStore.snapshot?.tunnels ?? []) {
				names[t.id] = t.name || t.id;
			}
			excludedTunnelNames = names;
			await hydrateExcludedTunnelNames();
		} catch {
			// Matrix can still work without settings payload.
		}
	}

	async function hydrateExcludedTunnelNames() {
		const missing = [...excludedTunnelIds].filter((id) => !excludedTunnelNames[id]);
		if (missing.length === 0) {
			excludedNamesReady = true;
			return;
		}
		const map: Record<string, string> = { ...excludedTunnelNames };
		const setName = (id: string, candidate: string) => {
			const key = normalizeExcludedTunnelId(id);
			const name = candidate.trim();
			if (!key || !name) return;
			const existing = map[key];
			if (!existing || existing === key) {
				map[key] = name;
			}
		};
		try {
			const [awg, system, singbox, subscriptions] = await Promise.all([
				api.listTunnels().catch(() => []),
				api.listSystemTunnels().catch(() => []),
				api.singboxListTunnels().catch(() => []),
				api.listSubscriptions().catch(() => []),
			]);
			for (const t of awg) setName(t.id, t.name || t.id);
			for (const t of system) {
				const label = t.description || t.id;
				setName(t.id, label);
				setName(`sys-${t.id}`, label);
			}
			for (const sub of subscriptions) {
				const label = sub.label?.trim() || sub.selectorTag?.trim() || sub.id;
				setName(sub.activeMember, label);
				for (const tag of sub.memberTags ?? []) setName(tag, label);
				for (const member of sub.members ?? []) setName(member.tag, label);
			}
			for (const t of singbox) setName(t.tag, t.tag);
			excludedTunnelNames = map;
		} catch {
			// Best-effort UX enrichment only.
		} finally {
			excludedNamesReady = true;
		}
	}

	onMount(() => {
		monitoringStore.loadCached();
		void loadSettings();
		triggerAutoRefresh();
		progressTimer = setInterval(() => {
			nowTs = Date.now();
		}, 200);
	});

	$effect(() => {
		if (typeof localStorage === 'undefined') return;
		localStorage.setItem(MONITORING_MODE_STORAGE_KEY, mode);
	});

	async function saveMonitoringTargetsSettings() {
		if (!settings) return;
		saving = true;
		try {
			settings = await api.updateSettings({
				pingCheck: {
					...settings.pingCheck,
					defaults: {
						...settings.pingCheck.defaults,
						target: settings.pingCheck.defaults.target,
					},
				},
				connectivityCheckUrl: settings.connectivityCheckUrl,
			});
			setGlobalSettings(settings);
			clearHistoryCache();
			await refresh(true);
			notifications.success('Monitoring targets saved');
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Failed to save monitoring targets');
		} finally {
			saving = false;
		}
	}

	async function saveMonitoringSettings() {
		if (!settings) return;
		if (Object.keys(monitoringErrors).length > 0) return;
		saving = true;
		try {
			const nextMonitoring = normalizeMonitoringSettings(currentMonitoringDraft);
			settings = await api.updateSettings({
				monitoring: nextMonitoring,
				pingCheck: {
					...settings.pingCheck,
					defaults: {
						...settings.pingCheck.defaults,
						target: settings.pingCheck.defaults.target,
					},
				},
				connectivityCheckUrl: settings.connectivityCheckUrl,
			});
			monitoringDraft = normalizeMonitoringSettings(settings.monitoring);
			setGlobalSettings(settings);
			clearHistoryCache();
			await refresh(true);
			notifications.success('Monitoring settings saved');
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Failed to save monitoring settings');
		} finally {
			saving = false;
		}
	}

	async function toggleTunnelExcluded(tunnelId: string, excluded: boolean, tunnelName = '') {
		if (!settings) {
			await loadSettings();
			if (!settings) return;
		}
		const previous = new Set(excludedTunnelIds);
		const next = new Set(excludedTunnelIds);
		const normalizedId = normalizeExcludedTunnelId(tunnelId);
		if (!normalizedId) return;
		if (excluded) next.add(normalizedId);
		else next.delete(normalizedId);
		excludedTunnelIds = next;
		if (excluded && tunnelName.trim() !== '') {
			excludedTunnelNames = { ...excludedTunnelNames, [normalizedId]: tunnelName };
		}
		try {
			settings = await api.updateSettings({ monitoringExcludedTunnels: [...next] });
			excludedTunnelIds = normalizeExcludedTunnelIds(settings.monitoringExcludedTunnels);
			await refresh(true);
			await hydrateExcludedTunnelNames();
		} catch (e) {
			excludedTunnelIds = previous;
			notifications.error(e instanceof Error ? e.message : 'Не удалось обновить исключения мониторинга');
		}
	}

	onDestroy(() => {
		if (autoRefreshTimeout) clearTimeout(autoRefreshTimeout);
		if (progressTimer) clearInterval(progressTimer);
		if (autoPressResetTimer) clearTimeout(autoPressResetTimer);
	});

	function openCell(target: MonitoringTarget, tunnel: MonitoringTunnel) {
		drawerTarget = target;
		drawerTunnel = tunnel;
		drawerOpen = true;
	}

	function closeDrawer() {
		drawerOpen = false;
	}

	$effect(() => {
		const id = $page.url.searchParams.get('pingcheck') ?? '';
		if (!id) {
			pingOpenKernel = false;
			pingOpenNative = false;
			pingTunnelId = '';
			return;
		}
		if (id === pingTunnelId) return;
		void openPingCheck(id);
	});

	async function openPingCheck(id: string) {
		try {
			const tunnel: AWGTunnel = await api.getTunnel(id);
			pingTunnelId = tunnel.id;
			pingTunnelName = tunnel.name || id;
			pingBackend = tunnel.backend === 'nativewg' ? 'nativewg' : 'kernel';
			if (pingBackend === 'nativewg') {
				pingNativeStatus = await api.getNativePingCheckStatus(id).catch(() => null);
				pingOpenNative = true;
				pingOpenKernel = false;
			} else {
				pingOpenKernel = true;
				pingOpenNative = false;
			}
		} catch {
			notifications.error('Не удалось открыть настройки pingcheck');
			closePingCheck();
		}
	}

	function closePingCheck() {
		const url = new URL(window.location.href);
		url.searchParams.delete('pingcheck');
		goto(url.pathname + url.search, { replaceState: true, keepFocus: true, noScroll: true });
	}

	function onPingSaved() {
		notifications.success('Настройки pingcheck сохранены');
		closePingCheck();
		void refresh();
	}

	function onPingRemoved() {
		closePingCheck();
		void refresh();
	}
</script>

<div class="monitoring-tab">
	<div class="meta-row">
		<div class="meta-left">
			<SegmentedControl
				value={mode}
				options={modeOptions}
				ariaLabel="Режим мониторинга"
				onchange={(next) => (mode = next)}
			/>
		</div>
		<div class="meta-actions">
			{#if mode === 'matrix'}
				<button
					type="button"
					class="planning-btn"
					onclick={() => (planningOpen = true)}
					aria-label="Открыть настройки мониторинга"
					title="Настройки"
				>
					<span class="planning-icon" aria-hidden="true">
						<Settings2 size={14} strokeWidth={2} />
					</span>
					<span>Настройки</span>
				</button>
				{#if updatedTimeLabel || ($monitoringStore.stale && refreshing)}
					<button
						type="button"
						class="refresh-btn timer-enabled refresh-btn-status"
						class:auto-press={autoPressActive}
						onclick={() => triggerRefresh(true)}
						disabled={refreshing}
						aria-label="Обновить мониторинг"
						title="Обновить"
						style={`--refresh-progress:${refreshProgress * 360}deg;`}
					>
						<span class="refresh-status">
							<span class="clock-dot" class:clock-dot-loading={refreshing}></span>
							{#if $monitoringStore.stale && refreshing}
								<span class="refresh-status-time">...</span>
							{:else if updatedTimeLabel}
								<span class="refresh-status-time">{updatedTimeLabel}</span>
							{/if}
						</span>
						<svg class="refresh-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
							<path d="M21 12a9 9 0 1 1-2.64-6.36M21 4v6h-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
						</svg>
					</button>
				{:else}
					<button
						type="button"
						class="refresh-btn timer-enabled"
						class:auto-press={autoPressActive}
						onclick={() => triggerRefresh(true)}
						disabled={refreshing}
						aria-label="Обновить мониторинг"
						title="Обновить"
						style={`--refresh-progress:${refreshProgress * 360}deg;`}
					>
						<svg class="refresh-icon" viewBox="0 0 24 24" fill="none" aria-hidden="true">
							<path d="M21 12a9 9 0 1 1-2.64-6.36M21 4v6h-6" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
						</svg>
					</button>
				{/if}
			{/if}
		</div>
	</div>

	{#if mode === 'matrix'}
		{#if $monitoringStore.snapshot}
			{#if excludedTunnelList.length > 0 && excludedNamesReady}
				<div class="excluded-strip">
					<span class="excluded-label">Исключены из мониторинга:</span>
					<div class="excluded-items">
						{#each excludedTunnelLabels as item}
							<button
								type="button"
								class="excluded-chip"
								onclick={() => toggleTunnelExcluded(item.id, false)}
								title="Вернуть в мониторинг"
							>
								{item.name}
							</button>
						{/each}
					</div>
				</div>
			{/if}
			<MatrixStatusStrip snapshot={$monitoringStore.snapshot} />
			<MatrixGrid
				snapshot={$monitoringStore.snapshot}
				onCellClick={openCell}
				excludedTunnelIds={excludedTunnelIds}
				onToggleTunnelExcluded={toggleTunnelExcluded}
			/>
		{:else if !$monitoringStore.loaded}
			<div class="loading"><LoadingSpinner size="lg" message="Получение данных мониторинга..." /></div>
		{:else}
			<EmptyState
				title="Нет данных мониторинга"
				description="Запустите хотя бы один туннель и подождите ~60 секунд для первого тика probe scheduler'а."
			/>
		{/if}
	{:else}
		<WatchdogMonitoringMode />
	{/if}

	<SideDrawer
		open={drawerOpen}
		onClose={closeDrawer}
		title={drawerTarget && drawerTunnel ? `${drawerTarget.name} × ${drawerTunnel.name}` : ''}
	>
		{#if drawerTarget && drawerTunnel}
			<MatrixDrillDown
				target={drawerTarget}
				tunnel={drawerTunnel}
				historyHours={monitoringSettings.historyHours}
				sampleIntervalSec={monitoringSettings.sampleIntervalSec}
				historyCapacity={historyCapacity}
				onClose={closeDrawer}
			/>
		{/if}
	</SideDrawer>

	<SideDrawer
		open={planningOpen}
		onClose={() => (planningOpen = false)}
		title="Настройки мониторинга"
		width={460}
	>
		{#if settings}
			<div class="planning-drawer">
				<section class="planning-section">
					<h4>Расписание</h4>
					<div class="planning-grid">
						<div class="planning-field">
							<span>Окно истории, часов</span>
							<input
								type="number"
								class="planning-input"
								min={1}
								max={168}
								step={1}
								bind:value={monitoringDraft.historyHours}
								disabled={saving}
							/>
							{#if monitoringErrors.historyHours}
								<span class="planning-error">{monitoringErrors.historyHours}</span>
							{/if}
						</div>
						<div class="planning-field">
							<span>Шаг выборки, секунд</span>
							<input
								type="number"
								class="planning-input"
								min={10}
								max={3600}
								step={1}
								bind:value={monitoringDraft.sampleIntervalSec}
								disabled={saving}
							/>
							{#if monitoringErrors.sampleIntervalSec}
								<span class="planning-error">{monitoringErrors.sampleIntervalSec}</span>
							{/if}
						</div>
						<div class="planning-field">
							<span>Обновление матрицы, секунд</span>
							<input
								type="number"
								class="planning-input"
								min={10}
								max={3600}
								step={1}
								bind:value={monitoringDraft.matrixRefreshIntervalSec}
								disabled={saving}
							/>
							{#if monitoringErrors.matrixRefreshIntervalSec}
								<span class="planning-error">{monitoringErrors.matrixRefreshIntervalSec}</span>
							{/if}
						</div>
						<div class="planning-preview">
							<span class="planning-stat-label">Вместимость истории</span>
							<strong>{formatDraftHistoryCapacity(currentMonitoringDraft)}</strong>
							{#if monitoringErrors.historyCapacity}
								<span class="planning-error planning-error--preview">{monitoringErrors.historyCapacity}</span>
							{/if}
						</div>
					</div>
					<div class="planning-actions">
						<Button
							variant="secondary"
							size="md"
							onclick={saveMonitoringSettings}
							disabled={saving || Object.keys(monitoringErrors).length > 0}
						>
							Сохранить расписание
						</Button>
					</div>
				</section>

				<section class="planning-section">
					<h4>Цели проверок</h4>
					<p class="planning-copy">ICMP-цель используется для глобального ping-check. HTTP-адрес вызывается через туннель для проверки доступности и задержки.</p>
					<label class="planning-field">
						<span>ICMP-цель</span>
						<input
							type="text"
							class="planning-input"
							bind:value={settings.pingCheck.defaults.target}
							placeholder="8.8.8.8"
							disabled={saving}
						/>
					</label>
					<label class="planning-field">
						<span>URL HTTP-проверки</span>
						<input
							type="url"
							class="planning-input"
							bind:value={settings.connectivityCheckUrl}
							placeholder="https://connectivitycheck.gstatic.com/generate_204"
							disabled={saving}
						/>
					</label>
					<div class="planning-actions">
						<Button variant="secondary" size="md" onclick={saveMonitoringTargetsSettings} disabled={saving}>
							Сохранить
						</Button>
					</div>
				</section>

				<section class="planning-section">
					<h4>Исключённые туннели</h4>
					<p class="planning-copy">Исключённые туннели скрываются из матрицы и пропускаются планировщиком мониторинга.</p>
					{#if excludedTunnelLabels.length > 0}
						<div class="planning-chip-list">
							{#each excludedTunnelLabels as item}
								<button
									type="button"
									class="excluded-chip"
									onclick={() => toggleTunnelExcluded(item.id, false)}
									title="Вернуть в мониторинг"
								>
									{item.name}
								</button>
							{/each}
						</div>
					{:else}
						<p class="planning-empty">Нет исключённых туннелей.</p>
					{/if}
				</section>
			</div>
		{/if}
	</SideDrawer>

	{#if pingTunnelId && pingBackend === 'kernel'}
		<KernelPingCheckModal
			bind:open={pingOpenKernel}
			tunnelId={pingTunnelId}
			tunnelName={pingTunnelName}
			onclose={closePingCheck}
			onSaved={onPingSaved}
		/>
	{/if}

	{#if pingTunnelId && pingBackend === 'nativewg'}
		<NativeWGPingCheckModal
			bind:open={pingOpenNative}
			tunnelId={pingTunnelId}
			tunnelName={pingTunnelName}
			status={pingNativeStatus}
			onclose={closePingCheck}
			onSaved={onPingSaved}
			onRemoved={onPingRemoved}
		/>
	{/if}
</div>

<style>
	.monitoring-tab {
		padding-top: 0.5rem;
	}

	.meta-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
		margin-bottom: 1rem;
		min-height: 28px;
	}

	.meta-left {
		display: inline-flex;
		align-items: center;
		min-height: 28px;
	}

	.meta-actions {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
	}

	.planning-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 0.4rem;
		height: 28px;
		padding: 0 0.625rem;
		border-radius: 6px;
		border: 1px solid var(--color-border);
		background: transparent;
		color: var(--color-text-muted);
		font-size: 0.8125rem;
		font-weight: 500;
		cursor: pointer;
		transition: all var(--t-fast) ease;
	}

	.planning-btn:hover {
		color: var(--color-accent);
		background: var(--color-bg-hover);
	}

	.planning-icon {
		flex: 0 0 auto;
	}

	.clock-dot {
		width: 8px;
		height: 8px;
		flex: 0 0 auto;
		border-radius: 50%;
		background: var(--color-success);
		box-shadow: 0 0 0 3px var(--color-success-tint);
		transition: background 0.2s ease;
	}

	.clock-dot-loading {
		background: var(--color-warning, var(--color-accent));
		animation: pulse 1s ease-in-out infinite;
	}

	.refresh-btn {
		position: relative;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 0.4rem;
		height: 28px;
		padding: 0 0.625rem;
		border-radius: 6px;
		border: 1px solid var(--color-border);
		background: transparent;
		color: var(--color-text-muted);
		font-size: 0.8125rem;
		font-weight: 500;
		cursor: pointer;
		transition: all var(--t-fast) ease;
	}

	.refresh-btn-status {
		gap: 0.5rem;
		padding-left: 0.55rem;
		padding-right: 0.55rem;
		border-radius: 999px;
	}

	.refresh-status {
		position: relative;
		z-index: 1;
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		font-family: var(--font-mono, ui-monospace, monospace);
		font-size: 11px;
		line-height: 1;
		color: var(--color-text-muted);
		white-space: nowrap;
	}

	.refresh-status-time {
		color: var(--color-text-primary);
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}

	.refresh-btn.timer-enabled::before {
		content: '';
		position: absolute;
		inset: -1px;
		border-radius: inherit;
		padding: 1px;
		background: conic-gradient(var(--color-accent) var(--refresh-progress), transparent 0deg);
		-webkit-mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0);
		mask: linear-gradient(#000 0 0) content-box, linear-gradient(#000 0 0);
		-webkit-mask-composite: xor;
		mask-composite: exclude;
		pointer-events: none;
		opacity: 0.95;
	}

	.refresh-btn:hover:not(:disabled) {
		color: var(--color-accent);
		background: var(--color-bg-hover);
	}

	.refresh-btn.auto-press:not(:disabled) {
		color: var(--color-accent);
		background: var(--color-bg-hover);
		transform: translateY(1px);
	}

	.refresh-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.refresh-icon {
		position: relative;
		z-index: 1;
		width: 14px;
		height: 14px;
	}

	.planning-drawer {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.planning-section {
		display: flex;
		flex-direction: column;
		gap: 0.625rem;
		padding-bottom: 0.75rem;
		border-bottom: 1px solid color-mix(in srgb, var(--color-border) 70%, transparent);
	}

	.planning-section:last-child {
		border-bottom: 0;
		padding-bottom: 0;
	}

	.planning-section h4 {
		margin: 0;
		font-size: 0.95rem;
		font-weight: 600;
	}

	.planning-copy,
	.planning-empty {
		margin: 0;
		font-size: 0.875rem;
		line-height: 1.45;
		color: var(--color-text-muted);
	}

	.planning-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.625rem;
	}

	.planning-preview {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
		padding: 0.75rem;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-sm);
		background: var(--color-bg-secondary);
	}

	.planning-field {
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
		font-size: 0.875rem;
	}

	.planning-input {
		width: 100%;
		padding: 0.7rem 0.85rem;
		border-radius: var(--radius-sm);
		border: 1px solid var(--color-border);
		background: var(--color-bg-secondary);
		color: var(--color-text-primary);
	}

	.planning-error {
		font-size: 0.75rem;
		color: var(--color-error);
		line-height: 1.3;
	}

	.planning-error--preview {
		margin-top: 0.25rem;
	}

	.planning-actions {
		display: flex;
		justify-content: flex-start;
		gap: 0.5rem;
		margin-top: 0.25rem;
	}

	.planning-stat-label {
		font-size: 0.72rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--color-text-muted);
	}

	.planning-chip-list {
		display: flex;
		flex-wrap: wrap;
		gap: 0.375rem;
	}

	.excluded-strip {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.5rem;
		margin-bottom: 0.75rem;
		padding: 0.625rem 0.75rem;
		border: 1px dashed var(--color-border);
		border-radius: var(--radius-md);
		background: color-mix(in srgb, var(--color-bg-secondary) 70%, transparent);
	}

	.excluded-label {
		font-size: 12px;
		color: var(--color-text-muted);
	}

	.excluded-items {
		display: inline-flex;
		flex-wrap: wrap;
		gap: 0.375rem;
	}

	.excluded-chip {
		height: 24px;
		padding: 0 0.55rem;
		border-radius: 999px;
		border: 1px solid color-mix(in srgb, var(--color-border) 82%, var(--color-accent) 18%);
		background: color-mix(in srgb, var(--color-bg-tertiary) 82%, transparent);
		color: var(--color-text-primary);
		font-size: 11px;
		cursor: pointer;
		transition:
			background var(--t-fast) ease,
			border-color var(--t-fast) ease,
			color var(--t-fast) ease;
	}

	.excluded-chip:hover {
		border-color: color-mix(in srgb, var(--color-accent) 35%, var(--color-border));
		background: color-mix(in srgb, var(--color-accent) 12%, transparent);
		color: var(--color-accent);
	}

	.loading {
		display: flex;
		justify-content: center;
		padding: 4rem 0;
	}

	@keyframes pulse {
		0%, 100% { opacity: 1; }
		50% { opacity: 0.4; }
	}

	@media (max-width: 640px) {
		.planning-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
