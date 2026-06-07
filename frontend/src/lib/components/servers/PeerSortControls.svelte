<script lang="ts">
	import type { PeerSortKey } from '$lib/utils/peerSort';
	import { peerSort } from '$lib/stores/peerSort';
	import { DEFAULT_SORT_VALUE } from '$lib/utils/tableSort';
	import { Dropdown, type DropdownOption } from '$lib/components/ui';

	interface Props {
		searchQuery: string;
		showSearch?: boolean;
		hideSortOnDesktop?: boolean;
	}

	let {
		searchQuery = $bindable(),
		showSearch = false,
		hideSortOnDesktop = false,
	}: Props = $props();

	const sortOptions: DropdownOption<PeerSortKey>[] = [
		{ value: 'name', label: 'По имени' },
		{ value: 'traffic', label: 'По трафику' },
		{ value: 'ip', label: 'По IP' },
		{ value: 'endpoint', label: 'Endpoint' },
		{ value: 'online', label: 'Онлайн' },
		{ value: 'handshake', label: 'Handshake' },
	];

	const dropdownOptions = $derived(
		([
			{ value: DEFAULT_SORT_VALUE, label: 'Исходный порядок' },
			...sortOptions,
		] satisfies DropdownOption<string>[])
	);
</script>

<div class="peer-sort-controls" class:hide-sort-on-desktop={hideSortOnDesktop}>
	{#if showSearch}
		<input
			class="peer-search"
			type="text"
			placeholder="Поиск..."
			bind:value={searchQuery}
		/>
	{/if}
	<div class="peer-sort-ui">
		<div class="peer-sort-select">
			<Dropdown
				value={$peerSort.sortBy ?? DEFAULT_SORT_VALUE}
				options={dropdownOptions}
				onchange={(k) => peerSort.setSortBy(k === DEFAULT_SORT_VALUE ? null : (k as PeerSortKey))}
				fullWidth
			/>
		</div>
		<button
			class="peer-sort-dir"
			disabled={$peerSort.sortBy === null}
			onclick={() => peerSort.toggleDir()}
			title="Направление сортировки"
		>
			{$peerSort.sortAsc ? '↑' : '↓'}
		</button>
	</div>
</div>

<style>
	.peer-sort-controls {
		display: flex;
		align-items: center;
		gap: 0.375rem;
	}

	.peer-sort-ui {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
	}

	.peer-sort-controls.hide-sort-on-desktop .peer-sort-ui {
		display: none;
	}

	.peer-search {
		box-sizing: border-box;
		width: 120px;
		height: 28px;
		min-height: 28px;
		max-height: 28px;
		padding: 0 0.5rem;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--bg-primary);
		color: var(--text-primary);
		font-size: 12px;
		line-height: 1;
	}

	.peer-search::placeholder {
		color: var(--text-muted);
	}

	.peer-sort-select {
		min-width: 130px;
	}

	.peer-sort-dir {
		padding: 0.125rem 0.375rem;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--bg-primary);
		color: var(--text-secondary);
		font-size: 0.75rem;
		cursor: pointer;
		line-height: 1;
		transition: color 0.15s ease, background 0.15s ease;
	}

	.peer-sort-dir:hover:not(:disabled) {
		background: var(--bg-hover);
		color: var(--text-primary);
	}

	.peer-sort-dir:disabled {
		opacity: 0.45;
		cursor: not-allowed;
	}

	@media (max-width: 640px) {
		.peer-sort-controls {
			display: grid;
			grid-template-columns: minmax(0, 1fr) auto;
			gap: 0.375rem;
			width: 100%;
		}

		.peer-search {
			grid-column: 1 / -1;
			width: 100%;
			min-width: 0;
		}

		.peer-sort-select {
			min-width: 0;
			width: 100%;
		}

		.peer-sort-dir {
			width: 34px;
			min-width: 34px;
			height: 34px;
		}

		.peer-sort-controls.hide-sort-on-desktop .peer-sort-ui {
			display: inline-flex;
			grid-column: 1 / -1;
			width: 100%;
		}
	}
</style>
