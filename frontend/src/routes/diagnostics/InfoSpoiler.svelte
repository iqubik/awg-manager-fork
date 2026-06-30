<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		title: string;
		summary?: string;
		open: boolean;
		onToggle: () => void;
		body?: Snippet;
	}

	let {
		title,
		summary = '',
		open,
		onToggle,
		body,
	}: Props = $props();
</script>

<section class="info-spoiler" class:expanded={open}>
	<header class="head">
		<button
			class="title-btn"
			type="button"
			onclick={onToggle}
			aria-expanded={open}
		>
			<span class="name">{title}</span>
		</button>
		{#if summary}
			<span class="summary">{summary}</span>
		{/if}
		<button
			class="chev"
			type="button"
			onclick={onToggle}
			aria-label={open ? 'Свернуть' : 'Развернуть'}
		>
			<span class:rotated={open}>›</span>
		</button>
	</header>

	{#if open && body}
		<div class="body">
			{@render body()}
		</div>
	{/if}
</section>

<style>
	.info-spoiler {
		font-size: 13px;
		border: 1px solid var(--color-border, var(--border));
		border-radius: var(--radius);
		background: var(--color-bg-secondary, var(--bg-secondary));
		overflow: hidden;
	}

	.head {
		display: flex;
		align-items: center;
		gap: 10px;
		padding: 10px 14px;
		min-height: 40px;
	}

	.head:hover {
		background: var(--color-bg-hover, var(--bg-hover));
	}

	.title-btn {
		display: flex;
		align-items: center;
		gap: 8px;
		flex: 1;
		min-width: 0;
		background: transparent;
		border: none;
		padding: 0;
		cursor: pointer;
		font: inherit;
		color: var(--color-text-primary);
		text-align: left;
	}

	.name {
		font-weight: 600;
		font-size: 13px;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.summary {
		font-family: var(--font-mono);
		font-size: 11px;
		color: var(--color-text-muted, var(--text-muted));
		flex-shrink: 0;
	}

	.chev {
		background: transparent;
		border: none;
		padding: 4px 6px;
		cursor: pointer;
		color: var(--color-text-muted, var(--text-muted));
		font-size: 16px;
		line-height: 1;
		flex-shrink: 0;
	}

	.chev span {
		display: inline-block;
		transition: transform var(--t-fast) ease;
	}

	.chev .rotated {
		transform: rotate(90deg);
	}

	.body {
		border-top: 1px solid var(--color-border, var(--border));
		padding: 8px 14px 12px;
		display: flex;
		flex-direction: column;
		gap: 4px;
		background: color-mix(in srgb, var(--color-bg-primary, var(--bg-primary)) 35%, transparent);
	}

	@media (max-width: 640px) {
		.head {
			flex-wrap: wrap;
			row-gap: 0.35rem;
		}

		.title-btn {
			min-width: min(100%, 16rem);
		}

		.summary {
			order: 3;
			width: 100%;
			flex-basis: 100%;
		}

		.chev {
			margin-left: auto;
		}

		.body {
			padding-inline: 12px;
		}
	}
</style>
