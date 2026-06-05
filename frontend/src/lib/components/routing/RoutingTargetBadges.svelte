<script lang="ts">
	import { Badge } from '$lib/components/ui';

	interface Props {
		labels: string[];
		/** Optional native tooltips; defaults to labels. */
		titles?: string[];
		/** Noun for overflow aria-label, e.g. «интерфейсов», «туннелей». */
		overflowNoun?: string;
	}

	let { labels, titles = [], overflowNoun = 'целей' }: Props = $props();

	const GAP = 4;

	let containerEl = $state<HTMLDivElement | null>(null);
	let measureEl = $state<HTMLDivElement | null>(null);
	let visibleCount = $state(1);
	let cachedContainerWidth = 0;

	let visibleLabels = $derived(labels.slice(0, visibleCount));
	let overflowCount = $derived(Math.max(0, labels.length - visibleCount));
	let hiddenLabels = $derived(labels.slice(visibleCount));
	let hiddenTitles = $derived(titles.slice(visibleCount));
	let overflowMeasure = $derived(`+${Math.max(1, labels.length - 1)}`);

	function readContainerWidth(): number {
		if (!containerEl) return 0;
		const parent = containerEl.parentElement;
		return Math.max(containerEl.clientWidth, parent?.clientWidth ?? 0);
	}

	function recalc(nextWidth = cachedContainerWidth) {
		const availableWidth = nextWidth || readContainerWidth();
		if (labels.length === 0) return;
		if (!measureEl || availableWidth === 0) {
			visibleCount = 1;
			return;
		}
		cachedContainerWidth = availableWidth;

		const children = Array.from(measureEl.children) as HTMLElement[];
		if (children.length < 2) return;

		const arrowW = children[0].getBoundingClientRect().width;
		const chipW = children[children.length - 1].getBoundingClientRect().width;
		const badgeEls = children.slice(1, -1);
		const total = badgeEls.length;

		let used = arrowW;
		let fit = 0;

		for (let i = 0; i < total; i++) {
			const badgeW = badgeEls[i].getBoundingClientRect().width;
			const isLastBadge = i === total - 1;
			const cost = GAP + badgeW + (isLastBadge ? 0 : GAP + chipW);
			if (used + cost <= availableWidth) {
				used += GAP + badgeW;
				fit++;
			} else {
				break;
			}
		}

		if (fit < total) {
			while (fit > 1) {
				let rowWidth = arrowW;
				for (let i = 0; i < fit; i++) {
					rowWidth += GAP + badgeEls[i].getBoundingClientRect().width;
				}
				rowWidth += GAP + chipW;
				if (rowWidth <= availableWidth) break;
				fit--;
			}
		}

		visibleCount = Math.max(1, fit || 1);
	}

	$effect(() => {
		if (!measureEl || !containerEl) return;
		void labels.length;
		void labels.join('\0');
		cachedContainerWidth = readContainerWidth();
		recalc(cachedContainerWidth);
	});

	$effect(() => {
		if (!containerEl) return;
		const ro = new ResizeObserver(() => {
			cachedContainerWidth = readContainerWidth();
			recalc(cachedContainerWidth);
		});
		ro.observe(containerEl);
		const parent = containerEl.parentElement;
		if (parent) ro.observe(parent);
		return () => ro.disconnect();
	});
</script>

{#if labels.length > 0}
	<div class="fitting-badges" bind:this={containerEl}>
		<div class="measure-row" bind:this={measureEl} aria-hidden="true">
			<span class="route-arrow">&rarr;</span>
			{#each labels as label, index (`${label}:${index}`)}
				<Badge variant="muted" mono size="xs">{label}</Badge>
			{/each}
			<Badge variant="dotted" mono size="xs" compact>{overflowMeasure}</Badge>
		</div>

		<div
			class="visible-row"
			class:sole={labels.length === 1 && overflowCount === 0}
		>
			<span class="route-arrow">&rarr;</span>
			{#each visibleLabels as label, index (index)}
				<Badge variant="muted" mono size="xs" title={titles[index] ?? label}>{label}</Badge>
			{/each}
			{#if overflowCount > 0}
				<span
					class="overflow-tip"
					tabindex="0"
					role="button"
					aria-label={`Ещё ${overflowCount} ${overflowNoun}`}
				>
					<Badge variant="dotted" mono size="xs" compact>+{overflowCount}</Badge>
					<div class="overflow-pop" role="tooltip">
						<div class="overflow-pop-title">Ещё {overflowCount}</div>
						<ul>
							{#each hiddenLabels as label, index (`${label}:${index}`)}
								<li title={hiddenTitles[index] ?? label}>{label}</li>
							{/each}
						</ul>
					</div>
				</span>
			{/if}
		</div>
	</div>
{/if}

<style>
	.fitting-badges {
		position: relative;
		flex: 1 1 auto;
		min-width: 0;
		width: 100%;
		max-width: 100%;
		overflow: visible;
	}

	.measure-row,
	.visible-row {
		display: flex;
		align-items: center;
		flex-wrap: nowrap;
		gap: 4px;
		line-height: 1;
		min-width: 0;
	}

	.measure-row {
		visibility: hidden;
		position: absolute;
		top: 0;
		left: 0;
		pointer-events: none;
		height: 0;
		overflow: hidden;
	}

	.visible-row {
		max-width: 100%;
		overflow: visible;
	}

	.visible-row :global(.badge) {
		flex-shrink: 0;
		max-width: 100%;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.visible-row.sole :global(.badge) {
		flex-shrink: 1;
		min-width: 0;
	}

	.overflow-tip {
		position: relative;
		display: inline-flex;
		overflow: visible;
		outline: none;
	}

	.overflow-tip:hover,
	.overflow-tip:focus-visible {
		z-index: 20;
	}

	.overflow-pop {
		position: absolute;
		right: 0;
		bottom: calc(100% + 8px);
		min-width: 7.5rem;
		max-width: min(16rem, calc(100vw - 16px));
		opacity: 0;
		visibility: hidden;
		transform: translateY(4px);
		transition:
			opacity 0.15s ease,
			transform 0.15s ease,
			visibility 0.15s ease;
		pointer-events: none;
		padding: 6px 8px;
		font-size: 11px;
		line-height: 1.35;
		color: var(--text-secondary);
		background: color-mix(in srgb, var(--bg-tertiary) 90%, var(--bg-secondary));
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		box-shadow: 0 6px 16px rgba(0, 0, 0, 0.3);
		text-align: left;
		white-space: normal;
	}

	.overflow-tip:hover .overflow-pop,
	.overflow-tip:focus-visible .overflow-pop,
	.overflow-tip:focus-within .overflow-pop {
		opacity: 1;
		visibility: visible;
		transform: translateY(0);
	}

	.overflow-pop-title {
		margin-bottom: 4px;
		font-size: 10px;
		font-weight: 600;
		color: var(--text-primary);
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}

	.overflow-pop ul {
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.overflow-pop li {
		font-family: var(--font-mono);
		font-size: 10px;
		color: var(--text-primary);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.overflow-pop li + li {
		margin-top: 3px;
	}
</style>
