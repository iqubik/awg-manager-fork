<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { LoadingSpinner } from '$lib/components/layout';
	import { getCachedHistory, setCachedHistory } from '$lib/stores/monitoring';
	import {
		MONITORING_RECENT_ROWS,
		MONITORING_SPARKLINE_POINTS,
	} from '$lib/constants/monitoring';
	import type { MonitoringTarget, MonitoringTunnel, MonitoringSample } from '$lib/types';
	import Sparkline from './Sparkline.svelte';
	import {
		computeMonitoringStats,
		downsampleMonitoringPoints,
		getMonitoringRecentPage,
	} from './matrixDrillDownLogic';

	interface Props {
		target: MonitoringTarget;
		tunnel: MonitoringTunnel;
		historyHours: number;
		sampleIntervalSec: number;
		historyCapacity: number;
		onClose: () => void;
	}

	let { target, tunnel, historyHours, sampleIntervalSec, historyCapacity }: Props = $props();

	let samples = $state<MonitoringSample[]>([]);
	let loading = $state(true);
	let recentPage = $state(0);
	const historyLimit = $derived(historyCapacity);

	onMount(async () => {
		const cached = getCachedHistory(target.id, tunnel.id, historyLimit);
		if (cached) {
			samples = cached;
			loading = false;
		}
		try {
			const fresh = await api.getMonitoringHistory({
				target: target.id,
				tunnelId: tunnel.id,
				limit: historyLimit,
			});
			samples = fresh;
			setCachedHistory(target.id, tunnel.id, fresh, historyLimit);
		} catch {
			// Keep cached samples if any; otherwise stay empty.
		} finally {
			loading = false;
		}
	});

	const stats = $derived(computeMonitoringStats(samples));
	const sparklinePoints = $derived(downsampleMonitoringPoints(samples, MONITORING_SPARKLINE_POINTS));
	const recentPageState = $derived(
		getMonitoringRecentPage({
			samples,
			recentPage,
			rowsPerPage: MONITORING_RECENT_ROWS,
		}),
	);
	const recentPageCount = $derived(recentPageState.recentPageCount);
	const recentPageStart = $derived(recentPageState.recentPageStart);
	const recentPageEnd = $derived(recentPageState.recentPageEnd);
	const recent = $derived(recentPageState.recent);
	const recentRangeLabel = $derived(recentPageState.recentRangeLabel);

	$effect(() => {
		if (recentPage >= recentPageCount) {
			recentPage = Math.max(0, recentPageCount - 1);
		}
	});
</script>

<div class="drill">
	{#if loading && samples.length === 0}
		<div class="loading"><LoadingSpinner /></div>
	{:else}
		<Sparkline points={sparklinePoints} width={420} height={100} />

		<div class="stats">
			<div class="stat">
				<span class="stat-value">{stats.avg !== null ? `${stats.avg}ms` : '—'}</span>
				<span class="stat-label">Среднее</span>
			</div>
			<div class="stat">
				<span class="stat-value">{stats.min !== null ? `${stats.min}ms` : '—'}</span>
				<span class="stat-label">Минимум</span>
			</div>
			<div class="stat">
				<span class="stat-value">{stats.max !== null ? `${stats.max}ms` : '—'}</span>
				<span class="stat-label">Максимум</span>
			</div>
			<div class="stat">
				<span class="stat-value">{stats.lossPct}%</span>
				<span class="stat-label">Потери</span>
			</div>
		</div>

		<div class="recent-head">
			<h4 class="recent-title">История замеров</h4>
			{#if samples.length > 0}
				<div class="recent-nav">
					<span class="recent-range">{recentRangeLabel} из {samples.length}</span>
					<button
						type="button"
						class="recent-nav-btn"
						onclick={() => (recentPage = Math.max(0, recentPage - 1))}
						disabled={recentPage === 0}
					>
						Новее
					</button>
					<button
						type="button"
						class="recent-nav-btn"
						onclick={() => (recentPage = Math.min(recentPageCount - 1, recentPage + 1))}
						disabled={recentPage >= recentPageCount - 1}
					>
						Старее
					</button>
				</div>
			{/if}
		</div>
		<table class="recent">
			<thead>
				<tr>
					<th>Время</th>
					<th>Задержка</th>
					<th>Статус</th>
				</tr>
			</thead>
			<tbody>
				{#each recent as s, i (s.ts + i)}
					<tr>
						<td class="ts">{new Date(s.ts).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' })}</td>
						<td class="lat">{s.ok && s.latencyMs !== null ? `${s.latencyMs}ms` : '—'}</td>
						<td class="status" class:ok={s.ok} class:fail={!s.ok}>{s.ok ? 'ОК' : 'СБОЙ'}</td>
					</tr>
				{/each}
			</tbody>
		</table>

		<small class="footer">Окно: {historyHours} часа · до {historyCapacity} точек · шаг {sampleIntervalSec} секунд</small>
	{/if}
</div>

<style>
	.drill {
		display: flex;
		flex-direction: column;
		gap: 0.875rem;
	}

	.loading {
		display: flex;
		justify-content: center;
		padding: 2rem 0;
	}

	.stats {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 0.5rem;
	}

	.stat {
		background: var(--color-bg-tertiary);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-sm);
		padding: 0.5rem 0.625rem;
		display: flex;
		flex-direction: column;
		gap: 0.125rem;
	}

	.stat-value {
		font-family: var(--font-mono);
		font-size: 16px;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
	}

	.stat-label {
		font-size: 10px;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.05em;
	}

	.recent-title {
		margin: 0.25rem 0 0;
		font-size: 12px;
		font-weight: 600;
		color: var(--color-text-secondary);
	}

	.recent-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
		flex-wrap: wrap;
	}

	.recent-nav {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		flex-wrap: wrap;
	}

	.recent-range {
		font-family: var(--font-mono);
		font-size: 11px;
		color: var(--color-text-muted);
		font-variant-numeric: tabular-nums;
	}

	.recent-nav-btn {
		height: 26px;
		padding: 0 0.625rem;
		border-radius: var(--radius-sm);
		border: 1px solid var(--color-border);
		background: var(--color-bg-secondary);
		color: var(--color-text-secondary);
		font-size: 11px;
		font-weight: 600;
		cursor: pointer;
		transition:
			background var(--t-fast) ease,
			color var(--t-fast) ease,
			border-color var(--t-fast) ease;
	}

	.recent-nav-btn:hover:not(:disabled) {
		background: var(--color-bg-hover);
		color: var(--color-text-primary);
	}

	.recent-nav-btn:disabled {
		opacity: 0.45;
		cursor: not-allowed;
	}

	.recent {
		width: 100%;
		border-collapse: collapse;
		font-family: var(--font-mono);
		font-size: 11px;
	}

	.recent th,
	.recent td {
		padding: 0.25rem 0.5rem;
		border-bottom: 1px solid rgba(255, 255, 255, 0.04);
		text-align: left;
	}

	.recent th {
		font-family: var(--font-sans);
		font-size: 10px;
		color: var(--color-text-muted);
		font-weight: 600;
		text-transform: uppercase;
	}

	.ts { color: var(--color-text-muted); }
	.lat { font-variant-numeric: tabular-nums; }
	.status.ok { color: var(--color-success); }
	.status.fail { color: var(--color-error); }

	.footer {
		font-size: 10px;
		color: var(--color-text-muted);
	}
</style>
