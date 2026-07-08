<script lang="ts">
	import { ChevronLeft, ChevronRight, Check, X } from 'lucide-svelte';
	import { formatTime } from '$lib/utils/format';
	import { formatWatchdogLatency, formatWatchdogState, type WatchdogCheckEntry } from '$lib/utils/watchdogHistory';

	interface Props {
		entries: WatchdogCheckEntry[];
		pageSize?: number;
		emptyLabel?: string;
	}

	let { entries, pageSize = 5, emptyLabel = 'Нет данных проверок.' }: Props = $props();

	let page = $state(0);
	const total = $derived(entries.length);
	const pageCount = $derived(Math.max(1, Math.ceil(total / pageSize)));
	const start = $derived(page * pageSize);
	const end = $derived(Math.min(start + pageSize, total));
	const pageEntries = $derived(entries.slice(start, end));
	const canPrev = $derived(page > 0);
	const canNext = $derived(end < total);

	$effect(() => {
		if (page >= pageCount) {
			page = Math.max(0, pageCount - 1);
		}
	});

	function prevPage(): void {
		if (canPrev) page -= 1;
	}

	function nextPage(): void {
		if (canNext) page += 1;
	}

</script>

<div class="wd-nav">
	<div class="wd-nav-head">
		<span class="wd-nav-range">
			{#if total > 0}
				{start + 1}–{end} из {total}
			{:else}
				0 из 0
			{/if}
		</span>
		<div class="wd-nav-buttons">
			<button type="button" class="wd-nav-btn" onclick={prevPage} disabled={!canPrev} aria-label="Более новые проверки">
				<ChevronLeft size={14} />
			</button>
			<button type="button" class="wd-nav-btn" onclick={nextPage} disabled={!canNext} aria-label="Более старые проверки">
				<ChevronRight size={14} />
			</button>
		</div>
	</div>

	{#if total === 0}
		<div class="wd-nav-empty">{emptyLabel}</div>
	{:else}
		<div class="wd-nav-log">
			{#each pageEntries as entry (entry.key)}
				{@const stateLabel = formatWatchdogState(entry)}
				<div class="wd-nav-row">
					<span class="wd-nav-ts">{formatTime(entry.timestamp)}</span>
					<span class="wd-nav-ico" class:ok={entry.success} class:bad={!entry.success}>
						{#if entry.success}<Check size={13} />{:else}<X size={13} />{/if}
					</span>
					<span class="wd-nav-lat">{formatWatchdogLatency(entry)}</span>
					<span class="wd-nav-note" title={entry.error}>{entry.error}</span>
					{#if stateLabel}
						<span class="wd-nav-state" title={stateLabel}>{stateLabel}</span>
					{/if}
				</div>
			{/each}
		</div>
	{/if}
</div>

<style>
	.wd-nav {
		display: flex;
		flex-direction: column;
		gap: 0.375rem;
	}

	.wd-nav-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
	}

	.wd-nav-range {
		font-family: var(--font-mono);
		font-size: 11px;
		color: var(--color-text-muted);
	}

	.wd-nav-buttons {
		display: inline-flex;
		align-items: center;
		gap: 0.25rem;
	}

	.wd-nav-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 1.5rem;
		height: 1.5rem;
		border-radius: 6px;
		border: 1px solid color-mix(in srgb, var(--color-border) 80%, transparent);
		background: color-mix(in srgb, var(--color-border) 10%, transparent);
		color: var(--color-text-muted);
		cursor: pointer;
		transition: all var(--t-fast) ease;
	}

	.wd-nav-btn:hover:not(:disabled) {
		color: var(--color-text-primary);
		background: color-mix(in srgb, var(--color-border) 18%, transparent);
	}

	.wd-nav-btn:disabled {
		opacity: 0.45;
		cursor: not-allowed;
	}

	.wd-nav-empty {
		font-size: 11px;
		color: var(--color-text-muted);
		font-style: italic;
	}

	.wd-nav-log {
		display: flex;
		flex-direction: column;
	}

	.wd-nav-row {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 4px 0;
		font-family: var(--font-mono);
		font-size: 11px;
		border-top: 1px solid color-mix(in srgb, var(--color-border) 45%, transparent);
	}

	.wd-nav-row:first-child {
		border-top: none;
	}

	.wd-nav-ts {
		color: var(--color-text-muted);
		min-width: 56px;
	}

	.wd-nav-ico {
		display: inline-flex;
		align-items: center;
	}

	.wd-nav-ico.ok {
		color: var(--color-success);
	}

	.wd-nav-ico.bad {
		color: var(--color-error);
	}

	.wd-nav-lat {
		color: var(--color-text-primary);
		min-width: 52px;
	}

	.wd-nav-note {
		color: var(--color-text-muted);
		font-style: italic;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.wd-nav-state {
		margin-left: auto;
		color: var(--color-text-primary);
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	@container (max-width: 380px) {
		.wd-nav-row {
			display: grid;
			grid-template-columns: 56px 18px 48px minmax(0, 1fr);
			align-items: center;
		}

		.wd-nav-state {
			grid-column: 1 / -1;
			margin-left: 0;
			padding-left: calc(56px + 18px + 48px + 16px);
			white-space: normal;
		}

		.wd-nav-note {
			white-space: normal;
		}
	}
</style>
