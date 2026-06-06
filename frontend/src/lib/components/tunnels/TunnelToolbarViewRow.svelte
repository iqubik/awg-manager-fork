<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		show?: boolean;
		showSearch?: boolean;
		showViewToggle?: boolean;
		search?: Snippet;
		viewToggle?: Snippet;
	}

	let {
		show = true,
		showSearch = false,
		showViewToggle = false,
		search,
		viewToggle,
	}: Props = $props();
</script>

{#if show && (showSearch || showViewToggle)}
	<div class="toolbar-view-row">
		{#if showSearch && search}
			<div class="tunnel-toolbar-search">
				{@render search()}
			</div>
		{/if}
		{#if showViewToggle && viewToggle}
			{@render viewToggle()}
		{/if}
	</div>
{/if}

<style>
	/* Match TUNNEL_MOBILE_LAYOUT_MAX_WIDTH_PX (760) — same breakpoint as mobile list cards. */
	.toolbar-view-row {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		min-width: 0;
	}

	.tunnel-toolbar-search {
		flex: 1 1 160px;
		min-width: 120px;
		max-width: 220px;
	}

	.tunnel-toolbar-search :global(.tunnel-sort-controls) {
		width: 100%;
	}

	.tunnel-toolbar-search :global(.tunnel-search) {
		width: 100%;
	}

	@media (max-width: 760px) {
		.toolbar-view-row {
			display: grid;
			grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
			gap: 0.5rem;
			width: 100%;
		}

		.toolbar-view-row:not(:has(.tunnel-toolbar-search)) {
			grid-template-columns: minmax(0, 1fr);
		}

		.toolbar-view-row > :only-child {
			grid-column: 1 / -1;
		}

		.tunnel-toolbar-search {
			min-width: 0;
			max-width: none;
			width: 100%;
		}

		.toolbar-view-row :global(.tunnel-sort-controls) {
			display: flex;
			width: 100%;
		}

		.toolbar-view-row :global(.tunnel-search) {
			width: 100%;
			min-width: 0;
		}

		.toolbar-view-row :global(.segmented-control) {
			width: 100%;
			min-width: 0;
			justify-content: stretch;
		}

		.toolbar-view-row :global(.segmented-control--icon .segmented-control-btn) {
			flex: 1 1 28px;
			min-width: 28px;
		}
	}
</style>
