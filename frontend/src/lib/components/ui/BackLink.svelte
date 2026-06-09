<script lang="ts">
	import { ArrowLeft } from 'lucide-svelte';

	export type BackLinkVariant = 'ghost' | 'accent';

	interface Props {
		href: string;
		label?: string;
		variant?: BackLinkVariant;
	}

	let { href, label = 'Назад', variant = 'ghost' }: Props = $props();
</script>

<a {href} class="back-link" class:variant-accent={variant === 'accent'}>
	<ArrowLeft size={18} strokeWidth={2.1} aria-hidden="true" />
	{label}
</a>

<style>
	.back-link.variant-accent {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 0.4375rem;
		height: 28px;
		min-height: 28px;
		max-height: 28px;
		padding: 0 0.625rem;
		box-sizing: border-box;
		border-radius: var(--radius-sm, 8px);
		background: var(--color-accent);
		color: var(--color-accent-contrast, var(--color-bg-primary));
		border: 1px solid transparent;
		font-family: inherit;
		font-size: 12px;
		font-weight: 500;
		line-height: 1;
		text-decoration: none;
		user-select: none;
		white-space: nowrap;
		vertical-align: middle;
		cursor: pointer;
		transition:
			background var(--t-fast) ease,
			color var(--t-fast) ease,
			border-color var(--t-fast) ease,
			filter var(--t-fast) ease;
	}

	.back-link.variant-accent:hover {
		background: var(--color-accent-hover);
		color: var(--color-accent-contrast, var(--color-bg-primary));
		text-decoration: none;
	}

	.back-link.variant-accent:focus-visible {
		outline: 2px solid color-mix(in srgb, var(--color-accent) 70%, white 20%);
		outline-offset: 2px;
	}

	.back-link.variant-accent :global(svg) {
		flex: 0 0 auto;
	}
</style>
