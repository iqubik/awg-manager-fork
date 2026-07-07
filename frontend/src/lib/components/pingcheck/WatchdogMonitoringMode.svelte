<script lang="ts">
	import { ChevronDown } from 'lucide-svelte';
	import { onDestroy, onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { pingCheckStatus, pingCheckLogs, loadPingLogs } from '$lib/stores/pingcheck';
	import { singboxStatus, singboxTunnels } from '$lib/stores/singbox';
	import { singboxWatchdogStatus } from '$lib/stores/singboxWatchdog';
	import { usageLevel } from '$lib/stores/settings';
	import { isSectionVisible } from '$lib/types/usageLevel';
	import { groupLogsByTunnel, computeCardStats } from '$lib/utils/pingStats';
	import WatchdogCard from './WatchdogCard.svelte';
	import SingboxWatchdogCard, { type SingboxWatchdogCardModel } from './SingboxWatchdogCard.svelte';
	import SingboxWatchdogSettingsDrawer from './SingboxWatchdogSettingsDrawer.svelte';
	import KernelPingCheckModal from '$lib/components/pingcheck/KernelPingCheckModal.svelte';
	import NativeWGPingCheckModal from '$lib/components/pingcheck/NativeWGPingCheckModal.svelte';
	import EmptyState from '$lib/components/layout/EmptyState.svelte';
	import { notifications } from '$lib/stores/notifications';
	import type {
		AWGTunnel,
		NativePingCheckConfig,
		NativePingCheckStatus,
		SingboxWatchdogLogEntry,
		SingboxWatchdogStatus,
		TunnelListItem,
	} from '$lib/types';

	let tunnelMeta = $state<TunnelListItem[]>([]);
	let configs = $state<Record<string, AWGTunnel['pingCheck']>>({});
	let loading = $state(true);

	let editId = $state<string | null>(null);
	let editName = $state('');
	let editBackend = $state<'kernel' | 'nativewg'>('kernel');
	let editNativeStatus = $state<NativePingCheckStatus | null>(null);
	let unsubSingboxStatus: (() => void) | undefined;
	let unsubSingboxTunnels: (() => void) | undefined;
	let unsubSingboxWatchdog: (() => void) | undefined;
	type WatchdogSectionId = 'awg' | 'singbox';
	const WATCHDOG_SECTIONS_OPEN_STORAGE_KEY = 'watchdog_monitoring_sections_open_v1';
	let watchdogSectionsHydrated = $state(false);
	let openSections = $state<Record<WatchdogSectionId, boolean>>({
		awg: true,
		singbox: true,
	});
	let singboxWatchdogDrawerOpen = $state(false);
	let singboxWatchdogDrawerTarget = $state<SingboxWatchdogStatus | null>(null);
	let singboxWatchdogLogs = $state<Record<string, SingboxWatchdogLogEntry[]>>({});
	let singboxWatchdogLogsLoaded = $state(false);
	let singboxWatchdogLogsLoadedKey = $state('');

	const statuses = $derived($pingCheckStatus.data ?? []);
	const logsByTunnel = $derived(groupLogsByTunnel($pingCheckLogs));
	const singboxSectionVisible = $derived(isSectionVisible($usageLevel, 'singboxTunnels'));
	const singboxState = $derived($singboxStatus);
	const singboxWatchdogState = $derived($singboxWatchdogStatus);
	const singboxInstalled = $derived(singboxState.data?.installed === true);
	const singboxWatchdogList = $derived(Array.isArray(singboxWatchdogState.data) ? singboxWatchdogState.data : []);

	async function loadConfigs(ids: string[]) {
		const next: Record<string, AWGTunnel['pingCheck']> = {};
		await Promise.all(
			ids.map(async (id) => {
				try {
					const tunnel = await api.getTunnel(id);
					if (tunnel.pingCheck) next[id] = tunnel.pingCheck;
				} catch {
					// Tunnel may disappear between refreshes.
				}
			}),
		);
		configs = next;
	}

	onMount(async () => {
		unsubSingboxStatus = singboxStatus.subscribe(() => {});
		unsubSingboxTunnels = singboxTunnels.subscribe(() => {});
		unsubSingboxWatchdog = singboxWatchdogStatus.subscribe(() => {});

		try {
			const raw = localStorage.getItem(WATCHDOG_SECTIONS_OPEN_STORAGE_KEY);
			if (raw) {
				openSections = normalizeWatchdogOpenSections(
					JSON.parse(raw) as Partial<Record<WatchdogSectionId, boolean>>,
				);
			} else {
				openSections = normalizeWatchdogOpenSections(null);
			}
		} catch {
			openSections = normalizeWatchdogOpenSections(null);
		} finally {
			watchdogSectionsHydrated = true;
		}

		try {
			await loadPingLogs();
			const snap = await api.getTunnelsAll();
			tunnelMeta = snap.tunnels ?? [];
		} catch {
			notifications.error('Не удалось загрузить мониторинг watchdog');
		} finally {
			loading = false;
		}
	});

	onDestroy(() => {
		unsubSingboxStatus?.();
		unsubSingboxTunnels?.();
		unsubSingboxWatchdog?.();
	});

	$effect(() => {
		if (typeof window === 'undefined' || !watchdogSectionsHydrated) return;
		localStorage.setItem(WATCHDOG_SECTIONS_OPEN_STORAGE_KEY, JSON.stringify(openSections));
	});

	$effect(() => {
		const ids = statuses.map((status) => status.tunnelId);
		if (ids.length > 0) void loadConfigs(ids);
	});

	function metaFor(id: string): TunnelListItem | undefined {
		return tunnelMeta.find((tunnel) => tunnel.id === id);
	}

	function configLine(cfg: AWGTunnel['pingCheck'] | undefined, method: string, failThreshold: number): string {
		if (!cfg) return `${method.toUpperCase()} · порог ${failThreshold}`;
		return `${(cfg.method || method).toUpperCase()} → ${cfg.target} · ${cfg.interval}с · порог ${cfg.failThreshold}`;
	}

	const awgCards = $derived.by(() => {
		const pcIds = new Set(statuses.map((status) => status.tunnelId));
		const enabled = statuses.map((status) => {
			const cfg = configs[status.tunnelId];
			const meta = metaFor(status.tunnelId);
			const isWatchdog = status.enabled === true;
			return {
				kind: 'pc' as const,
				id: status.tunnelId,
				name: status.tunnelName,
				backend: status.backend,
				awgVersion: meta?.awgVersion,
				statusKind: status.status,
				isWatchdog,
				configured: isWatchdog || !!cfg,
				configLine: configLine(cfg, status.method, status.failThreshold),
				stats: computeCardStats(logsByTunnel.get(status.tunnelId) ?? [], status),
			};
		});
		const noPc = tunnelMeta
			.filter((tunnel) => !pcIds.has(tunnel.id))
			.map((tunnel) => ({
				kind: 'note' as const,
				id: tunnel.id,
				name: tunnel.name,
				backend: (tunnel as { backend?: 'kernel' | 'nativewg' }).backend ?? 'kernel',
				awgVersion: tunnel.awgVersion,
			}));
		return [...enabled, ...noPc];
	});

	const singboxCards = $derived.by(() => {
		return singboxWatchdogList.map((status) => ({
			id: status.id,
			name: status.name,
			routeHref:
				status.kind === 'subscription'
					? `/subscriptions/${encodeURIComponent(status.ref)}`
					: `/singbox/${encodeURIComponent(status.ref)}`,
			source: status.kind,
			sourceLabel: status.kind === 'subscription' ? 'Подписка' : 'Sing-box',
			tag: status.activeMemberTag || status.checkTag,
			delayCheckTag: status.checkTag,
			primaryHistoryTag: status.checkTag,
			fallbackHistoryTag: status.activeMemberTag || undefined,
			trafficTag: status.trafficTag,
			protocol: status.protocol,
			security: status.security,
			transport: status.transport,
			proxyInterface: status.proxyInterface,
			kernelInterface: status.kernelInterface,
			running: status.running,
			configured: status.configured,
			enabled: status.enabled,
			statusKind: status.status,
			failCount: status.failCount,
			failThreshold: status.failThreshold,
			restartCount: status.restartCount,
			switchCount: status.switchCount,
			lastError: status.lastError,
			lastRecovery: status.lastRecovery,
			status,
			logs: singboxWatchdogLogs[status.id] ?? [],
		} satisfies SingboxWatchdogCardModel));
	});

	function isWatchdogSectionAvailable(id: WatchdogSectionId): boolean {
		return id === 'awg' ? awgCards.length > 0 : singboxSectionVisible;
	}

	function normalizeWatchdogOpenSections(
		value: Partial<Record<WatchdogSectionId, boolean>> | null | undefined,
	): Record<WatchdogSectionId, boolean> {
		return {
			awg: value?.awg ?? true,
			singbox: value?.singbox ?? true,
		};
	}

	function isWatchdogSectionOpen(id: WatchdogSectionId): boolean {
		return isWatchdogSectionAvailable(id) && openSections[id];
	}

	function setWatchdogSectionOpen(id: WatchdogSectionId, open: boolean): void {
		if (!isWatchdogSectionAvailable(id)) return;
		if (openSections[id] === open) return;

		openSections = {
			...openSections,
			[id]: open,
		};
	}

	function toggleWatchdogSection(id: WatchdogSectionId): void {
		setWatchdogSectionOpen(id, !isWatchdogSectionOpen(id));
	}

	const awgWatchdogTotal = $derived(awgCards.length);
	const awgWatchdogActive = $derived(
		awgCards.filter((card) => card.kind === 'pc' && card.isWatchdog).length,
	);
	const singboxWatchdogTotal = $derived(singboxCards.length);
	const singboxWatchdogRunning = $derived(
		singboxCards.filter((card) => card.enabled).length,
	);

	const singboxLoading = $derived.by(() => {
		if (!singboxSectionVisible) return false;
		if (!singboxState.data && (singboxState.status === 'idle' || singboxState.status === 'loading')) return true;
		if (!singboxInstalled) return false;
		return (
			!singboxWatchdogState.data &&
			(singboxWatchdogState.status === 'idle' || singboxWatchdogState.status === 'loading')
		);
	});

	const singboxErrorMessage = $derived.by(() => {
		if (!singboxSectionVisible) return '';
		if (!singboxState.data && singboxState.status === 'error') return 'Не удалось загрузить Sing-box.';
		if (singboxInstalled && !singboxWatchdogState.data && singboxWatchdogState.status === 'error') {
			return 'Не удалось загрузить Sing-box Watchdog.';
		}
		return '';
	});

	function groupSingboxWatchdogLogs(logs: SingboxWatchdogLogEntry[]): Record<string, SingboxWatchdogLogEntry[]> {
		const grouped: Record<string, SingboxWatchdogLogEntry[]> = {};
		for (const entry of logs) {
			if (!entry?.targetId) continue;
			(grouped[entry.targetId] ??= []).push(entry);
		}
		return grouped;
	}

	async function loadSingboxWatchdogLogs(): Promise<void> {
		try {
			const rawLogs = await api.singboxWatchdogLogs();
			const logs = Array.isArray(rawLogs) ? rawLogs : [];
			singboxWatchdogLogs = groupSingboxWatchdogLogs(logs);
			singboxWatchdogLogsLoaded = true;
		} catch {
			// Keep previous logs.
		}
	}

	$effect(() => {
		if (!singboxSectionVisible || !singboxInstalled || singboxCards.length === 0) return;
		const logsKey = singboxCards.map((card) => card.id).sort().join(',');
		if (singboxWatchdogLogsLoaded && singboxWatchdogLogsLoadedKey === logsKey) return;
		singboxWatchdogLogsLoadedKey = logsKey;
		void loadSingboxWatchdogLogs();
	});

	async function openConfig(id: string, name: string, backend: 'kernel' | 'nativewg') {
		editName = name;
		editBackend = backend;
		editNativeStatus = null;
		if (backend === 'nativewg') {
			try {
				editNativeStatus = await api.getNativePingCheckStatus(id);
			} catch {
				editNativeStatus = null;
			}
		}
		editId = id;
	}

	function closeConfig() {
		editId = null;
	}

	function afterSave() {
		editId = null;
		pingCheckStatus.refetch();
		void loadPingLogs();
	}

	async function checkNow() {
		try {
			await api.triggerPingCheck();
			notifications.success('Проверка запущена');
		} catch {
			notifications.error('Не удалось запустить проверку');
		}
	}

	async function enablePingcheck(id: string, name: string, backend: 'kernel' | 'nativewg') {
		try {
			const tunnel = await api.getTunnel(id);
			const pingCheck = tunnel.pingCheck;
			if (!pingCheck) {
				notifications.error('Нет сохранённых настроек мониторинга');
				return;
			}
			if (backend === 'nativewg') {
				const cfg: NativePingCheckConfig = {
					host: pingCheck.target,
					mode: pingCheck.method as 'icmp' | 'connect' | 'tls',
					updateInterval: pingCheck.interval,
					maxFails: pingCheck.failThreshold,
					minSuccess: pingCheck.minSuccess,
					timeout: pingCheck.timeout,
					restart: pingCheck.restart,
				};
				if (pingCheck.port) cfg.port = pingCheck.port;
				await api.configureNativePingCheck(id, cfg);
			} else {
				pingCheck.enabled = true;
				await api.updateTunnel(id, tunnel);
			}
			notifications.success(`Watchdog включён: ${name}`);
			pingCheckStatus.refetch();
			void loadPingLogs();
		} catch {
			notifications.error('Не удалось включить watchdog');
		}
	}

	async function disablePingcheck(id: string, name: string, backend: 'kernel' | 'nativewg') {
		try {
			if (backend === 'nativewg') {
				await api.removeNativePingCheck(id);
			} else {
				const tunnel = await api.getTunnel(id);
				if (tunnel.pingCheck) tunnel.pingCheck.enabled = false;
				await api.updateTunnel(id, tunnel);
			}
			notifications.success(`Watchdog выключен: ${name}`);
			pingCheckStatus.refetch();
			void loadPingLogs();
		} catch {
			notifications.error('Не удалось выключить watchdog');
		}
	}

	function openSingboxWatchdogSettings(target: SingboxWatchdogStatus): void {
		singboxWatchdogDrawerTarget = target;
		singboxWatchdogDrawerOpen = true;
	}

	function closeSingboxWatchdogSettings(): void {
		singboxWatchdogDrawerOpen = false;
		singboxWatchdogDrawerTarget = null;
	}

	function afterSingboxWatchdogSave(): void {
		closeSingboxWatchdogSettings();
		singboxWatchdogStatus.refetch();
		void loadSingboxWatchdogLogs();
	}

	async function checkSingboxWatchdogNow(card: SingboxWatchdogCardModel): Promise<void> {
		try {
			await api.singboxWatchdogCheckNow(card.id);
			await Promise.all([
				singboxWatchdogStatus.refetch(),
				loadSingboxWatchdogLogs(),
			]);
		} catch {
			notifications.error('Не удалось запустить Sing-box watchdog');
		}
	}

	async function enableSingboxWatchdog(card: SingboxWatchdogCardModel): Promise<void> {
		try {
			if (card.configured) {
				await api.singboxWatchdogEnable(card.id);
			} else if (card.status) {
				openSingboxWatchdogSettings(card.status);
				return;
			}
			await singboxWatchdogStatus.refetch();
			void loadSingboxWatchdogLogs();
			notifications.success(`Watchdog включён: ${card.name}`);
		} catch {
			notifications.error('Не удалось включить Sing-box watchdog');
		}
	}

	async function disableSingboxWatchdog(card: SingboxWatchdogCardModel): Promise<void> {
		try {
			await api.singboxWatchdogDisable(card.id);
			await singboxWatchdogStatus.refetch();
			void loadSingboxWatchdogLogs();
			notifications.success(`Watchdog выключен: ${card.name}`);
		} catch {
			notifications.error('Не удалось выключить Sing-box watchdog');
		}
	}
</script>

{#if loading}
	<div class="wd-grid wd-grid--loading">
		{#each Array(4) as _, i (i)}
			<div class="wd-skel"></div>
		{/each}
	</div>
{:else}
	{#if awgCards.length === 0 && !singboxSectionVisible}
		<EmptyState title="Нет туннелей" description="Создайте туннель, чтобы видеть мониторинг." />
	{:else}
		<div class="wd-section-stack">
			{#if awgCards.length > 0}
				<section class="wd-spoiler wd-spoiler--awg">
					<div class="wd-spoiler__header">
						<button
							type="button"
							class="wd-spoiler__summary"
							aria-expanded={isWatchdogSectionOpen('awg')}
							aria-controls="watchdog-section-awg"
							onclick={() => toggleWatchdogSection('awg')}
						>
							<span class="wd-spoiler__title">AWG / NativeWG</span>
							<span class="wd-spoiler__badge">{awgWatchdogTotal}</span>
							<span class="wd-spoiler__meta">{awgWatchdogActive}/{awgWatchdogTotal} watchdog активны</span>
							<span class="wd-spoiler__chevron" aria-hidden="true">
								<ChevronDown size={16} strokeWidth={2.25} />
							</span>
						</button>
					</div>

					{#if isWatchdogSectionOpen('awg')}
						<div id="watchdog-section-awg" class="wd-spoiler__body">
							<div class="wd-grid">
								{#each awgCards as card (card.id)}
									{#if card.kind === 'pc'}
										<WatchdogCard
											name={card.name}
											backend={card.backend}
											awgVersion={card.awgVersion}
											statusKind={card.statusKind}
											hasPingcheck={card.configured}
											isWatchdog={card.isWatchdog}
											configLine={card.configLine}
											stats={card.stats}
											onConfigure={() => openConfig(card.id, card.name, card.backend)}
											onCheckNow={checkNow}
											onDisable={() => disablePingcheck(card.id, card.name, card.backend)}
											onEnable={() => enablePingcheck(card.id, card.name, card.backend)}
										/>
									{:else}
										<WatchdogCard
											name={card.name}
											backend={card.backend}
											awgVersion={card.awgVersion}
											statusKind="disabled"
											hasPingcheck={false}
											isWatchdog={false}
											configLine=""
											stats={null}
											onConfigure={() => openConfig(card.id, card.name, card.backend)}
											onCheckNow={checkNow}
											onDisable={() => {}}
											onEnable={() => {}}
										/>
									{/if}
								{/each}
							</div>
						</div>
					{/if}
				</section>
			{/if}

			{#if singboxSectionVisible}
				<section class="wd-spoiler wd-spoiler--singbox">
					<div class="wd-spoiler__header">
						<button
							type="button"
							class="wd-spoiler__summary"
							aria-expanded={isWatchdogSectionOpen('singbox')}
							aria-controls="watchdog-section-singbox"
							onclick={() => toggleWatchdogSection('singbox')}
						>
							<span class="wd-spoiler__title">Sing-box</span>
							<span class="wd-spoiler__badge">
								{#if !singboxLoading && singboxInstalled}{singboxWatchdogTotal}{:else}—{/if}
							</span>
							<span class="wd-spoiler__meta">
								{#if singboxLoading}
									загрузка данных
								{:else if singboxErrorMessage}
									ошибка загрузки
								{:else if !singboxInstalled}
									не установлен
								{:else}
									{singboxWatchdogRunning}/{singboxWatchdogTotal} активны
								{/if}
							</span>
							<span class="wd-spoiler__chevron" aria-hidden="true">
								<ChevronDown size={16} strokeWidth={2.25} />
							</span>
						</button>
					</div>

					{#if isWatchdogSectionOpen('singbox')}
						<div id="watchdog-section-singbox" class="wd-spoiler__body">
							{#if singboxLoading}
								<div class="wd-notice">Загрузка данных Sing-box…</div>
							{:else if singboxErrorMessage}
								<div class="wd-notice wd-notice--error">{singboxErrorMessage}</div>
							{:else if !singboxInstalled}
								<div class="wd-notice">Sing-box не установлен.</div>
							{:else if singboxCards.length === 0}
								<div class="wd-notice">Нет Sing-box туннелей.</div>
							{:else}
								<div class="wd-grid">
									{#each singboxCards as card (card.id)}
										<SingboxWatchdogCard
											{card}
											onConfigure={() => card.status && openSingboxWatchdogSettings(card.status)}
											onCheckNow={() => checkSingboxWatchdogNow(card)}
											onDisable={() => disableSingboxWatchdog(card)}
											onEnable={() => enableSingboxWatchdog(card)}
										/>
									{/each}
								</div>
							{/if}
						</div>
					{/if}
				</section>
			{/if}
		</div>
	{/if}
{/if}

{#if editId && editBackend === 'kernel'}
	<KernelPingCheckModal open={true} tunnelId={editId} tunnelName={editName} onclose={closeConfig} onSaved={afterSave} />
{:else if editId && editBackend === 'nativewg'}
	<NativeWGPingCheckModal
		open={true}
		tunnelId={editId}
		tunnelName={editName}
		status={editNativeStatus}
		onclose={closeConfig}
		onSaved={afterSave}
		onRemoved={afterSave}
	/>
{/if}

<SingboxWatchdogSettingsDrawer
	open={singboxWatchdogDrawerOpen}
	target={singboxWatchdogDrawerTarget}
	onclose={closeSingboxWatchdogSettings}
	onSaved={afterSingboxWatchdogSave}
/>

<style>
	.wd-grid {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: 16px;
	}

	.wd-grid--loading {
		padding-top: 16px;
	}

	.wd-section-stack {
		display: flex;
		flex-direction: column;
		gap: 0.875rem;
		padding-top: 0.35rem;
	}

	.wd-spoiler {
		position: relative;
		border: 1px solid var(--section-border, var(--color-border));
		border-radius: 12px;
		background:
			linear-gradient(135deg, var(--section-tint, transparent) 0%, transparent 42%),
			var(--color-bg-tertiary);
		overflow: hidden;
	}

	.wd-spoiler::before {
		content: '';
		position: absolute;
		inset: 0 auto 0 0;
		width: 2px;
		background: color-mix(in srgb, var(--section-rail, var(--color-accent)) 70%, transparent);
		opacity: 0.75;
		pointer-events: none;
	}

	.wd-spoiler--awg {
		--section-tint: color-mix(in srgb, var(--color-accent) 10%, transparent);
		--section-tint-strong: color-mix(in srgb, var(--color-accent) 18%, transparent);
		--section-border: color-mix(in srgb, var(--color-accent) 28%, var(--color-border));
		--section-badge-bg: color-mix(in srgb, var(--color-accent) 88%, #ffffff 12%);
		--section-rail: var(--color-accent);
	}

	.wd-spoiler--singbox {
		--section-tint: color-mix(in srgb, #22c55e 8%, transparent);
		--section-tint-strong: color-mix(in srgb, #22c55e 14%, transparent);
		--section-border: color-mix(in srgb, #22c55e 24%, var(--color-border));
		--section-badge-bg: color-mix(in srgb, #22c55e 76%, var(--color-accent) 24%);
		--section-rail: #22c55e;
	}

	.wd-spoiler__header {
		background:
			linear-gradient(90deg, var(--section-tint-strong, transparent) 0%, transparent 55%),
			color-mix(in srgb, var(--color-bg-tertiary) 82%, var(--color-bg-secondary) 18%);
	}

	.wd-spoiler__summary {
		width: 100%;
		display: grid;
		grid-template-columns: auto auto minmax(0, 1fr) 2rem;
		align-items: center;
		gap: 0.625rem;
		min-height: 3rem;
		padding: 0.875rem 1rem;
		border: 0;
		background: transparent;
		color: var(--color-text-primary);
		text-align: left;
		cursor: pointer;
		transition:
			background 0.16s ease,
			box-shadow 0.16s ease;
	}

	.wd-spoiler__summary:hover {
		background:
			linear-gradient(90deg, var(--section-tint-strong, transparent) 0%, transparent 55%),
			color-mix(in srgb, var(--color-bg-tertiary) 70%, var(--color-bg-secondary) 30%);
	}

	.wd-spoiler__summary:focus-visible {
		outline: none;
		box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--color-accent) 42%, transparent);
	}

	.wd-spoiler__summary[aria-expanded="true"] {
		background:
			linear-gradient(90deg, var(--section-tint-strong, transparent) 0%, transparent 58%),
			color-mix(in srgb, var(--color-bg-tertiary) 62%, var(--color-bg-secondary) 38%);
	}

	.wd-spoiler__title {
		font-weight: 700;
	}

	.wd-spoiler__badge {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 1.25rem;
		height: 1.25rem;
		padding: 0 0.375rem;
		border-radius: 999px;
		background: var(--section-badge-bg, var(--color-accent));
		color: var(--color-accent-contrast, #fff);
		font-size: 0.6875rem;
		font-weight: 700;
		box-shadow: 0 0 0 1px color-mix(in srgb, var(--section-rail, var(--color-accent)) 30%, transparent);
	}

	.wd-spoiler__meta {
		min-width: 0;
		color: var(--color-text-muted);
		font-size: 0.8125rem;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.wd-spoiler__chevron {
		justify-self: end;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 2rem;
		height: 2rem;
		border-radius: 0.5rem;
		color: var(--color-text-muted);
		border: 1px solid transparent;
		background: transparent;
		transition:
			color 0.16s ease,
			background 0.16s ease,
			border-color 0.16s ease;
	}

	.wd-spoiler__chevron :global(svg) {
		display: block;
		transition: transform 0.16s ease;
	}

	.wd-spoiler__summary:hover .wd-spoiler__chevron {
		color: var(--color-text-primary);
		background: color-mix(in srgb, var(--section-rail, var(--color-accent)) 10%, transparent);
		border-color: color-mix(in srgb, var(--section-rail, var(--color-accent)) 22%, transparent);
	}

	.wd-spoiler__summary[aria-expanded="true"] .wd-spoiler__chevron {
		color: var(--section-rail, var(--color-accent));
		background: color-mix(in srgb, var(--section-rail, var(--color-accent)) 12%, transparent);
		border-color: color-mix(in srgb, var(--section-rail, var(--color-accent)) 26%, transparent);
	}

	.wd-spoiler__summary[aria-expanded="true"] .wd-spoiler__chevron :global(svg) {
		transform: rotate(180deg);
	}

	.wd-spoiler__body {
		padding: 0.75rem 1rem 1rem;
		background: transparent;
	}

	.wd-notice {
		padding: 14px 16px;
		border: 1px dashed var(--color-border);
		border-radius: 12px;
		color: var(--color-text-muted);
		background: color-mix(in srgb, var(--color-border) 10%, transparent);
	}

	.wd-notice--error {
		border-color: var(--color-error-border);
		color: var(--color-error);
		background: var(--color-error-tint);
	}

	.wd-skel {
		height: 220px;
		border-radius: 12px;
		background: var(--color-surface-2, rgba(255, 255, 255, 0.03));
		animation: wd-pulse 1.4s ease-in-out infinite;
	}

	@keyframes wd-pulse {
		0%, 100% { opacity: 0.4; }
		50% { opacity: 0.7; }
	}

	@media (max-width: 640px) {
		.wd-grid {
			grid-template-columns: 1fr;
		}

		.wd-spoiler__summary {
			grid-template-columns: minmax(0, auto) auto minmax(0, 1fr) 2rem;
			column-gap: 0.5rem;
			row-gap: 0.35rem;
			align-items: center;
		}

		.wd-spoiler__title {
			grid-column: 1;
			min-width: 0;
			white-space: nowrap;
			overflow: hidden;
			text-overflow: ellipsis;
		}

		.wd-spoiler__badge {
			grid-column: 2;
			grid-row: 1;
			justify-self: start;
		}

		.wd-spoiler__chevron {
			grid-column: 4;
			grid-row: 1;
		}

		.wd-spoiler__meta {
			grid-column: 1 / -1;
			grid-row: 2;
			white-space: normal;
		}
	}
</style>
