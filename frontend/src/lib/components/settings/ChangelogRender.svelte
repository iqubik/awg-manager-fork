<script lang="ts" module>
	// Inline markdown → HTML string. Order matters: escape HTML first, then
	// substitute backtick-code (so ** and * inside code aren't touched), then
	// bold, then italic. All three substitutions operate on escaped text, so
	// no user content reaches the browser as raw HTML.
	export function parseInline(text: string): string {
		const escaped = text
			.replace(/&/g, '&amp;')
			.replace(/</g, '&lt;')
			.replace(/>/g, '&gt;')
			.replace(/"/g, '&quot;')
			.replace(/'/g, '&#39;');
		return escaped
			.replace(/`([^`]+)`/g, '<code>$1</code>')
			.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
			.replace(/(^|[^*])\*([^*\s][^*]*?)\*(?!\*)/g, '$1<em>$2</em>')
			.replace(/(^|[^_])_([^_\s][^_]*?)_(?!_)/g, '$1<em>$2</em>');
	}
</script>

<script lang="ts">
	import type { ChangelogEntry } from '$lib/types';
	import {
		changelogItemKey,
		changelogVersionKey,
		createInitialAccordionState,
		isChangelogAccordionOpen,
		isChangelogVersionOpen,
		persistChangelogAccordionState,
		readChangelogAccordionState,
		splitChangelogItem,
		toggleChangelogAccordionState,
		toggleChangelogVersionState,
	} from './changelogAccordion';

	interface Props {
		entries: ChangelogEntry[];
	}

	let { entries }: Props = $props();
	let accordionState = $state(createInitialAccordionState([]));
	let accordionStateInitialized = $state(false);

	const GROUP_LABELS: Record<string, string> = {
		Added: 'Добавлено',
		Fixed: 'Исправлено',
		Changed: 'Изменено',
		Removed: 'Удалено',
		Security: 'Безопасность',
		Breaking: 'Breaking changes',
	};

	function label(heading: string): string {
		return GROUP_LABELS[heading] ?? heading;
	}

	function panelId(version: string, groupTitle: string, groupIndex: number, itemIndex: number): string {
		const slug = `${version}-${groupTitle}-${groupIndex}-${itemIndex}`
			.toLowerCase()
			.replace(/[^a-z0-9_-]+/g, '-')
			.replace(/^-+|-+$/g, '');
		return `changelog-accordion-${slug}`;
	}

	function versionPanelId(version: string, entryIndex: number): string {
		const slug = `${version}-${entryIndex}`
			.toLowerCase()
			.replace(/[^a-z0-9_-]+/g, '-')
			.replace(/^-+|-+$/g, '');
		return `changelog-version-accordion-${slug}`;
	}

	function toggleAccordion(itemKey: string): void {
		accordionState = toggleChangelogAccordionState(accordionState, itemKey);
		persistChangelogAccordionState(accordionState);
	}

	function toggleVersionAccordion(versionKey: string): void {
		accordionState = toggleChangelogVersionState(accordionState, versionKey);
		persistChangelogAccordionState(accordionState);
	}

	$effect(() => {
		if (accordionStateInitialized || entries.length === 0) return;
		accordionState = readChangelogAccordionState(entries);
		accordionStateInitialized = true;
	});
</script>

<div class="changelog">
	{#each entries as e, entryIndex (e.version)}
		{@const versionKey = changelogVersionKey(e.version)}
		{@const versionOpen = isChangelogVersionOpen(accordionState, versionKey)}
		{@const versionContentId = versionPanelId(e.version, entryIndex)}
		<section class="entry version-accordion">
			<button
				type="button"
				class="version-accordion-trigger"
				aria-expanded={versionOpen}
				aria-controls={versionContentId}
				onclick={() => toggleVersionAccordion(versionKey)}
			>
				<span class="entry-version">{e.version}</span>
				<span class="entry-date">{e.date}</span>
				<span class:open={versionOpen} class="version-accordion-chevron">▾</span>
			</button>

			{#if versionOpen}
				<div id={versionContentId} class="version-accordion-panel">
					{#each e.groups as g, groupIndex}
						{#if g.heading}
							<h4 class="group-heading">{label(g.heading)}</h4>
							<ul class="group-items">
								{#each g.items as item, itemIndex}
									{@const split = splitChangelogItem(item)}
									{@const itemKey = changelogItemKey(e.version, g.heading, split.title)}
									{@const isOpen = isChangelogAccordionOpen(accordionState, itemKey)}
									{@const itemPanelId = panelId(e.version, g.heading, groupIndex, itemIndex)}
									<li class="changelog-accordion-item">
										{#if split.details}
											<button
												type="button"
												class="changelog-accordion-trigger"
												aria-expanded={isOpen}
												aria-controls={itemPanelId}
												onclick={() => toggleAccordion(itemKey)}
											>
												<span class="changelog-accordion-title">{split.title}</span>
												<span class:open={isOpen} class="changelog-accordion-chevron">▾</span>
											</button>

											{#if isOpen}
												<div id={itemPanelId} class="changelog-accordion-panel">
													{split.details}
												</div>
											{/if}
										{:else}
											<div class="changelog-accordion-static">
												<span class="changelog-accordion-title">{split.title}</span>
											</div>
										{/if}
									</li>
								{/each}
							</ul>
						{:else}
							<div class="group-intro">
								{#each g.items as item}
									<p>{@html parseInline(item)}</p>
								{/each}
							</div>
						{/if}
					{/each}
				</div>
			{/if}
		</section>
	{/each}
</div>

<style>
	.changelog {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.entry {
		padding-bottom: 8px;
		border-bottom: 1px solid var(--border);
		min-width: 0;
	}
	.entry:last-child {
		border-bottom: none;
	}
	.version-accordion {
		padding-bottom: 8px;
	}
	.version-accordion-trigger {
		width: 100%;
		border: 1px solid transparent;
		background: transparent;
		padding: 10px 8px 6px;
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto auto;
		gap: 10px;
		align-items: center;
		text-align: left;
		color: inherit;
		cursor: pointer;
		border-radius: 10px;
		transition:
			background-color 0.16s ease,
			border-color 0.16s ease,
			color 0.16s ease;
	}
	.version-accordion-trigger:hover {
		background: color-mix(in srgb, var(--bg-secondary) 88%, var(--text-primary) 12%);
		border-color: color-mix(in srgb, var(--border) 72%, transparent);
	}
	.version-accordion-trigger:focus-visible {
		outline: none;
		background: color-mix(in srgb, var(--bg-secondary) 80%, var(--accent) 20%);
		border-color: color-mix(in srgb, var(--border) 44%, var(--accent) 56%);
	}
	.version-accordion-trigger[aria-expanded='true'] {
		background: color-mix(in srgb, var(--bg-secondary) 82%, var(--accent) 18%);
		border-color: color-mix(in srgb, var(--border) 56%, var(--accent) 44%);
	}
	.entry-version {
		font-size: 1rem;
		color: var(--text-primary);
		font-weight: 600;
		min-width: 0;
	}
	.entry-date {
		color: var(--text-muted);
		font-size: 0.8125rem;
		font-variant-numeric: tabular-nums;
	}
	.version-accordion-chevron {
		font-size: 0.92rem;
		line-height: 1;
		color: var(--text-secondary);
		opacity: 0.8;
		transform: rotate(-90deg);
		transition:
			transform 0.12s ease,
			color 0.12s ease,
			opacity 0.12s ease;
	}
	.version-accordion-chevron.open {
		transform: rotate(0deg);
		color: var(--text-primary);
		opacity: 1;
	}
	.version-accordion-panel {
		padding-bottom: 8px;
	}
	.group-intro {
		margin: 0 0 12px;
	}
	.group-intro p {
		margin: 0 0 8px;
		font-size: 0.875rem;
		color: var(--text-primary);
		line-height: 1.45;
	}
	.group-intro p:last-child {
		margin-bottom: 0;
	}
	.group-heading {
		margin: 8px 0 4px;
		font-size: 0.8125rem;
		font-weight: 600;
		color: var(--accent);
		text-transform: none;
	}
	.group-items {
		margin: 0 0 8px;
		padding-left: 0;
		list-style: none;
		display: flex;
		flex-direction: column;
		gap: 0;
		min-width: 0;
	}
	.changelog-accordion-item {
		min-width: 0;
		margin: 0;
		padding: 0;
		border-top: 1px solid color-mix(in srgb, var(--border) 78%, transparent);
	}
	.changelog-accordion-trigger {
		width: 100%;
		min-width: 0;
		border: 1px solid transparent;
		background: transparent;
		padding: 8px 10px;
		border-radius: 10px;
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		gap: 8px;
		align-items: center;
		text-align: left;
		cursor: pointer;
		color: var(--text-primary);
		transition:
			background-color 0.16s ease,
			border-color 0.16s ease,
			color 0.16s ease,
			box-shadow 0.16s ease;
	}
	.changelog-accordion-trigger:hover {
		background: color-mix(in srgb, var(--bg-secondary) 84%, var(--accent) 16%);
		border-color: color-mix(in srgb, var(--border) 62%, var(--accent) 38%);
	}
	.changelog-accordion-trigger:focus-visible {
		outline: none;
		background: color-mix(in srgb, var(--bg-secondary) 78%, var(--accent) 22%);
		border-color: color-mix(in srgb, var(--border) 44%, var(--accent) 56%);
		box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent) 42%, transparent);
	}
	.changelog-accordion-trigger[aria-expanded='true'] {
		background: color-mix(in srgb, var(--bg-secondary) 76%, var(--accent) 24%);
		border-color: color-mix(in srgb, var(--border) 48%, var(--accent) 52%);
	}
	.changelog-accordion-title {
		min-width: 0;
		font-size: 0.92rem;
		line-height: 1.32;
		white-space: normal;
		overflow-wrap: anywhere;
		word-break: break-word;
		color: var(--text-primary);
	}
	.changelog-accordion-chevron {
		align-self: start;
		margin-top: 1px;
		font-size: 0.9rem;
		line-height: 1;
		color: var(--text-secondary);
		opacity: 0.9;
		transform: rotate(-90deg);
		transition:
			transform 0.16s ease,
			color 0.16s ease,
			opacity 0.16s ease;
	}
	.changelog-accordion-chevron.open {
		transform: rotate(0deg);
		color: var(--text-primary);
	}
	.changelog-accordion-panel {
		margin: 4px 0 8px;
		padding: 0 10px 10px;
		color: var(--text-secondary);
		font-size: 0.86rem;
		line-height: 1.45;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
		animation: changelogAccordionReveal 0.16s ease;
	}
	.changelog-accordion-static {
		min-width: 0;
		padding: 8px 10px;
		border-radius: 10px;
		color: var(--text-primary);
		transition: background-color 0.16s ease;
	}
	.changelog-accordion-static:hover {
		background: color-mix(in srgb, var(--bg-secondary) 88%, var(--text-primary) 12%);
	}
	.group-items :global(code) {
		background: var(--bg-tertiary);
		padding: 0 4px;
		border-radius: 3px;
		font-family: ui-monospace, monospace;
		font-size: 0.8125rem;
	}
	.group-items :global(strong) {
		color: var(--text-primary);
		font-weight: 600;
	}
	@keyframes changelogAccordionReveal {
		from {
			opacity: 0;
			transform: translateY(-2px);
		}
		to {
			opacity: 1;
			transform: translateY(0);
		}
	}
</style>
