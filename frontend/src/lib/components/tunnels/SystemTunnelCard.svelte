<script lang="ts">
	import { untrack } from 'svelte';
	import type { SystemTunnel, ConnectivityResult } from '$lib/types';
	import { api } from '$lib/api/client';
	import { formatRelativeTime, formatDuration, formatBytes, formatBitRate } from '$lib/utils/format';
	import { TrafficChart, TrafficSparkline, Button, Badge, PingButton } from '$lib/components/ui';
	import TunnelTestIcon from './TunnelTestIcon.svelte';
	import { getTrafficRates, subscribeTraffic, loadHistory } from '$lib/stores/traffic';

	interface Props {
		tunnel: SystemTunnel;
		view?: 'cards' | 'compact' | 'list';
		onMarkServer?: (id: string) => void;
		ondetail?: (id: string) => void;
		ontest: (id: string, name: string) => void;
	}

	let { tunnel, view = 'cards', onMarkServer, ondetail, ontest }: Props = $props();

	let connectivity = $state<ConnectivityResult | null>(null);
	let checking = $state(false);
	let showEndpoint = $state(false);

	// Connectivity check toggle (persisted in localStorage)
	const CC_KEY_PREFIX = 'systunnel_cc_disabled_';
	// svelte-ignore state_referenced_locally — intentional: initial value from localStorage
	let checkDisabled = $state(localStorage.getItem(CC_KEY_PREFIX + tunnel.id) === 'true');

	function toggleCheckDisabled() {
		checkDisabled = !checkDisabled;
		localStorage.setItem(CC_KEY_PREFIX + tunnel.id, String(checkDisabled));
		if (checkDisabled) {
			connectivity = null;
		}
	}

	async function checkConnectivity() {
		if (tunnel.status !== 'up' || checking || checkDisabled) return;
		checking = true;
		try {
			connectivity = await api.checkSystemTunnelConnectivity(tunnel.id);
		} catch {
			connectivity = null;
		} finally {
			checking = false;
		}
	}

	// Auto-check connectivity every 60s when up
	$effect(() => {
		const status = tunnel.status;
		const disabled = checkDisabled;
		if (status !== 'up' || disabled) {
			connectivity = null;
			return;
		}
		untrack(() => checkConnectivity());
		const interval = setInterval(checkConnectivity, 60000);
		return () => clearInterval(interval);
	});

	// LED color
	const ledClass = $derived(
		tunnel.status !== 'up' ? 'led-gray' :
		tunnel.peer?.online ? 'led-green' : 'led-yellow'
	);

	// Traffic chart — live only (no server history for system tunnels)
	let rxRates = $state<number[]>([]);
	let txRates = $state<number[]>([]);

	let initialLoadDone = false;
	$effect(() => {
		const id = tunnel.id;
		if (initialLoadDone) return;
		initialLoadDone = true;
		untrack(() => loadHistory(id));
	});

	$effect(() => {
		const id = tunnel.id;
		const update = () => {
			const t = getTrafficRates(id);
			rxRates = t.rx;
			txRates = t.tx;
		};
		update();
		return subscribeTraffic(update);
	});

	// Collapsible chart (persisted in localStorage, separate prefix from managed cards)
	const CHART_KEY_PREFIX = 'chart_expanded_systunnel_';
	// svelte-ignore state_referenced_locally — intentional: initial value from localStorage
	let chartExpanded = $state(localStorage.getItem(CHART_KEY_PREFIX + tunnel.id) !== 'false');

	function toggleChart() {
		chartExpanded = !chartExpanded;
		localStorage.setItem(CHART_KEY_PREFIX + tunnel.id, String(chartExpanded));
	}

	let chartHeight = $derived(view === 'compact' ? 76 : 100);

	let inlineRxRate = $derived(rxRates.length > 0 ? rxRates[rxRates.length - 1] : 0);
	let inlineTxRate = $derived(txRates.length > 0 ? txRates[txRates.length - 1] : 0);

	let listStatusText = $derived(tunnel.status === 'up' ? (tunnel.peer?.online ? 'Активен' : 'Без handshake') : 'Выключен');

	let isDenseCard = $derived(view === 'cards');
	let isCompactCard = $derived(view === 'compact');

	type ConnectivityState = 'idle' | 'connected' | 'disconnected' | 'checking';
	let connState = $derived.by<ConnectivityState>(() => {
		if (tunnel.status !== 'up' || checkDisabled) return 'idle';
		if (checking || connectivity === null) return 'checking';
		return connectivity.connected ? 'connected' : 'disconnected';
	});
	let latencyMs = $derived(connectivity?.latency ?? null);
	let showConnectivityRow = $derived(tunnel.status === 'up');
	let showPingButton = $derived(showConnectivityRow && !checkDisabled);
	let compactStatusHint = $derived(
		isCompactCard && tunnel.status === 'up' && !tunnel.peer?.online ? 'Без handshake' : '',
	);

	let displayName = $derived(tunnel.description || tunnel.id);

	function openTest(): void {
		ontest(tunnel.id, displayName);
	}
</script>

{#if view === 'list'}
	<div class="card list-card" class:status-up={tunnel.status === 'up'} class:status-down={tunnel.status !== 'up'}>
		<div class="list-cell list-cell-primary">
			<h3 class="tunnel-name" title={tunnel.description || tunnel.id}>{tunnel.description || tunnel.id}</h3>
			<div class="flex items-center gap-2 flex-wrap">
				<span class="iface-name">{tunnel.interfaceName}</span>
				<span class="version-badge badge-system">Системный</span>
			</div>
			<div class="list-note">{tunnel.address || '—'}{#if tunnel.peer?.via}<span class="list-note-sep">·</span>{tunnel.peer.via}{/if}</div>
		</div>

		<div class="list-cell list-cell-status">
			<span class="list-label">Статус</span>
			<div class="list-status-main">
				<span class="led {ledClass}"></span>
				<span class="list-status-text">{listStatusText}</span>
			</div>
			{#if showConnectivityRow}
				<div class="connectivity-row">
					{#if showPingButton}
						<PingButton
							connectivity={connState}
							{latencyMs}
							checking={checking}
							onclick={checkConnectivity}
						/>
					{/if}
					<button
						class="connectivity-gear"
						class:gear-disabled={checkDisabled}
						onclick={toggleCheckDisabled}
						title={checkDisabled ? 'Проверка связности выключена. Нажмите для включения' : 'Выключить проверку связности'}
					>
						<svg width="14" height="14" viewBox="0 0 20 20" fill="currentColor">
							<path fill-rule="evenodd" d="M7.84 1.804A1 1 0 018.82 1h2.36a1 1 0 01.98.804l.331 1.652a6.993 6.993 0 011.929 1.115l1.598-.54a1 1 0 011.186.447l1.18 2.044a1 1 0 01-.205 1.251l-1.267 1.113a7.047 7.047 0 010 2.228l1.267 1.113a1 1 0 01.206 1.25l-1.18 2.045a1 1 0 01-1.187.447l-1.598-.54a6.993 6.993 0 01-1.929 1.115l-.33 1.652a1 1 0 01-.98.804H8.82a1 1 0 01-.98-.804l-.331-1.652a6.993 6.993 0 01-1.929-1.115l-1.598.54a1 1 0 01-1.186-.447l-1.18-2.044a1 1 0 01.205-1.251l1.267-1.114a7.05 7.05 0 010-2.227L1.821 7.773a1 1 0 01-.206-1.25l1.18-2.045a1 1 0 011.187-.447l1.598.54A6.993 6.993 0 017.51 3.456l.33-1.652zM10 13a3 3 0 100-6 3 3 0 000 6z" clip-rule="evenodd" />
						</svg>
					</button>
				</div>
			{/if}
		</div>

		<div class="list-cell list-cell-endpoint">
			<span class="list-label">Endpoint</span>
			<div class="flex items-center gap-1 min-w-0">
				<span class="detail-value truncate" title={showEndpoint ? tunnel.peer?.endpoint : ''}>{showEndpoint ? (tunnel.peer?.endpoint || '—') : '•••••••••'}</span>
				{#if tunnel.peer?.endpoint}
					<button
						class="eye-btn"
						onclick={() => showEndpoint = !showEndpoint}
						title={showEndpoint ? 'Скрыть' : 'Показать'}
					>
						{#if showEndpoint}
							<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
						{:else}
							<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
						{/if}
					</button>
				{/if}
			</div>
			<div class="list-note">MTU {tunnel.mtu}</div>
		</div>

		<div class="list-cell list-cell-traffic">
			<span class="list-label">Трафик</span>
			{#if tunnel.status === 'up'}
				<div class="list-traffic-chart">
					<TrafficChart
						{rxRates}
						{txRates}
						rxTotal={tunnel.peer?.rxBytes ?? 0}
						txTotal={tunnel.peer?.txBytes ?? 0}
						height={36}
						onclick={() => ondetail?.(tunnel.id)}
					/>
				</div>
			{:else}
				<div class="list-traffic-empty">Нет данных</div>
			{/if}
			<div class="list-note">↓ {formatBytes(tunnel.peer?.rxBytes ?? 0)} · ↑ {formatBytes(tunnel.peer?.txBytes ?? 0)}</div>
		</div>

		<div class="list-cell list-cell-stats">
			<span class="list-label">Активность</span>
			<div class="list-stat-row">
				<span>Handshake</span>
				<strong>{tunnel.peer?.lastHandshake ? formatRelativeTime(tunnel.peer.lastHandshake) : '—'}</strong>
			</div>
			<div class="list-stat-row">
				<span>Uptime</span>
				<strong>{tunnel.uptime ? formatDuration(tunnel.uptime) : '—'}</strong>
			</div>
		</div>

		<div class="list-cell list-cell-actions">
			<div class="actions-row list-actions-row">
				<Button variant="ghost" size="sm" href="/system-tunnels/{tunnel.id}">Изменить</Button>

				<span class="system-action-test">
					<Button variant="ghost" size="sm" onclick={openTest}>
						{#snippet iconBefore()}
							<TunnelTestIcon size={14} />
						{/snippet}
						Тест
					</Button>
				</span>

				{#if onMarkServer}
					<span class="system-action-primary">
						<Button variant="ghost" size="sm" onclick={() => onMarkServer?.(tunnel.id)}>В серверы</Button>
					</span>
				{/if}
			</div>
		</div>
	</div>
{:else}
	<div
		class="card flex flex-col transition-[border-color] duration-200"
		class:status-up={tunnel.status === 'up'}
		class:status-down={tunnel.status !== 'up'}
		class:view-compact={view === 'compact'}
		class:view-dense={view === 'cards'}
	>
		<!-- Header: name + status + connectivity -->
		{#if isDenseCard}
			<div class="header header-dense">
				<div class="header-dense-body">
					<div class="title-line-dense">
						<button
							type="button"
							class="tunnel-name tunnel-name-dense"
							title={displayName}
							onclick={() => ondetail?.(tunnel.id)}
						>
							{displayName}
						</button>
					</div>
					<div class="meta-tags-dense">
						<Badge variant="info" size="sm">Системный</Badge>
						<span class="iface-chip-dense" title={tunnel.interfaceName}>{tunnel.interfaceName}</span>
					</div>
				</div>
				<div class="dense-toolbar">
					{#if showConnectivityRow}
						<div class="dense-toolbar-bottom">
							<span class="led {ledClass}"></span>
							{#if showPingButton}
								<PingButton
									connectivity={connState}
									{latencyMs}
									checking={checking}
									size="sm"
									onclick={checkConnectivity}
								/>
							{/if}
							<button
								class="connectivity-gear"
								class:gear-disabled={checkDisabled}
								onclick={toggleCheckDisabled}
								title={checkDisabled ? 'Проверка связности выключена. Нажмите для включения' : 'Выключить проверку связности'}
							>
								<svg width="11" height="11" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
									<path fill-rule="evenodd" d="M7.84 1.804A1 1 0 018.82 1h2.36a1 1 0 01.98.804l.331 1.652a6.993 6.993 0 011.929 1.115l1.598-.54a1 1 0 011.186.447l1.18 2.044a1 1 0 01-.205 1.251l-1.267 1.113a7.047 7.047 0 010 2.228l1.267 1.113a1 1 0 01.206 1.25l-1.18 2.045a1 1 0 01-1.187.447l-1.598-.54a6.993 6.993 0 01-1.929 1.115l-.33 1.652a1 1 0 01-.98.804H8.82a1 1 0 01-.98-.804l-.331-1.652a6.993 6.993 0 01-1.929-1.115l-1.598.54a1 1 0 01-1.186-.447l-1.18-2.044a1 1 0 01.205-1.251l1.267-1.114a7.05 7.05 0 010-2.227L1.821 7.773a1 1 0 01-.206-1.25l1.18-2.045a1 1 0 011.187-.447l1.598.54A6.993 6.993 0 017.51 3.456l.33-1.652zM10 13a3 3 0 100-6 3 3 0 000 6z" clip-rule="evenodd" />
								</svg>
							</button>
						</div>
					{:else}
						<div class="dense-toolbar-top">
							<span class="led {ledClass}"></span>
						</div>
					{/if}
				</div>
			</div>
		{:else}
			<div class="header">
				<div class="head-left">
					<div class="title-line">
						<button
							type="button"
							class="tunnel-name"
							title={displayName}
							onclick={() => ondetail?.(tunnel.id)}
						>
							{displayName}
						</button>
					</div>
					<div class="meta-line">
						<span class="iface-name">{tunnel.interfaceName}</span>
						<span class="version-badge badge-system">Системный</span>
					</div>
					{#if compactStatusHint}
						<span class="status-hint-left">{compactStatusHint}</span>
					{/if}
				</div>
				<div class="head-right">
					{#if showConnectivityRow}
						<div class="connectivity-row">
							<span class="led {ledClass}"></span>
							{#if showPingButton}
								<PingButton
									connectivity={connState}
									{latencyMs}
									checking={checking}
									onclick={checkConnectivity}
								/>
							{/if}
							<button
								class="connectivity-gear"
								class:gear-disabled={checkDisabled}
								onclick={toggleCheckDisabled}
								title={checkDisabled ? 'Проверка связности выключена. Нажмите для включения' : 'Выключить проверку связности'}
							>
								<svg width="14" height="14" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
									<path fill-rule="evenodd" d="M7.84 1.804A1 1 0 018.82 1h2.36a1 1 0 01.98.804l.331 1.652a6.993 6.993 0 011.929 1.115l1.598-.54a1 1 0 011.186.447l1.18 2.044a1 1 0 01-.205 1.251l-1.267 1.113a7.047 7.047 0 010 2.228l1.267 1.113a1 1 0 01.206 1.25l-1.18 2.045a1 1 0 01-1.187.447l-1.598-.54a6.993 6.993 0 01-1.929 1.115l-.33 1.652a1 1 0 01-.98.804H8.82a1 1 0 01-.98-.804l-.331-1.652a6.993 6.993 0 01-1.929-1.115l-1.598.54a1 1 0 01-1.186-.447l-1.18-2.044a1 1 0 01.205-1.251l1.267-1.114a7.05 7.05 0 010-2.227L1.821 7.773a1 1 0 01-.206-1.25l1.18-2.045a1 1 0 011.187-.447l1.598.54A6.993 6.993 0 017.51 3.456l.33-1.652zM10 13a3 3 0 100-6 3 3 0 000 6z" clip-rule="evenodd" />
								</svg>
							</button>
						</div>
					{:else}
						<div class="led-row">
							<span class="led {ledClass}"></span>
						</div>
					{/if}
				</div>
			</div>
		{/if}

		<!-- Details: endpoint + via + IPv4 + uptime + handshake -->
		<div class="details">
			{#if view === 'cards'}
				<div class="details-dense-cols">
					<div class="details-dense-col">
						{#if tunnel.peer?.endpoint}
							<div class="kv-stacked-stat">
								<span class="kv-stacked-label">Сервер</span>
								<span class="kv-endpoint">
									<span
										class="kv-stacked-value truncate"
										title={showEndpoint ? tunnel.peer.endpoint : ''}
									>
										{showEndpoint ? tunnel.peer.endpoint : '•••••••••'}
									</span>
									<button
										class="eye-btn"
										onclick={() => showEndpoint = !showEndpoint}
										title={showEndpoint ? 'Скрыть' : 'Показать'}
									>
										{#if showEndpoint}
											<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
										{:else}
											<svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
										{/if}
									</button>
								</span>
							</div>
						{/if}
						{#if tunnel.peer?.via}
							<div class="kv-stacked-stat">
								<span class="kv-stacked-label">Подключение</span>
								<span class="kv-stacked-value" title={tunnel.peer.via}>{tunnel.peer.via}</span>
							</div>
						{/if}
						{#if tunnel.address}
							<div class="kv-stacked-stat">
								<span class="kv-stacked-label">IPv4</span>
								<span class="kv-stacked-value">{tunnel.address}</span>
							</div>
						{/if}
					</div>
					<div class="details-dense-col details-dense-col-right">
						<div class="kv-stacked-stat">
							<span class="kv-stacked-label">MTU</span>
							<span class="kv-stacked-value">{tunnel.mtu}</span>
						</div>
						{#if tunnel.status === 'up'}
							<div class="kv-stacked-stat">
								<span class="kv-stacked-label">Uptime</span>
								<span class="kv-stacked-value">
									{tunnel.uptime ? formatDuration(tunnel.uptime) : '—'}
								</span>
							</div>
							<div class="kv-stacked-stat">
								<span class="kv-stacked-label">Handshake</span>
								<span class="kv-stacked-value">
									{tunnel.peer?.lastHandshake
										? formatRelativeTime(tunnel.peer.lastHandshake)
										: '—'}
								</span>
							</div>
						{/if}
					</div>
				</div>
			{:else}
			{#if tunnel.peer?.endpoint}
				<div class="flex gap-4 items-start">
					<div class="flex flex-col gap-0.5 min-w-0 flex-1">
						<span class="detail-label">Endpoint</span>
						<span class="flex items-center gap-1 min-w-0">
							<span class="detail-value truncate" title={showEndpoint ? tunnel.peer.endpoint : ''}>{showEndpoint ? tunnel.peer.endpoint : '•••••••••'}</span>
							<button
								class="eye-btn"
								onclick={() => showEndpoint = !showEndpoint}
								title={showEndpoint ? 'Скрыть' : 'Показать'}
							>
								{#if showEndpoint}
									<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
								{:else}
									<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
								{/if}
							</button>
						</span>
					</div>
				</div>
			{/if}
			{#if tunnel.peer?.via}
				<div class="flex gap-4 items-start">
					<div class="flex flex-col gap-0.5 min-w-0 flex-1">
						<span class="detail-label">Подключение</span>
						<span class="detail-value">{tunnel.peer.via}</span>
					</div>
				</div>
			{/if}
			{#if tunnel.address}
				<div class="flex gap-4 items-start">
					<div class="flex flex-col gap-0.5 min-w-0 flex-1">
						<span class="detail-label">IPv4</span>
						<span class="detail-value">{tunnel.address}</span>
					</div>
				</div>
			{/if}
			{#if tunnel.status === 'up' && (tunnel.uptime || tunnel.peer?.lastHandshake)}
				<hr class="divider" />
				<div class="flex items-start stats-row">
					<div class="flex flex-col gap-0.5 min-w-0 flex-1">
						<span class="detail-label">Uptime</span>
						<span class="detail-value text-[11px] whitespace-nowrap">
							{tunnel.uptime ? formatDuration(tunnel.uptime) : '—'}
						</span>
					</div>
					<div class="flex flex-col gap-0.5 min-w-0 flex-1 items-end">
						<span class="detail-label">Handshake</span>
						<span class="detail-value text-[11px] whitespace-nowrap">
							{tunnel.peer?.lastHandshake ? formatRelativeTime(tunnel.peer.lastHandshake) : '—'}
						</span>
					</div>
				</div>
			{/if}
			{/if}
		</div>

		<!-- Actions -->
		<div class="actions-wrapper">
			<div class="actions-row">
				<Button variant="ghost" href="/system-tunnels/{tunnel.id}">
					{#snippet iconBefore()}
						<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
							<path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
						</svg>
					{/snippet}
					Изменить
				</Button>

				<span class="system-action-test">
					<Button variant="ghost" onclick={openTest}>
						{#snippet iconBefore()}
							<TunnelTestIcon size={16} />
						{/snippet}
						Тест
					</Button>
				</span>

				{#if onMarkServer}
					<span class="system-action-primary">
						<Button variant="ghost" onclick={() => onMarkServer?.(tunnel.id)}>
							{#snippet iconBefore()}
								<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
									<rect x="2" y="2" width="20" height="8" rx="2" ry="2"/>
									<rect x="2" y="14" width="20" height="8" rx="2" ry="2"/>
									<line x1="6" y1="6" x2="6.01" y2="6"/>
									<line x1="6" y1="18" x2="6.01" y2="18"/>
								</svg>
							{/snippet}
							В серверы
						</Button>
					</span>
				{/if}
			</div>
		</div>

		<!-- Traffic -->
		{#if tunnel.status === 'up'}
			{#if view === 'cards'}
				<button
					type="button"
					class="traffic-inline"
					onclick={() => ondetail?.(tunnel.id)}
					title="Открыть график трафика"
				>
					<TrafficSparkline
						rxData={rxRates}
						txData={txRates}
						width={84}
						height={22}
					/>
					<div class="traffic-inline-rates">
						<span class="traffic-inline-rate rx">↓ {formatBitRate(inlineRxRate)}</span>
						<span class="traffic-inline-rate tx">↑ {formatBitRate(inlineTxRate)}</span>
					</div>
				</button>
			{:else}
				<div class="chart-section">
					<button type="button" class="chart-header" onclick={toggleChart}>
						<span class="chart-label">Трафик</span>
						<span class="chart-chevron" class:expanded={chartExpanded}>▾</span>
					</button>
					<div class="chart-body" class:expanded={chartExpanded}>
						<TrafficChart
							{rxRates}
							{txRates}
							rxTotal={tunnel.peer?.rxBytes ?? 0}
							txTotal={tunnel.peer?.txBytes ?? 0}
							height={chartHeight}
							onclick={() => ondetail?.(tunnel.id)}
						/>
					</div>
				</div>
			{/if}
		{/if}
	</div>
{/if}

<style>
	/* Match TunnelCard border states */
	.status-up {
		border-color: var(--success);
	}

	.status-down {
		border-color: var(--text-muted, #6b7280);
	}

	.list-card {
		display: grid;
		grid-template-columns: minmax(220px, 1.3fr) minmax(170px, 0.9fr) minmax(220px, 1.1fr) minmax(180px, 1fr) minmax(150px, 0.9fr) auto;
		gap: 14px;
		align-items: center;
		padding: 12px 14px;
	}

	.list-cell {
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.list-label {
		font-size: 10px;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-muted);
	}

	.list-note {
		font-size: 11px;
		color: var(--text-muted);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.list-note-sep {
		padding: 0 4px;
	}

	.list-status-main {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.list-status-text {
		font-size: 12px;
		font-weight: 600;
		color: var(--text-primary);
	}

	.list-traffic-chart {
		min-height: 36px;
		padding: 2px 0;
	}

	.list-traffic-empty {
		font-size: 12px;
		color: var(--text-muted);
		padding: 8px 0;
	}

	.list-stat-row {
		display: flex;
		justify-content: space-between;
		gap: 10px;
		font-size: 11px;
		color: var(--text-muted);
	}

	.list-stat-row strong {
		font-size: 12px;
		font-weight: 600;
		color: var(--text-secondary);
		white-space: nowrap;
	}

	.list-actions-row {
		flex-direction: column;
		align-items: stretch;
	}

	.card.flex {
		gap: 1rem;
	}

	.card.view-compact {
		gap: 12px;
		padding: 12px 14px;
	}

	.card.view-dense {
		gap: 8px;
		padding: 10px 12px;
	}

	.card.view-dense .details {
		gap: 6px;
	}

	.title-line-dense {
		display: flex;
		align-items: baseline;
		gap: 6px;
		min-width: 0;
	}

	.tunnel-name-dense {
		flex: 1 1 auto;
		min-width: 0;
		font-size: 0.9rem;
		font-weight: 600;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.meta-tags-dense {
		display: flex;
		flex-wrap: wrap;
		margin-top: 4px;
		align-items: center;
		gap: 3px;
		min-width: 0;
		overflow: hidden;
	}

	.card.view-dense .meta-tags-dense :global(.badge) {
		font-size: 9px;
		padding: 1px 5px;
		line-height: 1.3;
		flex-shrink: 0;
	}

	.iface-chip-dense {
		display: inline-block;
		min-width: 0;
		flex-shrink: 1;
		font-size: 9px;
		font-weight: 500;
		font-family: var(--font-mono, monospace);
		line-height: 1.3;
		padding: 1px 5px;
		border-radius: var(--radius-sm);
		border: 1px solid var(--color-border);
		background: var(--color-bg-tertiary);
		color: var(--text-muted);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 10px;
	}

	.header.header-dense {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: flex-start;
		gap: 6px;
	}

	.header-dense-body {
		display: flex;
		flex-direction: column;
		gap: 1px;
		min-width: 0;
	}

	.head-left {
		display: flex;
		flex-direction: column;
		gap: 4px;
		min-width: 0;
	}

	.title-line {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
	}

	.head-right {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 4px;
		flex-shrink: 0;
	}

	.led-row {
		display: flex;
		align-items: center;
		justify-content: flex-end;
	}

	.dense-toolbar {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		flex-shrink: 0;
	}

	.dense-toolbar-top {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.dense-toolbar-bottom {
		display: flex;
		align-items: center;
		/* gap: 2px; */
	}

	.meta-line {
		display: flex;
		align-items: center;
		gap: 6px;
		flex-wrap: wrap;
	}

	.connectivity-row {
		display: flex;
		align-items: center;
		gap: 5px;
	}

	.connectivity-row .led,
	.dense-toolbar-bottom .led {
		flex-shrink: 0;
	}

	.card.view-dense .dense-toolbar-bottom .connectivity-gear {
		width: 16px;
		height: 16px;
		padding: 0;
	}

	.card.view-dense .dense-toolbar-top .led,
	.card.view-dense .dense-toolbar .led {
		width: 6px;
		height: 6px;
	}

	.card.view-compact .led {
		width: 8px;
		height: 8px;
	}

	.status-hint-left {
		align-self: flex-start;
		font-size: 11px;
		color: var(--color-warning, var(--warning, #f59e0b));
	}

	.details-dense-cols {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		gap: 10px 12px;
		align-items: end;
	}

	.details-dense-col {
		display: flex;
		flex-direction: column;
		gap: 6px;
		min-width: 0;
	}

	.details-dense-col-right {
		min-width: 4.75rem;
	}

	.kv-stacked-stat {
		display: flex;
		flex-direction: column;
		gap: 1px;
		min-width: 0;
	}

	.card.view-dense .kv-endpoint {
		display: flex;
		align-items: center;
		gap: 2px;
		min-width: 0;
	}

	.kv-stacked-label {
		font-size: 9px;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--text-muted);
		line-height: 1.2;
	}

	.kv-stacked-value {
		font-size: 10px;
		font-family: var(--font-mono, monospace);
		color: var(--text-secondary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		line-height: 1.25;
	}

	.traffic-inline {
		display: flex;
		align-items: center;
		gap: 0.45rem;
		width: 100%;
		min-width: 0;
		padding: 5px 6px;
		margin: 0;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-sm);
		background: var(--color-bg-secondary);
		cursor: pointer;
		font: inherit;
		color: inherit;
		text-align: left;
		transition: background 0.15s ease, border-color 0.15s ease;
	}

	.traffic-inline:hover {
		background: var(--color-bg-hover);
		border-color: var(--color-border-hover);
	}

	.traffic-inline:focus-visible {
		outline: 2px solid var(--color-accent);
		outline-offset: 2px;
	}

	.traffic-inline-rates {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 0.08rem;
		padding-block: 3px;
		min-width: 0;
		flex-shrink: 0;
		font-size: 10px;
		line-height: 1.15;
		font-family: var(--font-mono, monospace);
		font-variant-numeric: tabular-nums;
	}

	.traffic-inline-rate.rx {
		color: var(--color-accent);
	}

	.traffic-inline-rate.tx {
		color: var(--color-success);
	}

	.card.view-dense .actions-wrapper {
		padding-top: 8px;
	}

	.card.view-dense .actions-row :global(button),
	.card.view-dense .actions-row :global(a) {
		padding: 0.25rem 0.5rem !important;
		font-size: 0.6875rem !important;
		min-height: 0 !important;
	}

	.card.view-dense .actions-row :global(button svg),
	.card.view-dense .actions-row :global(a svg) {
		width: 12px !important;
		height: 12px !important;
	}

	.card.view-list {
		display: grid;
		grid-template-columns: minmax(0, 1.35fr) minmax(280px, 1fr) auto;
		gap: 12px 16px;
		align-items: start;
		padding: 12px 14px;
	}

	/* Tunnel name */
	.tunnel-name {
		background: none;
		border: none;
		padding: 0;
		font: inherit;
		font-size: 15px;
		font-weight: 600;
		color: var(--color-text-primary, var(--text-primary));
		text-align: left;
		cursor: pointer;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		min-width: 0;
	}

	.tunnel-name:hover {
		color: var(--color-accent, var(--accent));
	}

	.card.view-compact .tunnel-name {
		font-size: 14px;
	}

	.iface-name {
		font-size: 12px;
		font-family: var(--font-mono, monospace);
		color: var(--text-muted);
	}

	/* Badge */
	.version-badge {
		display: inline-flex;
		align-items: center;
		padding: 2px 8px;
		font-size: 11px;
		font-weight: 500;
		border-radius: 10px;
		background: var(--bg-tertiary);
		color: var(--text-muted);
	}

	.badge-system {
		background: rgba(148, 163, 184, 0.15);
	}

	/* LED indicator */
	.led {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		flex-shrink: 0;
		transition: background 0.3s ease, box-shadow 0.3s ease;
	}

	.led-green {
		background: var(--success, #10b981);
		box-shadow: 0 0 6px var(--success, #10b981);
	}

	.led-yellow {
		background: var(--warning, #f59e0b);
		box-shadow: 0 0 6px var(--warning, #f59e0b);
	}

	.led-gray {
		background: var(--text-muted, #6b7280);
		box-shadow: none;
	}

	/* Eye toggle */
	.eye-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 2px;
		border: none;
		background: none;
		color: var(--text-muted);
		cursor: pointer;
		border-radius: 4px;
		flex-shrink: 0;
		transition: color 0.15s;
	}

	.eye-btn:hover {
		color: var(--text-secondary);
	}

	/* Details */
	.details {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.card.view-compact .details {
		gap: 10px;
	}

	.detail-label {
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-muted);
	}

	.detail-value {
		font-size: 13px;
		font-family: var(--font-mono, monospace);
		color: var(--text-secondary);
	}

	.divider {
		border: none;
		border-top: 1px dashed var(--color-border);
		margin: 4px 0;
	}

	.stats-row {
		white-space: nowrap;
	}

	/* Actions */
	.actions-wrapper {
		padding-top: 12px;
		border-top: 1px solid var(--border);
	}

	/* Connectivity gear */
	.connectivity-gear {
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 2px;
		background: none;
		border: none;
		color: var(--text-muted);
		cursor: pointer;
		border-radius: 4px;
		transition: color 0.15s;
	}

	.connectivity-gear:hover {
		color: var(--accent);
	}

	.connectivity-gear.gear-disabled {
		opacity: 0.4;
	}

	/* Traffic chart (collapsible) */
	.chart-section {
		margin: 0 -1rem -1rem;
		border-radius: 0 0 var(--radius) var(--radius);
		background: var(--bg-secondary, rgba(0,0,0,0.15));
		overflow: hidden;
	}

	.card.view-compact .chart-section {
		margin: 0 -14px -12px;
	}

	.chart-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		width: 100%;
		padding: 6px 12px;
		border: none;
		background: none;
		cursor: pointer;
		user-select: none;
		transition: background 0.15s;
	}

	.chart-header:hover {
		background: rgba(255,255,255,0.03);
	}

	.chart-label {
		font-size: 0.6875rem;
		font-weight: 500;
		color: var(--text-muted);
		text-transform: uppercase;
		letter-spacing: 0.03em;
	}

	.chart-chevron {
		font-size: 0.875rem;
		color: var(--text-muted);
		transition: transform 0.2s ease;
		transform: rotate(-90deg);
	}

	.chart-chevron.expanded {
		transform: rotate(0deg);
	}

	.chart-body {
		max-height: 0;
		overflow: hidden;
		transition: max-height 0.2s ease;
		padding: 0 12px;
	}

	.chart-body.expanded {
		max-height: 300px;
		padding: 0 12px 4px;
	}

	.actions-row {
		display: flex;
		gap: 4px;
		align-items: center;
		flex-wrap: nowrap;
		justify-content: center;
	}

	.card.view-compact .actions-row {
		justify-content: flex-end;
	}

	.system-action-test,
	.system-action-primary {
		display: inline-flex;
	}

	.system-action-test :global(.btn:hover:not(:disabled):not(.is-disabled)) {
		color: var(--color-success);
		background: var(--color-success-tint);
	}

	.system-action-primary :global(.btn:hover:not(:disabled):not(.is-disabled)) {
		color: var(--color-accent);
		background: var(--color-accent-tint);
	}

	.list-actions-row .system-action-test,
	.list-actions-row .system-action-primary {
		display: flex;
		align-self: stretch;
	}

	.list-actions-row .system-action-test :global(.btn),
	.list-actions-row .system-action-primary :global(.btn) {
		width: 100%;
	}

	@media (max-width: 1080px) {
		.list-card {
			grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		}

		.list-cell-actions {
			grid-column: 1 / -1;
		}

		.list-actions-row {
			flex-direction: row;
			flex-wrap: wrap;
			justify-content: flex-end;
		}

	}

	@media (max-width: 720px) {
		.list-card {
			grid-template-columns: minmax(0, 1fr);
		}
	}
</style>
