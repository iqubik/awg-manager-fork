<!-- frontend/src/lib/components/routing/singboxRouter/ConnectionsBreakdownPanel.svelte -->
<script lang="ts">
	import type { ConnectionBucket } from '$lib/types/singboxConnections';
	import { formatBytes } from '$lib/utils/format';

	interface Props {
		title: string;
		buckets: ConnectionBucket[];
		activeKey: string; // '' = none active
		onSelect: (key: string) => void;
	}

	let { title, buckets, activeKey, onSelect }: Props = $props();

	const TOP_N = 3;
	const PALETTE = ['#da7756', '#4a9eff', '#3d9970', '#9c8aff', '#dab856'];
	const OTHER_COLOR = '#6e6e6e';

	const totalCount = $derived(buckets.reduce((s, b) => s + b.count, 0));

	// Top-N for donut + legend, "Other" rolls up remainder.
	const donutSlices = $derived.by(() => {
		const top = buckets.slice(0, TOP_N).map((b, i) => ({
			key: b.key,
			pct: b.pct,
			color: PALETTE[i % PALETTE.length],
		}));
		const otherPct = Math.max(0, 100 - top.reduce((s, x) => s + x.pct, 0));
		if (otherPct > 0 && buckets.length > TOP_N) {
			top.push({ key: 'Other', pct: otherPct, color: OTHER_COLOR });
		}
		return top;
	});

	// SVG donut math: r=40, perimeter ≈ 251.33
	const RADIUS = 40;
	const PERIMETER = 2 * Math.PI * RADIUS;

	function dashArray(pct: number): string {
		const len = (pct / 100) * PERIMETER;
		return `${len} ${PERIMETER}`;
	}
	function offset(prevPct: number): number {
		return -(prevPct / 100) * PERIMETER;
	}

	function bucketColor(key: string): string {
		const idx = donutSlices.findIndex((s) => s.key === key);
		if (idx < 0) return OTHER_COLOR;
		return donutSlices[idx].color;
	}
</script>

<div class="panel">
	<div class="panel-head">
		<span>{title}</span>
		<span class="muted">{buckets.length}</span>
	</div>

	<div class="pie-block">
		<svg width="96" height="96" viewBox="0 0 96 96" class="pie">
			<circle cx="48" cy="48" r={RADIUS} fill="none" stroke="var(--surface-2, #2c3134)" stroke-width="14" />
			{#each donutSlices as s, i}
				{@const prev = donutSlices.slice(0, i).reduce((sum, x) => sum + x.pct, 0)}
				<circle
					cx="48" cy="48" r={RADIUS}
					fill="none"
					stroke={s.color}
					stroke-width="14"
					stroke-dasharray={dashArray(s.pct)}
					stroke-dashoffset={offset(prev)}
				/>
			{/each}
		</svg>
		<div class="pie-center">
			<div class="pie-num">{totalCount}</div>
			<div class="pie-lbl">conn</div>
		</div>

		<div class="legend">
			{#each donutSlices as s}
				<div class="legend-row">
					<span class="legend-dot" style="background:{s.color}"></span>
					<span class="legend-name">{s.key}</span>
					<span class="legend-pct">{s.pct}%</span>
				</div>
			{/each}
		</div>
	</div>

	<div class="buckets">
		{#each buckets as b (b.key)}
			<button
				type="button"
				class="bucket"
				class:active={activeKey === b.key}
				onclick={() => onSelect(b.key)}
				title={`↑ ${formatBytes(b.upload)} · ↓ ${formatBytes(b.download)} · ${b.count} conn`}
			>
				<span class="bg" style="width: {b.pct}%; background:{bucketColor(b.key)}1a"></span>
				<span class="bucket-row">
					<span class="bucket-key">{b.key}</span>
					<span class="bucket-stats">
						↑{formatBytes(b.upload)} · ↓{formatBytes(b.download)} <span class="dot">·</span>
						<span class="count">{b.count}</span>
					</span>
				</span>
			</button>
		{/each}
		{#if buckets.length === 0}
			<div class="empty">Нет данных</div>
		{/if}
	</div>
</div>

<style>
	.panel {
		background: var(--surface-1, #1f2425);
		border: 1px solid var(--border-1, #2c3134);
		border-radius: 6px;
		overflow: hidden;
		display: flex;
		flex-direction: column;
		height: 100%;
	}
	.panel-head {
		padding: 8px 12px;
		border-bottom: 1px solid var(--border-1, #2c3134);
		display: flex;
		justify-content: space-between;
		font-size: 10px;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-secondary, #b8b6b3);
	}
	.muted { opacity: 0.5; }
	.pie-block {
		padding: 14px 12px;
		border-bottom: 1px solid var(--border-1, #2c3134);
		display: flex;
		align-items: center;
		gap: 14px;
		position: relative;
	}
	.pie { display: block; transform: rotate(-90deg); flex-shrink: 0; }
	.pie-center {
		position: absolute;
		left: 12px; top: 14px;
		width: 96px; height: 96px;
		display: flex; flex-direction: column;
		align-items: center; justify-content: center;
		text-align: center; pointer-events: none;
	}
	.pie-num {
		font-size: 14px; font-weight: 600;
		color: var(--text-primary, #e8e6e3);
		font-family: ui-monospace, monospace;
	}
	.pie-lbl {
		font-size: 9px; color: var(--text-tertiary, #6e6e6e);
		text-transform: uppercase; letter-spacing: 0.05em;
	}
	.legend {
		display: flex; flex-direction: column;
		gap: 4px; font-size: 11px; flex: 1; min-width: 0;
	}
	.legend-row { display: flex; align-items: center; gap: 6px; }
	.legend-dot { width: 9px; height: 9px; border-radius: 2px; flex-shrink: 0; }
	.legend-name {
		flex: 1; overflow: hidden; text-overflow: ellipsis;
		white-space: nowrap; color: var(--text-secondary, #b8b6b3);
	}
	.legend-pct {
		color: var(--text-tertiary, #6e6e6e);
		font-family: ui-monospace, monospace; font-size: 10px;
	}
	.buckets { flex: 1; min-height: 180px; max-height: 280px; overflow-y: auto; }
	.bucket {
		all: unset;
		display: block;
		width: 100%;
		padding: 6px 12px;
		border-bottom: 1px solid var(--border-1, #2c3134);
		position: relative;
		cursor: pointer;
		box-sizing: border-box;
	}
	.bucket:hover { background: var(--surface-hover, #262a2c); }
	.bucket.active { background: rgba(218, 119, 86, 0.12); }
	.bg {
		position: absolute;
		left: 0; top: 0; bottom: 0;
		pointer-events: none;
	}
	.bucket-row {
		position: relative;
		display: flex; justify-content: space-between; align-items: baseline;
		gap: 8px;
	}
	.bucket-key { font-size: 12px; color: var(--text-primary, #e8e6e3); }
	.bucket.active .bucket-key { color: #da7756; font-weight: 500; }
	.bucket-stats {
		display: flex; gap: 6px; flex-shrink: 0;
		font-family: ui-monospace, monospace;
		font-size: 11px; color: var(--text-secondary, #b8b6b3);
		font-variant-numeric: tabular-nums;
		white-space: nowrap;
	}
	.dot { color: var(--text-tertiary, #6e6e6e); }
	.count { color: var(--text-tertiary, #6e6e6e); }
	.empty {
		text-align: center; padding: 20px; font-size: 12px;
		color: var(--text-tertiary, #6e6e6e);
	}
</style>
