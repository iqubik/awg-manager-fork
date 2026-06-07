<script lang="ts">
	import type { Snippet } from 'svelte';
	import { StatusDot } from '$lib/components/ui';
	import type { StatusDotVariant } from '$lib/components/ui/StatusDot.svelte';

	interface Props {
		title: string;
		dotVariant?: StatusDotVariant;
		dotPulse?: boolean;
		dotLabel?: string;
		dense?: boolean;
		staticTitle?: boolean;
		onTitleClick?: () => void;
		badges?: Snippet;
		showDot?: boolean;
	}

	let {
		title,
		dotVariant = 'muted',
		dotPulse = false,
		dotLabel,
		dense = false,
		staticTitle = false,
		showDot = true,
		onTitleClick,
		badges,
	}: Props = $props();
</script>

<div class="tunnel-title-row" class:tunnel-title-row--dense={dense}>
	{#if showDot}
		<StatusDot variant={dotVariant} pulse={dotPulse} size={dense ? 'sm' : 'md'} ariaLabel={dotLabel ?? title} />
	{/if}
	{#if staticTitle || !onTitleClick}
		<span class="tunnel-title-row__name tunnel-title-row__name--static" title={title}>{title}</span>
	{:else}
		<button type="button" class="tunnel-title-row__name" title={title} onclick={onTitleClick}>{title}</button>
	{/if}
	{#if badges}
		<div class="tunnel-title-row__badges">
			{@render badges()}
		</div>
	{/if}
</div>
