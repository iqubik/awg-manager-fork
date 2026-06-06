<script lang="ts">
	import { untrack } from 'svelte';
	import type { TunnelListItem } from '$lib/types';
	import { Toggle, TrafficChart, TrafficSparkline, VersionBadge, Badge } from '$lib/components/ui';
	import { tunnels } from '$lib/stores/tunnels';
	import { api } from '$lib/api/client';
	import {
		formatRelativeTime,
		formatDuration,
		secondsSince,
		formatBytes,
		formatBitRate,
	} from '$lib/utils/format';
	import { getTrafficRates, subscribeTraffic, loadHistory } from '$lib/stores/traffic';
	import {
		awgConnectivityDown,
		awgListShowsPingButton,
		awgPingStatusNote,
		awgShowConnectivityRow,
	} from '$lib/utils/awgPingStatus';
	import ConnectivitySettingsModal from './ConnectivitySettingsModal.svelte';
	import TunnelDiagnosticsModal from '$lib/components/testing/TunnelDiagnosticsModal.svelte';
	import TunnelTestIcon from './TunnelTestIcon.svelte';
	import { PingButton } from '$lib/components/ui';

	interface Props {
		tunnel: TunnelListItem;
		view?: 'cards' | 'compact' | 'list';
		toggleLoading?: boolean;
		deleteLoading?: boolean;
		onToggleOnOff?: () => void;
		ondelete?: () => void;
		ondetail?: (id: string) => void;
		autoConnectivityNonce?: number;
		autoConnectivityDelayMs?: number;
	}

	let {
		tunnel,
		view = 'cards',
		toggleLoading = false,
		deleteLoading = false,
		onToggleOnOff,
		ondelete,
		ondetail,
		autoConnectivityNonce = 0,
		autoConnectivityDelayMs = 0,
	}: Props = $props();

	// ─── Toggle / status logic ─────────────────────────────────────
	let isOn = $derived(['running', 'starting', 'broken'].includes(tunnel.status));
	let toggleDisabled = $derived(toggleLoading || tunnel.hasAddressConflict === true);

	let statusHint = $derived.by(() => {
		switch (tunnel.status) {
			case 'starting': return 'Запуск...';
			case 'needs_start': return 'Ожидает запуска';
			case 'needs_stop': return 'Остановка...';
			case 'broken': return 'Сломан';
			case 'running':
				if (tunnel.pingCheck.status === 'recovering') {
					const n = tunnel.pingCheck.restartCount;
					return n > 0 ? `Восстановление (${n})` : 'Проверка связи...';
				}
				return '';
			default: return '';
		}
	});

	// ─── Connectivity (SSE-driven + manual recheck) ────────────────
	let connectivitySettingsOpen = $state(false);
	let diagnosticsOpen = $state(false);

	let checkMethod = $derived(tunnel.connectivityCheck?.method || 'http');
	let isCheckDisabled = $derived(checkMethod === 'disabled');

	const connMap = tunnels.connectivityMap;
	let connData = $derived(($connMap).get(tunnel.id));
	let isActive = $derived(tunnel.status === 'running' || tunnel.status === 'broken');

	type ConnectivityState = 'idle' | 'connected' | 'disconnected' | 'checking';
	let connectivity = $derived.by<ConnectivityState>(() => {
		if (!isActive || isCheckDisabled) return 'idle';
		if (!connData) return 'checking';
		return connData.connected ? 'connected' : 'disconnected';
	});
	let latencyMs = $derived(connData?.latency ?? null);
	let connectivityDown = $derived(awgConnectivityDown(tunnel, connData));

	let ledColor = $derived.by(() => {
		switch (tunnel.status) {
			case 'running':
				if (tunnel.pingCheck.status === 'recovering') return 'orange';
				if (connectivityDown) return 'red';
				return 'green';
			case 'starting':
			case 'needs_start':
			case 'needs_stop': return 'yellow';
			case 'broken': return 'orange';
			default: return 'gray';
		}
	});

	let ledPulse = $derived(
		['starting', 'needs_start', 'needs_stop', 'broken'].includes(tunnel.status) ||
		(tunnel.status === 'running' && tunnel.pingCheck.status === 'recovering')
	);

	let manualChecking = $state(false);
	async function checkConnectivityManual(): Promise<void> {
		if (manualChecking) return;
		manualChecking = true;
		try {
			const result = await api.checkConnectivity(tunnel.id);
			tunnels.updateConnectivity(tunnel.id, result.connected, result.latency ?? null);
		} catch {
			tunnels.updateConnectivity(tunnel.id, false, null);
		} finally {
			manualChecking = false;
		}
	}

	let lastAutoConnectivityNonce = 0;
	$effect(() => {
		const nonce = autoConnectivityNonce;
		const delay = autoConnectivityDelayMs;
		if (nonce <= 0 || nonce === lastAutoConnectivityNonce) return;
		lastAutoConnectivityNonce = nonce;
		if (!isActive || isCheckDisabled) return;

		const timer = setTimeout(() => {
			untrack(() => void checkConnectivityManual());
		}, delay);
		return () => clearTimeout(timer);
	});

	// ─── Server / address parsing ───────────────────────────────────
	let showEndpoint = $state(false);

	let serverHost = $derived.by(() => {
		const endpoint = tunnel.endpoint ?? '';
		const match = endpoint.match(/^(?:\[([^\]]+)\]|([^:]+)):(\d+)$/);
		if (match) return match[1] || match[2] || endpoint;
		return endpoint;
	});

	let serverPort = $derived.by(() => {
		const endpoint = tunnel.endpoint ?? '';
		const match = endpoint.match(/:(\d+)$/);
		return match ? match[1] : '';
	});

	let addresses = $derived.by(() => {
		const addr = tunnel.address ?? '';
		const parts = addr.split(',').map((s) => s.trim());
		const ipv4 = parts.find((p) => !p.includes(':')) || '';
		const ipv6 = parts.find((p) => p.includes(':')) || '';
		return { ipv4, ipv6 };
	});

	let ipv6Display = $derived.by(() => {
		const full = addresses.ipv6;
		if (!full || full.length <= 20) return full;
		return full.slice(0, 12) + '...' + full.slice(-8);
	});

	let connectionDisplay = $derived.by(() => {
		const iface = tunnel.resolvedIspInterface || tunnel.ispInterface || '';
		const label = tunnel.resolvedIspInterfaceLabel || tunnel.ispInterfaceLabel || '';
		if (!iface) return '';
		return label ? `${label} (${iface})` : iface;
	});

	let listStatusText = $derived.by(() => {
		if (tunnel.hasAddressConflict) return 'Конфликт IP';
		switch (tunnel.status) {
			case 'running':
				if (manualChecking || connectivity === 'checking') return 'Проверка';
				if (!isCheckDisabled && connectivity === 'disconnected') return 'Нет связи';
				return 'Активен';
			case 'starting':
				return 'Запуск';
			case 'needs_start':
				return 'Готов';
			case 'needs_stop':
				return 'Остановка';
			case 'broken':
				return 'Ошибка';
			case 'disabled':
				return 'Выключен';
			default:
				return 'Остановлен';
		}
	});

	// ─── Traffic chart ─────────────────────────────────────────────
	let rxRates = $state<number[]>([]);
	let txRates = $state<number[]>([]);

	let tunnelId = $derived(tunnel.id);
	let chartHeight = $derived(view === 'compact' ? 76 : 100);

	let inlineRxRate = $derived(rxRates.length > 0 ? rxRates[rxRates.length - 1] : 0);
	let inlineTxRate = $derived(txRates.length > 0 ? txRates[txRates.length - 1] : 0);

	function compactRelativeTime(value: string | null | undefined): string {
		return value ? formatRelativeTime(value).replace(/\s+назад$/u, '') : '—';
	}

	$effect(() => {
		const id = tunnelId;
		const update = () => {
			const t = getTrafficRates(id);
			rxRates = t.rx;
			txRates = t.tx;
		};
		update();
		return subscribeTraffic(update);
	});

	let initialLoadDone = false;
	$effect(() => {
		const id = tunnelId;
		if (initialLoadDone) return;
		initialLoadDone = true;
		untrack(() => loadHistory(id));
	});

	// ─── Card border class hook (status-tinted) ─────────────────────
	let borderState = $derived.by<
		'running' | 'recovering' | 'unreachable' | 'broken' | 'transitional' | 'disabled' | 'idle'
	>(() => {
		if (tunnel.status === 'running') {
			if (tunnel.pingCheck?.status === 'recovering') return 'recovering';
			if (connectivityDown) return 'unreachable';
			return 'running';
		}
		if (tunnel.status === 'broken') return 'broken';
		if (['starting', 'needs_start', 'needs_stop'].includes(tunnel.status)) return 'transitional';
		if (tunnel.status === 'disabled') return 'disabled';
		return 'idle';
	});

	let isDenseCard = $derived(view === 'cards');
	let isListCard = $derived(view === 'list');
	let pingStatusNote = $derived(
		isListCard ? null : awgPingStatusNote(tunnel, isDenseCard ? 'short' : 'full'),
	);
	let showConnectivityRow = $derived(awgShowConnectivityRow(tunnel.status));
	let toggleStarting = $derived(tunnel.status === 'starting');
	/** Hide header hint when the same state is shown in the ping row (normal / compact cards). */
	let headerStatusHint = $derived(
		!isDenseCard && !isListCard && pingStatusNote ? '' : statusHint,
	);
	/** Ping row: transitional note or live connectivity when check is enabled. */
	let showPingButton = $derived(
		isListCard
			? awgListShowsPingButton(tunnel, connData)
			: pingStatusNote !== null ||
					(!isCheckDisabled &&
						(borderState === 'running' ||
							borderState === 'recovering' ||
							borderState === 'unreachable')),
	);

</script>

{#if view === 'list'}
	<div class="card list-card border-{borderState}">
		<div class="list-cell list-cell-primary">
			<div class="title-line">
				<button
					type="button"
					class="tunnel-name"
					title={tunnel.name}
					onclick={() => ondetail?.(tunnel.id)}
				>
					{tunnel.name}
				</button>
				{#if tunnel.defaultRoute}
					<Badge variant="accent" size="sm">default</Badge>
				{/if}
			</div>
			<div class="meta-line">
				<span class="iface-name">{tunnel.interfaceName || tunnel.id}</span>
				{#if tunnel.backend}
					<VersionBadge kind="backend" value={tunnel.backend} />
				{/if}
				{#if tunnel.awgVersion}
					<VersionBadge kind="awg" value={tunnel.awgVersion} />
				{/if}
			</div>
			<div class="list-note">
				{addresses.ipv4 || '—'}
				{#if connectionDisplay}
					<span class="list-note-sep">·</span>
					{connectionDisplay}
				{/if}
			</div>
		</div>

		<div class="list-cell list-cell-status">
			<span class="list-label">Статус</span>
			<div class="list-status-main">
				<span
					class="led led-{ledColor}"
					class:led-pulse={ledPulse}
				></span>
				<span class="list-status-text">{listStatusText}</span>
		<span
			class:toggle-recovering={borderState === 'recovering'}
			class:toggle-starting={toggleStarting}
			class:toggle-unreachable={borderState === 'unreachable'}
			title={tunnel.hasAddressConflict ? 'Конфликт адресов — другой туннель с таким же IP уже запущен' : undefined}
		>
			<Toggle
				checked={isOn}
				onchange={() => onToggleOnOff?.()}
				loading={toggleLoading}
				disabled={toggleDisabled}
				variant="flip"
				size="sm"
			/>
		</span>
		</div>
		{#if statusHint}
			<div class="list-note list-status-hint" class:recovering={borderState === 'recovering'}>{statusHint}</div>
		{/if}
	{#if showConnectivityRow}
		<div class="connectivity-row" class:recovering={pingStatusNote?.tone === 'recovering'}>
			{#if showPingButton}
				<PingButton
					{connectivity}
					{latencyMs}
					statusNote={pingStatusNote?.text}
					statusNoteTone={pingStatusNote?.tone}
					checking={manualChecking}
					onclick={checkConnectivityManual}
				/>
			{/if}
			<button
				class="connectivity-gear"
				onclick={() => connectivitySettingsOpen = true}
				title="Настройки проверки связности"
			>
				<svg width="14" height="14" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M7.84 1.804A1 1 0 018.82 1h2.36a1 1 0 01.98.804l.331 1.652a6.993 6.993 0 011.929 1.115l1.598-.54a1 1 0 011.186.447l1.18 2.044a1 1 0 01-.205 1.251l-1.267 1.113a7.047 7.047 0 010 2.228l1.267 1.113a1 1 0 01.206 1.25l-1.18 2.045a1 1 0 01-1.187.447l-1.598-.54a6.993 6.993 0 01-1.929 1.115l-.33 1.652a1 1 0 01-.98.804H8.82a1 1 0 01-.98-.804l-.331-1.652a6.993 6.993 0 01-1.929-1.115l-1.598.54a1 1 0 01-1.186-.447l-1.18-2.044a1 1 0 01.205-1.251l1.267-1.114a7.05 7.05 0 010-2.227L1.821 7.773a1 1 0 01-.206-1.25l1.18-2.045a1 1 0 011.187-.447l1.598.54A6.993 6.993 0 017.51 3.456l.33-1.652zM10 13a3 3 0 100-6 3 3 0 000 6z" clip-rule="evenodd" /></svg>
			</button>
		</div>
	{/if}
</div>

<div class="list-cell list-cell-endpoint">
			<span class="list-label">Endpoint</span>
			<div class="kv-endpoint">
				<span class="kv-value truncate" title={showEndpoint ? serverHost : ''}>{showEndpoint ? (serverHost || '—') : '•••••••••'}</span>
				<button
					class="eye-btn"
					onclick={() => showEndpoint = !showEndpoint}
					title={showEndpoint ? 'Скрыть' : 'Показать'}
				>
					{#if showEndpoint}
						<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
					{:else}
						<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
					{/if}
				</button>
				<span class="list-port">:{serverPort || '—'}</span>
			</div>
			<div class="list-note">
				IPv4 {addresses.ipv4 || '—'}
				{#if addresses.ipv6}
					<span class="list-note-sep">·</span>
					IPv6 <span title={addresses.ipv6}>{ipv6Display}</span>
				{/if}
			</div>
		</div>

		<div class="list-cell list-cell-traffic">
			<span class="list-label">Трафик</span>
			{#if tunnel.status === 'running'}
				<div class="list-traffic-chart">
					<TrafficChart
						{rxRates}
						{txRates}
						rxTotal={tunnel.rxBytes ?? 0}
						txTotal={tunnel.txBytes ?? 0}
						height={36}
						onclick={() => ondetail?.(tunnel.id)}
					/>
				</div>
			{:else}
				<div class="list-traffic-empty">Нет данных</div>
			{/if}
			<div class="list-note">↓ {formatBytes(tunnel.rxBytes ?? 0)} · ↑ {formatBytes(tunnel.txBytes ?? 0)}</div>
		</div>

		<div class="list-cell list-cell-stats">
			<span class="list-label">Активность</span>
			<div class="list-stat-row">
				<span>Handshake</span>
				<strong>{tunnel.lastHandshake ? formatRelativeTime(tunnel.lastHandshake) : '—'}</strong>
			</div>
			<div class="list-stat-row">
				<span>Uptime</span>
				<strong>{tunnel.startedAt ? formatDuration(secondsSince(tunnel.startedAt)) : '—'}</strong>
			</div>
		</div>

		<div class="list-cell list-cell-actions">
			<div class="actions list-actions">
				<a class="action-btn" href="/tunnels/{tunnel.id}">
					<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
					Изменить
				</a>
			<button
				class="action-btn action-test"
				type="button"
				onclick={() => (diagnosticsOpen = true)}
			>
				<TunnelTestIcon />
				Тест
			</button>
				<button
					class="action-btn action-danger"
					disabled={deleteLoading}
					onclick={() => ondelete?.()}
					title="Удалить туннель"
				>
					{#if deleteLoading}
						<span class="action-spinner"></span>
					{:else}
						<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3,6 5,6 21,6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
					{/if}
					Удалить
				</button>
			</div>
		</div>
	</div>
{:else}
	<div
		class="card border-{borderState}"
		class:view-compact={view === 'compact'}
		class:view-dense={view === 'cards'}
	>
		<!-- Header -->
		<div class="header" class:header-dense={view === 'cards'}>
			{#if view === 'cards'}
			<div class="header-dense-body">
				<div class="tunnel-name-row">
					<span
						class="led title-led led-{ledColor}"
						class:led-pulse={ledPulse}
						title={statusHint || listStatusText}
					></span>
					<button
						type="button"
						class="tunnel-name tunnel-name-dense"
						title={tunnel.name}
						onclick={() => ondetail?.(tunnel.id)}
					>
						{tunnel.name}
					</button>
					{#if tunnel.defaultRoute}
						<Badge variant="accent" size="sm">default</Badge>
					{/if}
				</div>
				<div class="meta-tags-dense">
					<span class="iface-plain-dense" title={tunnel.interfaceName || tunnel.id}>
						{tunnel.interfaceName || tunnel.id}
					</span>
					{#if tunnel.awgVersion}
						<VersionBadge kind="awg" value={tunnel.awgVersion} />
					{/if}
					{#if tunnel.backend}
						<VersionBadge kind="backend" value={tunnel.backend} />
					{/if}
				</div>
			</div>
			<div class="dense-toolbar" title={statusHint || undefined}>
				<!-- row 1: toggle -->
				<div class="dense-toolbar-top">
				<span
					class:toggle-recovering={borderState === 'recovering'}
			class:toggle-starting={toggleStarting}
			class:toggle-unreachable={borderState === 'unreachable'}
					title={tunnel.hasAddressConflict ? 'Конфликт адресов — другой туннель с таким же IP уже запущен' : undefined}
				>
					<Toggle
						checked={isOn}
						onchange={() => onToggleOnOff?.()}
						loading={toggleLoading}
						disabled={toggleDisabled}
						size="sm"
						variant="flip"
					/>
				</span>
				</div>
			<!-- row 2: ping + gear (only when running) -->
			{#if showConnectivityRow}
		<div class="dense-toolbar-bottom" class:recovering={pingStatusNote?.tone === 'recovering'}>
					{#if showPingButton}
						<PingButton
							{connectivity}
							{latencyMs}
							statusNote={pingStatusNote?.text}
							statusNoteTone={pingStatusNote?.tone}
							checking={manualChecking}
							size="sm"
							onclick={checkConnectivityManual}
						/>
					{/if}
					<button
						class="connectivity-gear"
						onclick={() => connectivitySettingsOpen = true}
						title="Настройки проверки связности"
					>
						<svg width="11" height="11" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M7.84 1.804A1 1 0 018.82 1h2.36a1 1 0 01.98.804l.331 1.652a6.993 6.993 0 011.929 1.115l1.598-.54a1 1 0 011.186.447l1.18 2.044a1 1 0 01-.205 1.251l-1.267 1.113a7.047 7.047 0 010 2.228l1.267 1.113a1 1 0 01.206 1.25l-1.18 2.045a1 1 0 01-1.187.447l-1.598-.54a6.993 6.993 0 01-1.929 1.115l-.33 1.652a1 1 0 01-.98.804H8.82a1 1 0 01-.98-.804l-.331-1.652a6.993 6.993 0 01-1.929-1.115l-1.598.54a1 1 0 01-1.186-.447l-1.18-2.044a1 1 0 01.205-1.251l1.267-1.114a7.05 7.05 0 010-2.227L1.821 7.773a1 1 0 01-.206-1.25l1.18-2.045a1 1 0 011.187-.447l1.598.54A6.993 6.993 0 017.51 3.456l.33-1.652zM10 13a3 3 0 100-6 3 3 0 000 6z" clip-rule="evenodd" /></svg>
					</button>
				</div>
			{/if}
			</div>
			{:else}
				<div class="head-left">
					<div class="title-line">
						<span
							class="led title-led led-{ledColor}"
							class:led-pulse={ledPulse}
							title={statusHint || listStatusText}
						></span>
						<button
							type="button"
							class="tunnel-name"
							title={tunnel.name}
							onclick={() => ondetail?.(tunnel.id)}
						>
							{tunnel.name}
						</button>
						{#if tunnel.defaultRoute}
							<Badge variant="accent" size="sm">default</Badge>
						{/if}
					</div>
					<div class="meta-line">
						<span class="iface-name">{tunnel.interfaceName || tunnel.id}</span>
						{#if tunnel.backend}
							<VersionBadge kind="backend" value={tunnel.backend} />
						{/if}
						{#if tunnel.awgVersion}
							<VersionBadge kind="awg" value={tunnel.awgVersion} />
						{/if}
					</div>
					{#if view === 'compact' && headerStatusHint}
						<span class="status-hint status-hint-left">{headerStatusHint}</span>
					{/if}
				</div>

				<div class="head-right">
					<div class="led-toggle">
					<span
						class:toggle-recovering={borderState === 'recovering'}
						class:toggle-starting={toggleStarting}
						class:toggle-unreachable={borderState === 'unreachable'}
						title={tunnel.hasAddressConflict ? 'Конфликт адресов — другой туннель с таким же IP уже запущен' : undefined}
					>
						<Toggle
							checked={isOn}
							onchange={() => onToggleOnOff?.()}
							loading={toggleLoading}
							disabled={toggleDisabled}
							variant="flip"
						/>
					</span>
					</div>
				{#if view !== 'compact' && headerStatusHint}
					<span class="status-hint">{headerStatusHint}</span>
				{/if}
			{#if showConnectivityRow}
				<div class="connectivity-row" class:recovering={pingStatusNote?.tone === 'recovering'}>
					{#if showPingButton}
						<PingButton
							{connectivity}
							{latencyMs}
							statusNote={pingStatusNote?.text}
							statusNoteTone={pingStatusNote?.tone}
							checking={manualChecking}
							onclick={checkConnectivityManual}
						/>
					{/if}
					<button
						class="connectivity-gear"
						onclick={() => connectivitySettingsOpen = true}
						title="Настройки проверки связности"
					>
						<svg width="14" height="14" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M7.84 1.804A1 1 0 018.82 1h2.36a1 1 0 01.98.804l.331 1.652a6.993 6.993 0 011.929 1.115l1.598-.54a1 1 0 011.186.447l1.18 2.044a1 1 0 01-.205 1.251l-1.267 1.113a7.047 7.047 0 010 2.228l1.267 1.113a1 1 0 01.206 1.25l-1.18 2.045a1 1 0 01-1.187.447l-1.598-.54a6.993 6.993 0 01-1.929 1.115l-.33 1.652a1 1 0 01-.98.804H8.82a1 1 0 01-.98-.804l-.331-1.652a6.993 6.993 0 01-1.929-1.115l-1.598.54a1 1 0 01-1.186-.447l-1.18-2.044a1 1 0 01.205-1.251l1.267-1.114a7.05 7.05 0 010-2.227L1.821 7.773a1 1 0 01-.206-1.25l1.18-2.045a1 1 0 011.187-.447l1.598.54A6.993 6.993 0 017.51 3.456l.33-1.652zM10 13a3 3 0 100-6 3 3 0 000 6z" clip-rule="evenodd" /></svg>
					</button>
				</div>
			{/if}
		</div>
	{/if}
</div>

		<!-- Details -->
		<div class="details">
			{#if view === 'cards'}
				<div class="details-dense-cols">
					<div class="details-dense-col">
						<div class="kv-stacked-stat">
							<span class="kv-stacked-label">Сервер</span>
							<span class="kv-endpoint">
								<span
									class="kv-stacked-value truncate"
									title={showEndpoint ? serverHost : ''}
								>
									{showEndpoint ? (serverHost || '—') : '•••••••••'}
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
						{#if connectionDisplay}
							<div class="kv-stacked-stat">
								<span class="kv-stacked-label">Подключение</span>
								<span class="kv-stacked-value" title={connectionDisplay}>{connectionDisplay}</span>
							</div>
						{/if}
						<div class="kv-stacked-stat">
							<span class="kv-stacked-label">IPv4</span>
							<span class="kv-stacked-value">{addresses.ipv4 || '—'}</span>
						</div>
						{#if addresses.ipv6}
							<div class="kv-stacked-stat">
								<span class="kv-stacked-label">IPv6</span>
								<span class="kv-stacked-value cursor-help" title={addresses.ipv6}>{ipv6Display}</span>
							</div>
						{/if}
					</div>
					<div class="details-dense-col details-dense-col-right">
						<div class="kv-stacked-stat">
							<span class="kv-stacked-label">Порт</span>
							<span class="kv-stacked-value">{serverPort || '—'}</span>
						</div>
						{#if tunnel.status === 'running'}
							<div class="kv-stacked-stat">
								<span class="kv-stacked-label">Uptime</span>
								<span class="kv-stacked-value">
									{tunnel.startedAt ? formatDuration(secondsSince(tunnel.startedAt)) : '—'}
								</span>
							</div>
							<div class="kv-stacked-stat">
								<span class="kv-stacked-label">Handshake</span>
								<span class="kv-stacked-value" title={tunnel.lastHandshake || ''}>
									{compactRelativeTime(tunnel.lastHandshake)}
								</span>
							</div>
						{/if}
					</div>
				</div>
			{:else}
			<div class="kv-row">
				<div class="kv kv-grow">
					<span class="kv-label">Сервер</span>
					<span class="kv-endpoint">
						<span class="kv-value truncate" title={showEndpoint ? serverHost : ''}>{showEndpoint ? (serverHost || '—') : '•••••••••'}</span>
						<button
							class="eye-btn"
							onclick={() => showEndpoint = !showEndpoint}
							title={showEndpoint ? 'Скрыть' : 'Показать'}
						>
							{#if showEndpoint}
								<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
							{:else}
								<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
							{/if}
						</button>
					</span>
				</div>
				<div class="kv kv-shrink">
					<span class="kv-label">Порт</span>
					<span class="kv-value">{serverPort || '—'}</span>
				</div>
			</div>

			{#if tunnel.resolvedIspInterface || tunnel.ispInterface}
				{@const iface = tunnel.resolvedIspInterface || tunnel.ispInterface}
				{@const label = tunnel.resolvedIspInterfaceLabel || tunnel.ispInterfaceLabel || ''}
				<div class="kv-row">
					<div class="kv">
						<span class="kv-label">Подключение</span>
						<span class="kv-value">
							{#if label}
								{label} <span class="kv-secondary">({iface})</span>
							{:else}
								{iface}
							{/if}
						</span>
					</div>
				</div>
			{/if}

			<div class="kv-row">
				<div class="kv">
					<span class="kv-label">IPv4</span>
					<span class="kv-value">{addresses.ipv4 || '—'}</span>
				</div>
				{#if addresses.ipv6}
					<div class="kv">
						<span class="kv-label">IPv6</span>
						<span class="kv-value cursor-help" title={addresses.ipv6}>{ipv6Display}</span>
					</div>
				{/if}
			</div>

			{#if tunnel.status === 'running'}
				<hr class="divider" />
				<div class="kv-row stats-row">
					<div class="kv kv-grow">
						<span class="kv-label">Uptime</span>
						<span class="kv-value">
							{tunnel.startedAt ? formatDuration(secondsSince(tunnel.startedAt)) : '—'}
						</span>
					</div>
					<div class="kv kv-grow kv-right">
						<span class="kv-label">Handshake</span>
						<span class="kv-value" title={tunnel.lastHandshake || ''}>
							{tunnel.lastHandshake ? formatRelativeTime(tunnel.lastHandshake) : '—'}
						</span>
					</div>
				</div>
			{/if}
			{/if}
		</div>

		<!-- Actions -->
		<div class="actions">
			<a class="action-btn" href="/tunnels/{tunnel.id}">
				<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
				Изменить
			</a>
			<button
				class="action-btn action-test"
				type="button"
				onclick={() => (diagnosticsOpen = true)}
			>
				<TunnelTestIcon />
				Тест
			</button>
			<button
				class="action-btn action-danger"
				disabled={deleteLoading}
				onclick={() => ondelete?.()}
				title="Удалить туннель"
			>
				{#if deleteLoading}
					<span class="action-spinner"></span>
				{:else}
					<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3,6 5,6 21,6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
				{/if}
				Удалить
			</button>
		</div>

		<!-- Traffic (running only) -->
		{#if tunnel.status === 'running'}
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
						responsive
						height={22}
					/>
					<div class="traffic-inline-rates">
						<span class="traffic-inline-rate rx">↓ {formatBitRate(inlineRxRate)}</span>
						<span class="traffic-inline-rate tx">↑ {formatBitRate(inlineTxRate)}</span>
					</div>
				</button>
			{:else}
				<div class="chart-section">
					<div class="chart-body">
						<TrafficChart
							{rxRates}
							{txRates}
							rxTotal={tunnel.rxBytes ?? 0}
							txTotal={tunnel.txBytes ?? 0}
							height={chartHeight}
							onclick={() => ondetail?.(tunnel.id)}
						/>
					</div>
				</div>
			{/if}
		{/if}
	</div>
{/if}

<TunnelDiagnosticsModal
	open={diagnosticsOpen}
	kind="awg"
	targetId={tunnel.id}
	displayName={tunnel.name}
	subjectLabel="туннель"
	onclose={() => (diagnosticsOpen = false)}
/>

<ConnectivitySettingsModal
	bind:open={connectivitySettingsOpen}
	tunnelId={tunnel.id}
	tunnelAddress={tunnel.address}
	onclose={() => connectivitySettingsOpen = false}
	onSaved={() => connectivitySettingsOpen = false}
/>

<style>
	.card {
		display: flex;
		flex-direction: column;
		gap: 12px;
		padding: 14px 16px;
		background: var(--color-bg-secondary);
		border: 1px solid var(--color-border);
		border-radius: var(--radius);
		transition: border-color var(--t-fast) ease;
	}

	.card.border-running { border-color: var(--color-success-border); }
	.card.border-recovering { border-color: var(--color-broken-border); }
	.card.border-unreachable { border-color: var(--color-error-border); }
	.card.border-broken { border-color: var(--color-broken-border); }
	.card.border-transitional { border-color: var(--color-warning-border); }
	.card.border-disabled { border-color: var(--color-text-muted); }

	/* Toggle tint when recovering (orange) or starting (yellow) */
	.toggle-recovering :global(.toggle-container.flip input:checked + .flip-track),
	.toggle-recovering :global(.toggle-container.sm.flip input:checked + .flip-track) {
		background: color-mix(in srgb, var(--color-broken) 18%, var(--color-bg-tertiary));
		box-shadow:
			inset 2px 0 4px rgba(0, 0, 0, 0.18),
			0 0 6px color-mix(in srgb, var(--color-broken) 35%, transparent);
		transition: background 0.4s ease, box-shadow 0.4s ease;
	}

	.toggle-recovering :global(.toggle-container.flip input:checked + .flip-track .flip-lever),
	.toggle-recovering :global(.toggle-container.sm.flip input:checked + .flip-track .flip-lever) {
		background: linear-gradient(
			to bottom,
			color-mix(in srgb, var(--color-broken) 75%, white),
			var(--color-broken)
		);
		box-shadow:
			0 1px 3px rgba(0, 0, 0, 0.3),
			0 0 5px color-mix(in srgb, var(--color-broken) 45%, transparent);
		transition: background 0.4s ease, box-shadow 0.4s ease, transform 0.2s ease;
	}

	.toggle-starting :global(.toggle-container.flip input:checked + .flip-track),
	.toggle-starting :global(.toggle-container.sm.flip input:checked + .flip-track) {
		background: color-mix(in srgb, var(--color-warning) 18%, var(--color-bg-tertiary));
		box-shadow:
			inset 2px 0 4px rgba(0, 0, 0, 0.18),
			0 0 6px color-mix(in srgb, var(--color-warning) 35%, transparent);
		transition: background 0.4s ease, box-shadow 0.4s ease;
	}

	.toggle-starting :global(.toggle-container.flip input:checked + .flip-track .flip-lever),
	.toggle-starting :global(.toggle-container.sm.flip input:checked + .flip-track .flip-lever) {
		background: linear-gradient(
			to bottom,
			color-mix(in srgb, var(--color-warning) 75%, white),
			var(--color-warning)
		);
		box-shadow:
			0 1px 3px rgba(0, 0, 0, 0.3),
			0 0 5px color-mix(in srgb, var(--color-warning) 45%, transparent);
		transition: background 0.4s ease, box-shadow 0.4s ease, transform 0.2s ease;
	}

	.toggle-unreachable :global(.toggle-container.flip input:checked + .flip-track),
	.toggle-unreachable :global(.toggle-container.sm.flip input:checked + .flip-track) {
		background: color-mix(in srgb, var(--color-error) 18%, var(--color-bg-tertiary));
		box-shadow:
			inset 2px 0 4px rgba(0, 0, 0, 0.18),
			0 0 6px color-mix(in srgb, var(--color-error) 35%, transparent);
		transition: background 0.4s ease, box-shadow 0.4s ease;
	}

	.toggle-unreachable :global(.toggle-container.flip input:checked + .flip-track .flip-lever),
	.toggle-unreachable :global(.toggle-container.sm.flip input:checked + .flip-track .flip-lever) {
		background: linear-gradient(
			to bottom,
			color-mix(in srgb, var(--color-error) 75%, white),
			var(--color-error)
		);
		box-shadow:
			0 1px 3px rgba(0, 0, 0, 0.3),
			0 0 5px color-mix(in srgb, var(--color-error) 45%, transparent);
		transition: background 0.4s ease, box-shadow 0.4s ease, transform 0.2s ease;
	}

	.list-card {
		display: grid;
		grid-template-columns: minmax(220px, 1.35fr) minmax(170px, 0.95fr) minmax(220px, 1.2fr) minmax(180px, 1fr) minmax(150px, 0.9fr) auto;
		gap: 14px;
		align-items: center;
		padding: 12px 14px;
	}

	.list-cell {
		min-width: 0;
	}

	.list-cell-primary,
	.list-cell-status,
	.list-cell-endpoint,
	.list-cell-traffic,
	.list-cell-stats {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.list-label {
		font-size: 10px;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--color-text-muted);
	}

	.list-note {
		font-size: 11px;
		color: var(--color-text-muted);
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
		color: var(--color-text-primary);
		transition: color 0.4s ease;
	}

	.card.border-recovering .list-status-text {
		color: var(--color-broken);
	}

	.card.border-unreachable .list-status-text {
		color: var(--color-error);
	}

	.list-status-hint.recovering {
		color: var(--color-broken);
		transition: color 0.4s ease;
	}

	.list-port {
		font-size: 12px;
		font-family: var(--font-mono);
		color: var(--color-text-muted);
		flex-shrink: 0;
	}

	.list-traffic-chart {
		min-height: 36px;
		padding: 2px 0;
	}

	.list-traffic-empty {
		font-size: 12px;
		color: var(--color-text-muted);
		padding: 8px 0;
	}

	.list-stat-row {
		display: flex;
		justify-content: space-between;
		gap: 10px;
		font-size: 11px;
		color: var(--color-text-muted);
	}

	.list-stat-row strong {
		font-size: 12px;
		font-weight: 600;
		color: var(--color-text-secondary);
		white-space: nowrap;
	}

	.list-actions {
		flex-direction: column;
		align-items: stretch;
	}

	.card.view-compact {
		gap: 10px;
		padding: 12px 14px;
	}

	.card.view-dense {
		gap: 8px;
		padding: 10px 12px;
	}

	.card.view-dense .head-left {
		gap: 3px;
	}

	.card.view-dense .details {
		gap: 6px;
		padding: 4px 0;
	}

	.title-line-dense {
		display: flex;
		align-items: baseline;
		gap: 6px;
		min-width: 0;
	}

	.tunnel-name-row {
		display: flex;
		align-items: center;
		gap: 5px;
		min-width: 0;
		overflow: hidden;
	}

	.tunnel-name-row :global(.badge) {
		flex-shrink: 0;
		font-size: 9px;
		padding: 1px 6px;
		border-radius: var(--radius-sm);
	}

	.tunnel-name-dense {
		flex: 0 1 auto;
		min-width: 0;
		font-size: 13px;
		line-height: var(--sbx-card-title-line-height);
		font-weight: 600;
	}

	.tunnel-protocol {
		flex-shrink: 0;
		font-size: 10px;
		font-weight: 500;
		font-family: var(--font-mono);
		color: var(--color-text-muted);
		white-space: nowrap;
		letter-spacing: 0.02em;
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

	.meta-tags-dense {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		margin-top: 3px;
		gap: 3px;
		min-width: 0;
		overflow: hidden;
	}

	.card.view-dense .meta-tags-dense :global(.badge),
	.card.view-dense .meta-tags-dense :global(.vb) {
		font-size: 9px;
		padding: 1px 5px;
		line-height: 1.3;
		flex-shrink: 0;
	}

	.iface-plain-dense {
		font-size: 9px;
		font-weight: 500;
		font-family: var(--font-mono);
		color: var(--color-text-muted);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		flex-shrink: 1;
		min-width: 0;
	}

	/* uniform border-radius for all badges in second row */
	.card.view-dense .meta-tags-dense :global(.vb) {
		border-radius: var(--radius-sm);
	}

	.card.view-dense .dense-toolbar-bottom .connectivity-gear {
		width: 16px;
		height: 16px;
		padding: 0;
	}

	.details-dense-cols {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 5.5rem;
		gap: 10px 12px;
		align-items: start;
	}

	.details-dense-col {
		display: flex;
		flex-direction: column;
		gap: 6px;
		min-width: 0;
	}

	.details-dense-col-right {
		width: 100%;
		overflow: hidden;
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
		color: var(--color-text-muted);
		line-height: 1.2;
	}

	.kv-stacked-value {
		font-size: 10px;
		font-family: var(--font-mono);
		color: var(--color-text-secondary);
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
		transition: background var(--t-fast) ease, border-color var(--t-fast) ease;
	}

	.traffic-inline :global(svg.responsive) {
		flex: 1 1 auto;
		width: 100%;
		min-width: 0;
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
		font-family: var(--font-mono);
		font-variant-numeric: tabular-nums;
	}

	.traffic-inline-rate.rx {
		color: var(--color-accent);
	}

	.traffic-inline-rate.tx {
		color: var(--color-success);
	}

	.card.view-dense .actions,
	.card.view-compact .actions {
		gap: 2px;
		justify-content: center;
	}

	.card.view-dense .action-btn {
		padding: 3px 6px;
		font-size: 10px;
		gap: 3px;
	}

	.card.view-dense .action-btn svg {
		width: 12px;
		height: 12px;
	}

	.card.view-list {
		display: grid;
		grid-template-columns: minmax(0, 1.35fr) minmax(280px, 1fr) auto;
		gap: 12px 16px;
		align-items: start;
		padding: 12px 14px;
	}

	/* Header */
	.header {
		display: flex;
		justify-content: space-between;
		align-items: flex-start;
		gap: 10px;
	}

	.card.view-dense .header.header-dense {
		gap: 4px;
	}

	.head-left {
		display: flex;
		flex-direction: column;
		gap: 4px;
		min-width: 0;
	}

	.title-led {
		flex: 0 0 auto;
		margin-top: 1px;
	}

	.card.view-dense .title-led {
		width: 6px;
		height: 6px;
		margin-top: 0;
	}

	.title-line {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
	}

	.tunnel-name {
		background: none;
		border: none;
		padding: 0;
		font: inherit;
		font-size: 15px;
		line-height: var(--sbx-card-title-line-height);
		font-weight: 600;
		color: var(--color-text-primary);
		text-align: left;
		cursor: pointer;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		min-width: 0;
	}

	.tunnel-name:hover {
		color: var(--color-accent);
	}

	.card.view-compact .tunnel-name {
		font-size: 14px;
	}

	.meta-line {
		display: flex;
		align-items: center;
		gap: 6px;
		flex-wrap: wrap;
	}

	.iface-name {
		font-size: 11px;
		font-family: var(--font-mono);
		color: var(--color-text-muted);
	}

	.head-right {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 4px;
		flex-shrink: 0;
	}

	.led-toggle {
		display: flex;
		align-items: center;
		gap: 8px;
	}

	.led {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		flex-shrink: 0;
		transition: background var(--t-med) ease, box-shadow var(--t-med) ease;
	}

	.led-green {
		background: var(--color-success);
		box-shadow: 0 0 6px var(--color-success);
	}

	.led-yellow {
		background: var(--color-warning);
		box-shadow: 0 0 6px var(--color-warning);
	}

	.led-orange {
		background: var(--color-broken);
		box-shadow: 0 0 6px var(--color-broken);
	}

	.led-red {
		background: var(--color-error);
		box-shadow: 0 0 6px var(--color-error);
	}

	.led-gray {
		background: var(--color-text-muted);
		box-shadow: none;
	}

	.led-pulse {
		animation: led-blink 1.5s ease-in-out infinite;
	}

	@keyframes led-blink {
		0%, 100% { opacity: 1; }
		50% { opacity: 0.3; }
	}

	.status-hint {
		font-size: 11px;
		color: var(--color-text-muted);
		transition: color 0.4s ease;
	}

	.card.border-recovering .status-hint {
		color: var(--color-broken);
	}

	.card.border-unreachable .status-hint {
		color: var(--color-error);
	}

	.status-hint-left {
		align-self: flex-start;
		font-size: 11px;
		color: var(--color-broken);
		transition: color 0.4s ease;
	}

	.connectivity-row {
		display: flex;
		align-items: center;
		gap: 5px;
	}

	.connectivity-gear {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		border: none;
		cursor: pointer;
		padding: 2px;
		background: none;
		color: var(--color-text-muted);
		border-radius: var(--radius-sm);
		transition: color var(--t-fast) ease;
	}

	.connectivity-gear:hover {
		color: var(--color-accent);
	}

	/* Details */
	.details {
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 8px 0;
		border-top: 1px solid var(--color-border);
		border-bottom: 1px solid var(--color-border);
	}

	.card.view-compact .details {
		gap: 8px;
		padding: 6px 0;
	}

	.card.view-list .details {
		padding: 0;
		border-top: none;
		border-bottom: none;
	}

	.kv-row {
		display: flex;
		gap: 14px;
		align-items: flex-start;
	}

	.kv {
		display: flex;
		flex-direction: column;
		gap: 2px;
		min-width: 0;
	}

	.kv-grow { flex: 1; }
	.kv-shrink { flex-shrink: 0; }
	.kv-right { align-items: flex-end; }

	.kv-label {
		font-size: 10px;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--color-text-muted);
	}

	.kv-value {
		font-size: 12px;
		font-family: var(--font-mono);
		color: var(--color-text-secondary);
	}

	.kv-secondary {
		color: var(--color-text-muted);
	}

	.kv-endpoint {
		display: flex;
		align-items: center;
		gap: 4px;
		min-width: 0;
	}

	.eye-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		padding: 2px;
		border: none;
		background: none;
		color: var(--color-text-muted);
		cursor: pointer;
		border-radius: var(--radius-sm);
		flex-shrink: 0;
		transition: color var(--t-fast) ease;
	}

	.eye-btn:hover {
		color: var(--color-text-secondary);
	}

	.divider {
		border: none;
		border-top: 1px dashed var(--color-border);
		margin: 4px 0;
	}

	.stats-row {
		white-space: nowrap;
	}

	.truncate {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	/* Actions */
	.actions {
		display: flex;
		gap: 4px;
		justify-content: flex-end;
		align-items: center;
	}

	.card.view-list .actions {
		flex-direction: column;
		align-items: stretch;
		justify-content: flex-start;
	}

	.action-btn {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 5px 9px;
		font-size: 11px;
		font-weight: 500;
		border: none;
		background: transparent;
		color: var(--color-text-secondary);
		cursor: pointer;
		border-radius: var(--radius-sm);
		text-decoration: none;
		font-family: inherit;
		transition: background var(--t-fast) ease, color var(--t-fast) ease;
	}

	.action-btn:hover:not(:disabled) {
		background: var(--color-bg-hover);
		color: var(--color-text-primary);
	}

	.action-btn:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.action-danger:hover:not(:disabled) {
		color: var(--color-error);
		background: var(--color-error-tint);
	}
	.action-btn.action-test:hover:not(:disabled),
	.list-actions :global(.action-btn.action-test:hover:not(:disabled)) {
		color: var(--color-success);
		background: var(--color-success-tint);
	}

	.action-spinner {
		width: 12px;
		height: 12px;
		border: 2px solid currentColor;
		border-top-color: transparent;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	/* Chart */
	.chart-section {
		margin: 0 -16px -14px;
		border-radius: 0 0 var(--radius) var(--radius);
		background: var(--color-bg-secondary);
		overflow: hidden;
	}

	.card.view-compact .chart-section {
		margin: 0 -14px -12px;
	}

	.card.view-list .chart-section {
		grid-column: 1 / -1;
		margin: 0;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-sm);
	}

	.chart-body {
		padding: 0 12px 4px;
	}

	@media (max-width: 720px) {
		.card.view-list .actions,
		.actions {
			display: grid;
			grid-template-columns: repeat(3, minmax(0, 1fr));
			gap: 4px;
		}

		.action-btn {
			width: 100%;
			justify-content: center;
			padding-inline: 6px;
		}

		.action-btn svg {
			flex-shrink: 0;
		}
	}

	@media (max-width: 1080px) {
		.list-card {
			grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		}

		.list-cell-actions {
			grid-column: 1 / -1;
		}

		.list-actions {
			flex-direction: row;
			flex-wrap: wrap;
			justify-content: flex-end;
		}

		.card.view-list {
			grid-template-columns: minmax(0, 1fr);
		}

		.card.view-list .actions {
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
