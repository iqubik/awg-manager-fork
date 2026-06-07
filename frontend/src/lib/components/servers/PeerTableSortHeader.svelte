<script lang="ts">
	import { TableSortHeader } from '$lib/components/ui';
	import { peerSort } from '$lib/stores/peerSort';
	import type { PeerSortKey } from '$lib/utils/peerSort';

	interface Props {
		label: string;
		sortKey: PeerSortKey;
	}

	let { label, sortKey }: Props = $props();
</script>

<TableSortHeader
	{label}
	{sortKey}
	activeSortKey={$peerSort.sortBy}
	sortAsc={$peerSort.sortAsc}
	onchange={(key) => peerSort.toggleSort(key)}
/>
