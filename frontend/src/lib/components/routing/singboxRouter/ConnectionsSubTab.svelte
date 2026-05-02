<!-- frontend/src/lib/components/routing/singboxRouter/ConnectionsSubTab.svelte -->
<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import type {
		ClashConnectionsRaw,
		ConnectionFilters,
		ConnectionsSnapshot,
	} from '$lib/types/singboxConnections';
	import { parseSnapshot, matchFilters, aggregateBy } from '$lib/utils/singboxConnections';
	import { createClashWS, type WSStatus } from '$lib/utils/clashWebSocket';
	import { api } from '$lib/api/client';
	import { notifications } from '$lib/stores/notifications';
	import { formatBytes } from '$lib/utils/format';
	import ConnectionsBreakdown from './ConnectionsBreakdown.svelte';
	import ConnectionsFilters from './ConnectionsFilters.svelte';
	import ConnectionsTable from './ConnectionsTable.svelte';
	import ConnectionsBulkBar from './ConnectionsBulkBar.svelte';

	let snapshot = $state<ConnectionsSnapshot>({
		connections: [], downloadTotal: 0, uploadTotal: 0, connectionsTotal: 0,
	});
	let clientsByIP = $state<Map<string, string>>(new Map());
	let wsStatus = $state<WSStatus>('connecting');
	let lastMessageAt = $state(0);
	let tick = $state(0);
	let filters = $state<ConnectionFilters>({ search: '', outbound: '', network: 'all', rule: '' });
	let sortBy = $state<'' | 'download' | 'upload' | 'start' | 'source' | 'destination' | 'outbound'>('download');
	let sortDir = $state<'asc' | 'desc'>('desc');
	let page = $state(0);
	const pageSize = 50;

	let wsClose: (() => void) | null = null;
	let clientsTimer: ReturnType<typeof setInterval> | null = null;
	let staleTimer: ReturnType<typeof setInterval> | null = null;

	const filteredConns = $derived(snapshot.connections.filter((c) => matchFilters(c, filters)));

	const byOutbound = $derived(aggregateBy(filteredConns, (c) => c.outboundLabel));
	const byHost = $derived(aggregateBy(filteredConns, (c) => c.metadata.host || c.metadata.destinationIP));
	const byClient = $derived(aggregateBy(filteredConns, (c) => c.clientName || c.metadata.sourceIP));

	const outboundOptions = $derived(
		Array.from(new Set(snapshot.connections.map((c) => c.chains[0] ?? '').filter(Boolean))).sort()
	);
	const ruleOptions = $derived(
		Array.from(new Set(snapshot.connections.map((c) => c.rule).filter(Boolean))).sort()
	);

	const totalUp = $derived(filteredConns.reduce((s, c) => s + c.upload, 0));
	const totalDown = $derived(filteredConns.reduce((s, c) => s + c.download, 0));

	const statusLabel = $derived.by(() => {
		void tick; // re-evaluate every tick
		const sinceMs = Date.now() - lastMessageAt;
		if (wsStatus === 'open' && lastMessageAt > 0 && sinceMs < 2000) return { dot: '●', text: 'Live', cls: 'ok' };
		if (wsStatus === 'open' && sinceMs < 5000) return { dot: '●', text: 'Live', cls: 'ok' };
		if (wsStatus === 'open') return { dot: '◐', text: 'Stale', cls: 'warn' };
		if (wsStatus === 'connecting') return { dot: '◯', text: 'Подключение…', cls: 'warn' };
		if (wsStatus === 'closed') return { dot: '◯', text: 'Переподключение…', cls: 'err' };
		return { dot: '◯', text: 'Ошибка', cls: 'err' };
	});

	async function refetchClients(): Promise<void> {
		try {
			const data = await api.singboxGetClientsByIP();
			const m = new Map<string, string>();
			for (const [ip, name] of Object.entries(data.clientsByIP ?? {})) {
				m.set(ip.toLowerCase(), name);
			}
			clientsByIP = m;
		} catch {
			/* best-effort, leave existing map */
		}
	}

	function onFilterToggle(kind: 'outbound' | 'host' | 'client', key: string): void {
		page = 0;
		if (kind === 'outbound') {
			filters = { ...filters, outbound: filters.outbound === key ? '' : key };
		} else if (kind === 'host') {
			filters = { ...filters, search: filters.search === key ? '' : key };
		} else {
			filters = { ...filters, search: filters.search === key ? '' : key };
		}
	}

	function onSortChange(k: typeof sortBy): void {
		if (sortBy === k) {
			sortDir = sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			sortBy = k;
			sortDir = 'desc';
		}
	}

	async function killOne(id: string): Promise<void> {
		const removed = snapshot.connections.find((c) => c.id === id);
		snapshot = { ...snapshot, connections: snapshot.connections.filter((c) => c.id !== id) };
		try {
			const r = await fetch(`/api/singbox/clash/connections/${encodeURIComponent(id)}`, {
				method: 'DELETE',
				credentials: 'same-origin',
			});
			if (!r.ok) throw new Error(String(r.status));
			notifications.success('Соединение закрыто');
		} catch {
			if (removed) {
				snapshot = { ...snapshot, connections: [...snapshot.connections, removed] };
			}
			notifications.error('Не удалось закрыть соединение');
		}
	}

	async function killVisible(): Promise<void> {
		const ids = filteredConns.map((c) => c.id);
		const removedSet = new Set(ids);
		snapshot = {
			...snapshot,
			connections: snapshot.connections.filter((c) => !removedSet.has(c.id)),
		};
		const results = await Promise.allSettled(
			ids.map((id) =>
				fetch(`/api/singbox/clash/connections/${encodeURIComponent(id)}`, {
					method: 'DELETE',
					credentials: 'same-origin',
				}),
			),
		);
		const ok = results.filter((r) => r.status === 'fulfilled' && (r.value as Response).ok).length;
		notifications.success(`Закрыто ${ok} из ${ids.length}`);
	}

	onMount(() => {
		void refetchClients();
		clientsTimer = setInterval(refetchClients, 30_000);
		wsClose = createClashWS<ClashConnectionsRaw>(
			'/api/singbox/clash/connections',
			(raw) => {
				snapshot = parseSnapshot(raw, clientsByIP);
				lastMessageAt = Date.now();
			},
			(s) => { wsStatus = s; },
		);
		// Force statusLabel to re-derive every second so "Stale" can flip.
		staleTimer = setInterval(() => { tick += 1; }, 1000);
	});

	onDestroy(() => {
		wsClose?.();
		if (clientsTimer !== null) clearInterval(clientsTimer);
		if (staleTimer !== null) clearInterval(staleTimer);
	});
</script>

<div class="totals">
	<span class="totals-text">
		<span class="totals-count">
			Всего: <strong class="num">{filteredConns.length}</strong>
			{#if filteredConns.length !== snapshot.connectionsTotal}
				<span class="muted">из <span class="num">{snapshot.connectionsTotal}</span></span>
			{/if}
			соединений
		</span>
		<span class="dot">·</span>
		<span class="totals-bytes num">↑ {formatBytes(totalUp)}</span>
		<span class="dot">·</span>
		<span class="totals-bytes num">↓ {formatBytes(totalDown)}</span>
	</span>
	<span class="status status-{statusLabel.cls}">
		<span class="dot-icon">{statusLabel.dot}</span> {statusLabel.text}
	</span>
</div>

<ConnectionsBreakdown {byOutbound} {byHost} {byClient} {filters} {onFilterToggle} />

<ConnectionsFilters {filters} {outboundOptions} {ruleOptions} onChange={(f) => { filters = f; page = 0; }} />

<ConnectionsBulkBar visible={filteredConns} total={snapshot.connectionsTotal} onConfirmKill={killVisible} />

<ConnectionsTable
	connections={filteredConns}
	{sortBy} {sortDir} {onSortChange}
	onKill={killOne}
	{page} {pageSize}
	onPageChange={(p) => (page = p)}
/>

<style>
	.totals {
		display: flex; justify-content: space-between; align-items: center;
		padding: 8px 12px; margin-bottom: 12px;
		background: var(--surface-1, #1f2425);
		border: 1px solid var(--border-1, #2c3134);
		border-radius: 6px;
		font-size: 13px;
		min-height: 36px;
	}
	.muted { color: var(--text-tertiary, #6e6e6e); }
	.dot { color: var(--text-tertiary, #6e6e6e); margin: 0 6px; }
	.num { font-variant-numeric: tabular-nums; }
	.totals-text { display: inline-flex; align-items: baseline; gap: 0; white-space: nowrap; }
	.totals-bytes { display: inline-block; min-width: 90px; }
	.status {
		display: inline-flex; align-items: center; gap: 4px;
		font-size: 12px;
		min-width: 160px;
		justify-content: flex-end;
		font-variant-numeric: tabular-nums;
	}
	.status-ok .dot-icon { color: #3d9970; }
	.status-warn .dot-icon { color: #dab856; }
	.status-err .dot-icon { color: #ff6b6b; }
</style>
