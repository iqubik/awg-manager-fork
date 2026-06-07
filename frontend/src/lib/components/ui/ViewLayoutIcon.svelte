<script lang="ts">
	import type { ViewLayoutIconName } from './segmentedControl';

	interface Props {
		name: ViewLayoutIconName;
	}

	let { name }: Props = $props();
</script>

{#if name === 'dense'}
	<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
		<rect x="4" y="5" width="7" height="6" rx="1.5" />
		<rect x="13" y="5" width="7" height="6" rx="1.5" />
		<rect x="4" y="13" width="7" height="6" rx="1.5" />
		<rect x="13" y="13" width="7" height="6" rx="1.5" />
	</svg>
{:else if name === 'compact'}
	<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
		<rect x="4" y="5" width="16" height="14" rx="2" />
		<path d="M7 9h10" />
		<path d="M7 13h6" />
	</svg>
{:else}
	<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
		<path d="M9 7h11" />
		<path d="M9 12h11" />
		<path d="M9 17h11" />
		<circle cx="5" cy="7" r="1.2" fill="currentColor" stroke="none" />
		<circle cx="5" cy="12" r="1.2" fill="currentColor" stroke="none" />
		<circle cx="5" cy="17" r="1.2" fill="currentColor" stroke="none" />
	</svg>
{/if}

<style>
	svg {
		width: 1rem;
		height: 1rem;
		display: block;
	}
</style>
