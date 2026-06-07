<script lang="ts">
	import { TableSortHeader } from '$lib/components/ui';

	interface Props {
		label: string;
		sortKey?: string;
		activeSortKey?: string | null;
		sortAsc?: boolean;
		onchange?: (key: string) => void;
		align?: 'left' | 'right' | 'center';
	}

	let {
		label,
		sortKey,
		activeSortKey = null,
		sortAsc = true,
		onchange,
		align = 'left',
	}: Props = $props();
</script>

<span class="tunnel-list-col-head" class:tunnel-list-col-head--right={align === 'right'} class:tunnel-list-col-head--center={align === 'center'}>
	{#if sortKey && onchange}
		<TableSortHeader
			{label}
			{sortKey}
			{activeSortKey}
			{sortAsc}
			{onchange}
		/>
	{:else}
		{label}
	{/if}
</span>

<style>
	.tunnel-list-col-head {
		display: block;
	}

	.tunnel-list-col-head--right {
		text-align: right;
	}

	.tunnel-list-col-head--center {
		text-align: center;
	}
</style>
