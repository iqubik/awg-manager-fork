<script lang="ts">
	import { Dropdown, type DropdownOption } from '$lib/components/ui';

	const DEFAULT_SORT_VALUE = '__default__';

	interface SortOption {
		value: string | null;
		label: string;
	}

	interface Props {
		searchQuery: string;
		sortKey: string | null;
		sortAsc: boolean;
		options: SortOption[];
		showSearch?: boolean;
		/** When false, toolbar is search-only (sort via table headers on desktop list). */
		showSort?: boolean;
		hideSortOnDesktop?: boolean;
		onSearchChange: (value: string) => void;
		onSortChange: (key: string | null) => void;
		onToggleDir: () => void;
	}

	let {
		searchQuery,
		sortKey,
		sortAsc,
		options,
		showSearch = false,
		showSort = true,
		hideSortOnDesktop = false,
		onSearchChange,
		onSortChange,
		onToggleDir,
	}: Props = $props();

	const dropdownOptions = $derived(
		([
			{ value: DEFAULT_SORT_VALUE, label: 'Исходный порядок' },
			...options.map((option) => ({ value: option.value ?? DEFAULT_SORT_VALUE, label: option.label })),
		] satisfies DropdownOption<string>[])
	);
</script>

<div class="tunnel-sort-controls" class:hide-sort-on-desktop={hideSortOnDesktop}>
	{#if showSearch}
		<input
			class="tunnel-search"
			type="text"
			placeholder="Поиск..."
			value={searchQuery}
			oninput={(e) => onSearchChange((e.currentTarget as HTMLInputElement).value)}
		/>
	{/if}
	{#if showSort}
		<div class="tunnel-sort-ui">
			<div class="tunnel-sort-select">
				<Dropdown
					value={sortKey ?? DEFAULT_SORT_VALUE}
					options={dropdownOptions}
					onchange={(k) => onSortChange(k === DEFAULT_SORT_VALUE ? null : k)}
					fullWidth
				/>
			</div>
			<button class="tunnel-sort-dir" type="button" onclick={onToggleDir} title="Направление сортировки">
				{sortAsc ? '↑' : '↓'}
			</button>
		</div>
	{/if}
</div>

<style>
	.tunnel-sort-controls {
		display: flex;
		align-items: center;
		gap: 0.375rem;
	}

	.tunnel-sort-ui {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
	}

	.tunnel-sort-controls.hide-sort-on-desktop .tunnel-sort-ui {
		display: none;
	}

	.tunnel-search {
		width: 140px;
		height: 32px;
		box-sizing: border-box;
		padding: 0 0.5rem;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--bg-primary);
		color: var(--text-primary);
		font-size: 0.6875rem;
	}

	.tunnel-search::placeholder {
		color: var(--text-muted);
	}

	.tunnel-sort-select {
		min-width: 150px;
	}

	.tunnel-sort-dir {
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

	.tunnel-sort-dir:hover {
		background: var(--bg-hover);
		color: var(--text-primary);
	}

	@media (max-width: 760px) {
		.tunnel-sort-controls {
			width: 100%;
		}

		.tunnel-search {
			width: 100%;
			min-width: 0;
		}

		.tunnel-sort-controls:has(.tunnel-sort-ui) {
			display: grid;
			grid-template-columns: minmax(0, 1fr) auto;
			gap: 0.375rem;
		}

		.tunnel-sort-controls:has(.tunnel-sort-ui) .tunnel-search {
			grid-column: 1 / -1;
		}

		.tunnel-sort-select {
			min-width: 0;
			width: 100%;
		}

		.tunnel-sort-dir {
			width: 34px;
			min-width: 34px;
			height: 34px;
		}
	}
</style>
