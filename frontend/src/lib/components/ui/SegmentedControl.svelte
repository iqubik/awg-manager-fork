<script lang="ts" generics="T extends string">
	import ViewLayoutIcon from './ViewLayoutIcon.svelte';
	import type { SegmentedOption } from './segmentedControl';

	interface Props {
		value: T;
		options: SegmentedOption<T>[];
		ariaLabel: string;
		disabled?: boolean;
		/** Icon-only buttons (28px); label used for aria-label and title. */
		variant?: 'text' | 'icon';
		onchange: (value: T) => void;
	}

	let {
		value,
		options,
		ariaLabel,
		disabled = false,
		variant = 'text',
		onchange,
	}: Props = $props();

	const isIcon = $derived(variant === 'icon' || options.some((o) => o.icon != null));

	function select(next: T) {
		if (disabled || next === value) return;
		onchange(next);
	}
</script>

<div
	class="segmented-control"
	class:segmented-control--icon={isIcon}
	role="group"
	aria-label={ariaLabel}
>
	{#each options as option (option.value)}
		<button
			type="button"
			class="segmented-control-btn"
			class:active={value === option.value}
			aria-pressed={value === option.value}
			aria-label={isIcon ? option.label : undefined}
			title={isIcon ? option.label : undefined}
			{disabled}
			onclick={() => select(option.value)}
		>
			{#if option.icon}
				<ViewLayoutIcon name={option.icon} />
			{:else}
				{option.label}
			{/if}
		</button>
	{/each}
</div>

<style>
	.segmented-control {
		display: inline-flex;
		align-items: center;
		gap: 0.25rem;
		box-sizing: border-box;
		height: 32px;
		padding: 2px;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-sm);
		background: var(--color-bg-secondary);
		flex-shrink: 0;
	}

	.segmented-control-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		height: 26px;
		padding: 0 12px;
		border: none;
		border-radius: calc(var(--radius-sm) - 2px);
		background: transparent;
		color: var(--color-text-muted);
		font-size: 12px;
		font-weight: 500;
		font-family: inherit;
		cursor: pointer;
		white-space: nowrap;
		transition:
			background var(--t-fast) ease,
			color var(--t-fast) ease;
	}

	.segmented-control--icon .segmented-control-btn {
		width: 28px;
		padding: 0;
	}

	.segmented-control-btn:hover:not(:disabled) {
		background: var(--color-bg-hover);
		color: var(--color-text-primary);
	}

	.segmented-control-btn.active {
		background: var(--color-accent-tint);
		color: var(--color-accent);
		font-weight: 600;
	}

	.segmented-control-btn:focus-visible {
		outline: 2px solid var(--color-accent);
		outline-offset: 2px;
	}

	.segmented-control-btn:disabled {
		opacity: 0.6;
		cursor: not-allowed;
	}

	@media (max-width: 640px) {
		.segmented-control {
			display: flex;
			width: 100%;
		}

		.segmented-control-btn {
			flex: 1;
			min-width: 0;
			padding-inline: 0.5rem;
		}

		.segmented-control--icon .segmented-control-btn {
			width: auto;
			padding: 0;
		}
	}
</style>
