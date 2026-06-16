<script lang="ts">
	import {
		Activity,
		ArrowDownUp,
		ChevronDown,
		ChevronUp,
		Clock3,
		Server,
		SlidersHorizontal,
		Type,
		Wifi,
	} from 'lucide-svelte';
	import { Dropdown, type DropdownOption } from '$lib/components/ui';
	import { DEFAULT_SORT_VALUE } from '$lib/utils/tableSort';

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
		/** Show sort controls only on mobile; desktop keeps table-header sorting. */
		mobileSortOnly?: boolean;
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
		mobileSortOnly = false,
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
	const mobileSortOptions = $derived([
		{ value: null, label: 'Ручной', icon: 'manual' },
		...options.map((option) => ({
			value: option.value,
			label: option.label,
			icon: mobileIconFor(option.value),
		})),
	]);

	function mobileIconFor(value: string | null): 'manual' | 'name' | 'status' | 'server' | 'traffic' | 'time' | 'mode' | 'ping' {
		switch (value) {
			case 'name':
			case 'label':
				return 'name';
			case 'status':
			case 'running':
			case 'active':
				return 'status';
			case 'endpoint':
			case 'server':
				return 'server';
			case 'traffic':
				return 'traffic';
			case 'handshake':
			case 'updated':
			case 'delay':
				return 'time';
			case 'mode':
			case 'protocol':
				return 'mode';
			case 'ping':
				return 'ping';
			default:
				return 'manual';
		}
	}

	function handleMobileOptionPress(key: string | null): void {
		if (key === null) {
			onSortChange(null);
			return;
		}
		if (sortKey === key) {
			onToggleDir();
			return;
		}
		onSortChange(key);
	}
</script>

<div class="tunnel-sort-controls" class:mobile-sort-only={mobileSortOnly}>
	{#if showSearch}
		<div class="tunnel-search-wrap">
			<input
				class="tunnel-search"
				type="text"
				placeholder="Поиск..."
				value={searchQuery}
				oninput={(e) => onSearchChange((e.currentTarget as HTMLInputElement).value)}
			/>
			{#if searchQuery.trim()}
				<button
					type="button"
					class="tunnel-search-clear"
					aria-label="Очистить поиск"
					title="Очистить поиск"
					onclick={() => onSearchChange('')}
				>
					×
				</button>
			{/if}
		</div>
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
			<button
				class="tunnel-sort-dir"
				type="button"
				disabled={sortKey === null}
				onclick={onToggleDir}
				title="Направление сортировки"
			>
				{sortAsc ? '↑' : '↓'}
			</button>
		</div>
		<div class="tunnel-sort-mobile" aria-label="Сортировка на мобильном">
			{#each mobileSortOptions as option (option.value ?? '__default__')}
				{@const active = sortKey === option.value || (option.value === null && sortKey === null)}
				<button
					type="button"
					class="tunnel-sort-chip"
					class:is-active={active}
					onclick={() => handleMobileOptionPress(option.value)}
					title={active && option.value !== null
						? `Повторное нажатие меняет направление: ${sortAsc ? 'по возрастанию' : 'по убыванию'}`
						: option.label}
					aria-pressed={active}
				>
					<span class="tunnel-sort-chip-icon" aria-hidden="true">
						{#if option.icon === 'name'}
							<Type size={14} strokeWidth={1.9} />
						{:else if option.icon === 'status'}
							<Activity size={14} strokeWidth={1.9} />
						{:else if option.icon === 'server'}
							<Server size={14} strokeWidth={1.9} />
						{:else if option.icon === 'traffic'}
							<ArrowDownUp size={14} strokeWidth={1.9} />
						{:else if option.icon === 'time'}
							<Clock3 size={14} strokeWidth={1.9} />
						{:else if option.icon === 'mode'}
							<SlidersHorizontal size={14} strokeWidth={1.9} />
						{:else if option.icon === 'ping'}
							<Wifi size={14} strokeWidth={1.9} />
						{:else}
							<SlidersHorizontal size={14} strokeWidth={1.9} />
						{/if}
					</span>
					<span class="tunnel-sort-chip-copy">
						<span class="tunnel-sort-chip-label">{option.label}</span>
						<span class="tunnel-sort-chip-state">
							{#if option.value === null}
								{active ? 'без сортировки' : 'ручной порядок'}
							{:else if active}
								{sortAsc ? 'по возрастанию' : 'по убыванию'}
							{:else}
								выбрать
							{/if}
						</span>
					</span>
					{#if active && option.value !== null}
						<span class="tunnel-sort-chip-dir" aria-hidden="true">
							{#if sortAsc}
								<ChevronUp size={14} strokeWidth={2} />
							{:else}
								<ChevronDown size={14} strokeWidth={2} />
							{/if}
						</span>
					{/if}
				</button>
			{/each}
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

	.tunnel-sort-mobile {
		display: none;
	}

	.tunnel-search-wrap {
		position: relative;
		flex: 1 1 auto;
		min-width: 0;
	}

	.tunnel-search {
		width: 140px;
		height: 32px;
		box-sizing: border-box;
		padding: 0 1.8rem 0 0.5rem;
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--bg-primary);
		color: var(--text-primary);
		font-size: 0.6875rem;
	}

	.tunnel-search-wrap .tunnel-search {
		width: 100%;
	}

	.tunnel-search-clear {
		position: absolute;
		top: 50%;
		right: 0.375rem;
		transform: translateY(-50%);
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 1.125rem;
		height: 1.125rem;
		padding: 0;
		border: 0;
		border-radius: 999px;
		background: transparent;
		color: var(--color-text-muted, var(--text-muted));
		font-size: 1rem;
		line-height: 1;
		cursor: pointer;
		transition:
			color 0.15s ease,
			background 0.15s ease;
	}

	.tunnel-search-clear:hover {
		color: var(--color-text-primary, var(--text-primary));
		background: color-mix(in srgb, var(--color-text-muted, var(--text-muted)) 14%, transparent);
	}

	.tunnel-search-clear:focus-visible {
		outline: 2px solid var(--color-accent, var(--accent));
		outline-offset: 2px;
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

	.tunnel-sort-dir:hover:not(:disabled) {
		background: var(--bg-hover);
		color: var(--text-primary);
	}

	.tunnel-sort-dir:disabled {
		opacity: 0.45;
		cursor: not-allowed;
	}

	@media (max-width: 760px) {
		.tunnel-sort-controls {
			width: 100%;
		}

		.tunnel-search-wrap {
			width: 100%;
			min-width: 0;
		}

		.tunnel-sort-controls:has(.tunnel-sort-ui) {
			display: grid;
			grid-template-columns: minmax(0, 1fr) auto;
			gap: 0.375rem;
		}

		.tunnel-sort-controls:has(.tunnel-sort-ui) .tunnel-search-wrap {
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

		.tunnel-sort-ui {
			display: none;
		}

		.tunnel-sort-controls:has(.tunnel-sort-mobile) {
			display: flex;
			flex-direction: column;
			align-items: stretch;
			gap: 0.5rem;
		}

		.tunnel-sort-controls:has(.tunnel-sort-mobile) .tunnel-search-wrap {
			width: 100%;
		}

		.tunnel-sort-mobile {
			display: flex;
			gap: 0.5rem;
			width: 100%;
			min-width: 0;
			overflow-x: auto;
			padding-bottom: 0.15rem;
			scroll-snap-type: x proximity;
			-webkit-overflow-scrolling: touch;
			scrollbar-width: none;
		}

		.tunnel-sort-mobile::-webkit-scrollbar {
			display: none;
		}

		.tunnel-sort-chip {
			display: inline-flex;
			align-items: center;
			gap: 0.55rem;
			min-width: min(74vw, 13.75rem);
			padding: 0.6rem 0.7rem;
			border: 1px solid color-mix(in srgb, var(--border) 88%, transparent);
			border-radius: calc(var(--radius, 10px) + 2px);
			background:
				linear-gradient(180deg,
					color-mix(in srgb, var(--bg-secondary) 96%, transparent),
					color-mix(in srgb, var(--bg-primary) 98%, transparent));
			color: var(--text-secondary);
			text-align: left;
			scroll-snap-align: start;
			flex: 0 0 auto;
			box-shadow: 0 8px 18px rgba(0, 0, 0, 0.14);
			transition:
				transform 0.18s ease,
				border-color 0.18s ease,
				background 0.18s ease,
				color 0.18s ease;
		}

		.tunnel-sort-chip:active {
			transform: scale(0.985);
		}

		.tunnel-sort-chip.is-active {
			border-color: color-mix(in srgb, var(--accent) 72%, var(--border));
			background:
				linear-gradient(180deg,
					color-mix(in srgb, var(--accent) 18%, var(--bg-secondary)),
					color-mix(in srgb, var(--accent) 10%, var(--bg-primary)));
			color: var(--text-primary);
		}

		.tunnel-sort-chip-icon {
			display: inline-flex;
			align-items: center;
			justify-content: center;
			width: 1.85rem;
			height: 1.85rem;
			border-radius: 999px;
			background: color-mix(in srgb, var(--bg-tertiary) 82%, transparent);
			flex: 0 0 auto;
		}

		.tunnel-sort-chip.is-active .tunnel-sort-chip-icon {
			background: color-mix(in srgb, var(--accent) 22%, transparent);
			color: var(--accent);
		}

		.tunnel-sort-chip-copy {
			display: flex;
			flex-direction: column;
			gap: 0.1rem;
			min-width: 0;
			flex: 1 1 auto;
		}

		.tunnel-sort-chip-label {
			font-size: 0.78rem;
			font-weight: 600;
			line-height: 1.2;
			color: inherit;
		}

		.tunnel-sort-chip-state {
			font-size: 0.65rem;
			line-height: 1.2;
			letter-spacing: 0.02em;
			color: var(--text-muted);
			text-transform: uppercase;
		}

		.tunnel-sort-chip.is-active .tunnel-sort-chip-state {
			color: color-mix(in srgb, var(--accent) 76%, var(--text-secondary));
		}

		.tunnel-sort-chip-dir {
			display: inline-flex;
			align-items: center;
			justify-content: center;
			width: 1.35rem;
			height: 1.35rem;
			border-radius: 999px;
			background: color-mix(in srgb, var(--accent) 18%, transparent);
			color: var(--accent);
			flex: 0 0 auto;
		}
	}

	@media (min-width: 761px) {
		.tunnel-sort-controls.mobile-sort-only .tunnel-sort-ui,
		.tunnel-sort-controls.mobile-sort-only .tunnel-sort-mobile {
			display: none;
		}
	}
</style>
