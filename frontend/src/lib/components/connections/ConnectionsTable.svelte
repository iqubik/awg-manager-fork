<script lang="ts">
	import type { ConntrackConnection, ConnectionsPagination, RuleHit } from '$lib/types';
	import { formatBytes } from '$lib/utils/format';
	import { Button, Badge } from '$lib/components/ui';

	interface GroupedRuleHit {
		key: string;
		label: string;
		count: number;
		tooltip: string;
	}

	const mobileSortOptions = [
		{ value: 'proto', label: 'Протокол' },
		{ value: 'src', label: 'Источник' },
		{ value: 'dst', label: 'Назначение' },
		{ value: 'iface', label: 'Интерфейс' },
		{ value: 'state', label: 'Состояние' },
		{ value: 'bytes', label: 'Трафик' },
	] as const;

	interface Props {
		connections: ConntrackConnection[];
		pagination: ConnectionsPagination;
		sortBy: '' | 'proto' | 'src' | 'dst' | 'iface' | 'state' | 'bytes';
		sortDir: 'asc' | 'desc';
		onSortChange: (column: 'proto' | 'src' | 'dst' | 'iface' | 'state' | 'bytes') => void;
		onPageChange: (offset: number) => void;
		/** Плейсхолдер-строки при первой загрузке (пустой список) */
		showSkeleton?: boolean;
	}

	let {
		connections,
		pagination,
		sortBy,
		sortDir,
		onSortChange,
		onPageChange,
		showSkeleton = false,
	}: Props = $props();

	const skelRowWidths = [
		['42%', '55%', '48%', '36%', '52%', '40%'],
		['38%', '62%', '44%', '40%', '44%', '36%'],
		['45%', '50%', '58%', '38%', '48%', '44%'],
	] as const;

	let currentPage = $derived(Math.floor(pagination.offset / pagination.limit) + 1);
	let totalPages = $derived(Math.ceil(pagination.total / pagination.limit) || 1);
	let hasPrev = $derived(pagination.offset > 0);
	let hasNext = $derived(pagination.offset + pagination.limit < pagination.total);

	function prevPage() {
		onPageChange(Math.max(0, pagination.offset - pagination.limit));
	}

	function nextPage() {
		onPageChange(pagination.offset + pagination.limit);
	}

	function groupRules(rules: RuleHit[] = []): GroupedRuleHit[] {
		const groups = new Map<
			string,
			{
				key: string;
				label: string;
				count: number;
				tips: Set<string>;
			}
		>();

		for (const rule of rules) {
			const key = rule.listId || rule.listName || rule.pattern || rule.fqdn || 'unknown';
			const label = rule.listName || rule.listId || rule.pattern || rule.fqdn || '—';

			if (!groups.has(key)) {
				groups.set(key, {
					key,
					label,
					count: 0,
					tips: new Set<string>(),
				});
			}

			const group = groups.get(key);
			if (!group) continue;

			group.count += 1;

			const fqdn = rule.fqdn?.trim();
			const pattern = rule.pattern?.trim();
			const tip = fqdn && pattern ? `${fqdn} (pattern: ${pattern})` : fqdn || pattern || '';
			if (tip) {
				group.tips.add(tip);
			}
		}

		return [...groups.values()].map((group) => ({
			key: group.key,
			label: group.label,
			count: group.count,
			tooltip: [...group.tips].join('\n'),
		}));
	}
</script>

{#snippet sortHeader(column: 'proto' | 'src' | 'dst' | 'iface' | 'state' | 'bytes', label: string)}
	<th class="sortable" class:active={sortBy === column} onclick={() => onSortChange(column)}>
		<span class="sort-label">{label}</span>
		{#if sortBy === column}
			<span class="sort-arrow">{sortDir === 'asc' ? '▲' : '▼'}</span>
		{/if}
	</th>
{/snippet}

<div class="mobile-sortbar" aria-label="Сортировка соединений">
	<span class="mobile-sort-label">Сортировать:</span>
	<div class="mobile-sort-chips">
		{#each mobileSortOptions as option}
			<button
				type="button"
				class="chip mobile-sort-chip"
				class:chip-active={sortBy === option.value}
				onclick={() => onSortChange(option.value)}
			>
				{option.label}
				{#if sortBy === option.value}
					<span class="sort-arrow">{sortDir === 'asc' ? '▲' : '▼'}</span>
				{/if}
			</button>
		{/each}
	</div>
</div>

<div class="mobile-connections">
	{#each connections as conn, i (conn.src + conn.srcPort + conn.dst + conn.dstPort + conn.protocol + i)}
		<article class="conn-card" class:row-tunneled={conn.tunnelId !== ''}>
			<div class="conn-card-head">
				<span class="proto-badge proto-{conn.protocol}">{conn.protocol.toUpperCase()}</span>

				{#if conn.state}
					{@const stateVariant = conn.state === 'ESTABLISHED' ? 'success' : conn.state.startsWith('SYN') ? 'warning' : 'muted'}
					<Badge variant={stateVariant} size="sm">{conn.state}</Badge>
				{:else}
					<Badge variant="muted" size="sm">—</Badge>
				{/if}
			</div>

			<div class="conn-card-row">
				<span class="conn-card-label">Источник</span>
				<span class="conn-card-value mono">
					{conn.src}{#if conn.srcPort > 0}:{conn.srcPort}{/if}
					{#if conn.clientName}
						<span class="client-name">{conn.clientName}</span>
					{/if}
				</span>
			</div>

			<div class="conn-card-row">
				<span class="conn-card-label">Назначение</span>
				<span class="conn-card-value mono">
					{conn.dst}{#if conn.dstPort > 0}:{conn.dstPort}{/if}
					{#if conn.rules && conn.rules.length > 0}
						<div class="rule-badges">
							{#each groupRules(conn.rules) as group (group.key)}
								<span title={group.tooltip}>
									<Badge variant="accent" size="sm">
										{group.label}{group.count > 1 ? ` ×${group.count}` : ''}
									</Badge>
								</span>
							{/each}
						</div>
					{/if}
				</span>
			</div>

			<div class="conn-card-meta">
				<div class="conn-card-row compact-row">
					<span class="conn-card-label">Интерфейс</span>
					<span class="conn-card-value">
						{#if conn.tunnelId}
							<Badge variant="accent" size="sm">{conn.tunnelName}</Badge>
						{:else}
							<Badge variant="muted" size="sm">{conn.interface || '—'}</Badge>
						{/if}
					</span>
				</div>

				<div class="conn-card-row compact-row">
					<span class="conn-card-label">Трафик</span>
					<span class="conn-card-value mono">{formatBytes(conn.bytes)}</span>
				</div>
			</div>
		</article>
	{/each}
</div>

<div class="table-wrapper desktop-table">
	<table class="conn-table">
		<thead>
			<tr>
				{@render sortHeader('proto', 'Протокол')}
				{@render sortHeader('src', 'Источник')}
				{@render sortHeader('dst', 'Назначение')}
				{@render sortHeader('iface', 'Интерфейс')}
				{@render sortHeader('state', 'Состояние')}
				{@render sortHeader('bytes', 'Трафик')}
			</tr>
		</thead>
		<tbody>
			{#each connections as conn, i (conn.src + conn.srcPort + conn.dst + conn.dstPort + conn.protocol + i)}
				<tr class:row-tunneled={conn.tunnelId !== ''}>
					<td>
						<span class="proto-badge proto-{conn.protocol}">{conn.protocol.toUpperCase()}</span>
					</td>
					<td class="mono">
						{conn.src}{#if conn.srcPort > 0}:{conn.srcPort}{/if}
						{#if conn.clientName}
							<span class="client-name">{conn.clientName}</span>
						{/if}
					</td>
					<td class="mono">
						{conn.dst}{#if conn.dstPort > 0}:{conn.dstPort}{/if}
						{#if conn.rules && conn.rules.length > 0}
							<div class="rule-badges">
								{#each groupRules(conn.rules) as group (group.key)}
									<span title={group.tooltip}>
										<Badge variant="accent" size="sm">
											{group.label}{group.count > 1 ? ` ×${group.count}` : ''}
										</Badge>
									</span>
								{/each}
							</div>
						{/if}
					</td>
					<td>
						{#if conn.tunnelId}
							<Badge variant="accent" size="sm">{conn.tunnelName}</Badge>
						{:else}
							<Badge variant="muted" size="sm">{conn.interface || '—'}</Badge>
						{/if}
					</td>
					<td>
						{#if conn.state}
							{@const stateVariant = conn.state === 'ESTABLISHED' ? 'success' : conn.state.startsWith('SYN') ? 'warning' : 'muted'}
							<Badge variant={stateVariant} size="sm">{conn.state}</Badge>
						{:else}
							<Badge variant="muted" size="sm">—</Badge>
						{/if}
					</td>
					<td class="mono">{formatBytes(conn.bytes)}</td>
				</tr>
			{/each}
			{#if showSkeleton && connections.length === 0}
				{#each skelRowWidths as widths, ri (ri)}
					<tr class="row-skel" aria-hidden="true">
						{#each widths as w, ci (ci)}
							<td>
								<span class="cell-skel" style:width={w}></span>
							</td>
						{/each}
					</tr>
				{/each}
			{/if}
		</tbody>
	</table>
</div>

{#if totalPages > 1}
	<div class="pagination">
		<span>Стр. {currentPage} из {totalPages}</span>
		<div class="pagination-btns">
			<Button variant="ghost" size="sm" disabled={!hasPrev} onclick={prevPage}>&larr; Назад</Button>
			<Button variant="ghost" size="sm" disabled={!hasNext} onclick={nextPage}>Далее &rarr;</Button>
		</div>
	</div>
{/if}

<style>
	.table-wrapper {
		overflow-x: auto;
	}

	.mobile-sortbar,
	.mobile-connections {
		display: none;
	}

	.conn-table {
		width: 100%;
		border-collapse: collapse;
		font-size: 0.75rem;
	}

	.conn-table th {
		text-align: left;
		padding: 0.5rem 0.625rem;
		color: var(--color-text-muted);
		font-weight: 500;
		font-size: 0.8125rem;
		letter-spacing: 0.01em;
		border-bottom: 1px solid var(--color-border);
		white-space: nowrap;
	}

	.conn-table td {
		padding: 0.4375rem 0.625rem;
		border-bottom: 1px solid var(--color-border);
		white-space: nowrap;
	}

	.conn-table tr:hover td {
		background: var(--color-bg-hover);
	}

	.row-skel td {
		padding-top: 0.5rem;
		padding-bottom: 0.5rem;
	}

	.row-skel:hover td {
		background: transparent;
	}

	.cell-skel {
		display: inline-block;
		height: 0.6875rem;
		max-width: 100%;
		border-radius: 4px;
		background: var(--color-border);
		vertical-align: middle;
		animation: skel-pulse 1.1s ease-in-out infinite;
	}

	@keyframes skel-pulse {
		0%,
		100% {
			opacity: 0.38;
		}
		50% {
			opacity: 0.72;
		}
	}

	.mono {
		font-family: var(--font-mono);
		font-size: 0.6875rem;
	}

	.row-tunneled {
		background: var(--color-tunneled-row);
	}

	.client-name {
		font-size: 0.625rem;
		color: var(--color-text-muted);
		display: block;
		margin-top: 1px;
		font-family: inherit;
	}

	.pagination {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-top: 0.75rem;
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}

	.pagination-btns {
		display: flex;
		gap: 0.375rem;
	}

	.rule-badges {
		display: flex;
		flex-wrap: wrap;
		gap: 0.25rem;
		margin-top: 0.25rem;
	}

	.sortable {
		cursor: pointer;
		user-select: none;
	}

	.sortable:hover,
	.sortable.active {
		color: var(--color-accent);
	}

	.sort-arrow {
		font-size: 0.6rem;
		margin-left: 0.25rem;
		vertical-align: middle;
	}

	@media (max-width: 767px) {
		.desktop-table {
			display: none;
		}

		.mobile-sortbar {
			display: flex;
			flex-direction: column;
			gap: 0.375rem;
			margin-bottom: 0.625rem;
		}

		.mobile-sort-label {
			font-size: 0.6875rem;
			color: var(--color-text-muted);
		}

		.mobile-sort-chips {
			display: flex;
			flex-wrap: wrap;
			gap: 0.25rem;
		}

		.mobile-sort-chip {
			font-size: 0.6875rem;
		}

		.mobile-connections {
			display: grid;
			gap: 0.5rem;
		}

		.conn-card {
			border: 1px solid var(--color-border);
			border-radius: 8px;
			background: var(--color-bg-secondary);
			padding: 0.625rem;
		}

		.conn-card.row-tunneled {
			background: var(--color-tunneled-row);
		}

		.conn-card-head {
			display: flex;
			align-items: center;
			justify-content: space-between;
			gap: 0.5rem;
			margin-bottom: 0.5rem;
		}

		.conn-card-row {
			display: grid;
			grid-template-columns: 5.25rem minmax(0, 1fr);
			gap: 0.5rem;
			align-items: start;
			margin-top: 0.375rem;
		}

		.conn-card-label {
			font-size: 0.625rem;
			color: var(--color-text-muted);
		}

		.conn-card-value {
			min-width: 0;
			overflow-wrap: anywhere;
		}

		.conn-card-meta {
			margin-top: 0.5rem;
			padding-top: 0.5rem;
			border-top: 1px solid var(--color-border);
			display: grid;
			gap: 0.375rem;
		}

		.compact-row {
			margin-top: 0;
			align-items: center;
		}

		.rule-badges {
			white-space: normal;
		}

		.pagination {
			gap: 0.5rem;
		}
	}
</style>
