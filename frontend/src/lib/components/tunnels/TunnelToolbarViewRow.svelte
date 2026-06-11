<script lang="ts">
	import type { Snippet } from 'svelte';
	import TunnelTableSortControls from '$lib/components/tunnels/TunnelTableSortControls.svelte';

	const TUNNEL_SEARCH_MIN_ROWS = 5;
	type SortOption = {
		value: string;
		label: string;
	};

	interface Props {
		sourceRowCount?: number;
		showViewToggle?: boolean;
		searchQuery: string;
		sortKey?: string | null;
		sortAsc?: boolean;
		sortOptions?: SortOption[];
		onSearchChange: (value: string) => void;
		onSortChange?: (key: string | null) => void;
		onToggleDir?: () => void;
		viewToggle?: Snippet;
	}

	let {
		sourceRowCount = 0,
		showViewToggle = false,
		searchQuery,
		sortKey = null,
		sortAsc = true,
		sortOptions = [],
		onSearchChange,
		onSortChange = () => {},
		onToggleDir = () => {},
		viewToggle,
	}: Props = $props();

	let showSearch = $derived(sourceRowCount >= TUNNEL_SEARCH_MIN_ROWS);
	let showMobileSort = $derived(sortOptions.length > 0);
	let show = $derived(showSearch || showViewToggle || showMobileSort);
</script>

{#if show && (showSearch || showViewToggle || showMobileSort)}
	<div class="toolbar-view-row">
		{#if showSearch}
			<div class="tunnel-toolbar-search">
				<TunnelTableSortControls
					{searchQuery}
					sortKey={null}
					sortAsc={true}
					options={[]}
					showSearch={true}
					showSort={false}
					{onSearchChange}
					onSortChange={() => {}}
					onToggleDir={() => {}}
				/>
			</div>
		{/if}
		{#if showViewToggle && viewToggle}
			{@render viewToggle()}
		{/if}
		{#if showMobileSort}
			<div class="toolbar-mobile-sort">
				<TunnelTableSortControls
					{searchQuery}
					{sortKey}
					{sortAsc}
					options={sortOptions}
					showSearch={false}
					showSort={true}
					mobileSortOnly={true}
					{onSearchChange}
					{onSortChange}
					{onToggleDir}
				/>
			</div>
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

	.toolbar-mobile-sort {
		display: none;
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

		.toolbar-mobile-sort {
			display: block;
			grid-column: 1 / -1;
			width: 100%;
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
