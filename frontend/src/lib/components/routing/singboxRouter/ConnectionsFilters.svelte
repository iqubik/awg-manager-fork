<!-- frontend/src/lib/components/routing/singboxRouter/ConnectionsFilters.svelte -->
<script lang="ts">
	import type { ConnectionFilters, NetworkFilter } from '$lib/types/singboxConnections';

	interface Props {
		filters: ConnectionFilters;
		outboundOptions: string[];
		ruleOptions: string[];
		onChange: (next: ConnectionFilters) => void;
	}

	let { filters, outboundOptions, ruleOptions, onChange }: Props = $props();

	// svelte-ignore state_referenced_locally
	let searchValue = $state(filters.search);
	let debounceTimer: ReturnType<typeof setTimeout> | null = null;

	function commitSearch(): void {
		onChange({ ...filters, search: searchValue });
	}

	function onSearchInput(e: Event): void {
		searchValue = (e.target as HTMLInputElement).value;
		if (debounceTimer !== null) clearTimeout(debounceTimer);
		debounceTimer = setTimeout(commitSearch, 200);
	}

	function setOutbound(v: string): void { onChange({ ...filters, outbound: v }); }
	function setNetwork(v: NetworkFilter): void { onChange({ ...filters, network: v }); }
	function setRule(v: string): void { onChange({ ...filters, rule: v }); }
</script>

<div class="row">
	<input
		type="text"
		class="search"
		placeholder="Поиск host / IP / клиент"
		value={searchValue}
		oninput={onSearchInput}
	/>

	<select class="select" value={filters.outbound} onchange={(e) => setOutbound((e.target as HTMLSelectElement).value)}>
		<option value="">Outbound: все</option>
		{#each outboundOptions as o}
			<option value={o}>{o}</option>
		{/each}
	</select>

	<select class="select" value={filters.network} onchange={(e) => setNetwork((e.target as HTMLSelectElement).value as NetworkFilter)}>
		<option value="all">Network: все</option>
		<option value="tcp">TCP</option>
		<option value="udp">UDP</option>
	</select>

	<select class="select" value={filters.rule} onchange={(e) => setRule((e.target as HTMLSelectElement).value)}>
		<option value="">Rule: все</option>
		{#each ruleOptions as r}
			<option value={r}>{r}</option>
		{/each}
	</select>
</div>

<style>
	.row {
		display: flex; gap: 8px; flex-wrap: wrap;
		margin-bottom: 12px;
	}
	.search {
		flex: 1; min-width: 220px;
		padding: 6px 10px;
		font-size: 13px;
		background: var(--surface-1, #1f2425);
		border: 1px solid var(--border-1, #2c3134);
		border-radius: 6px;
		color: var(--text-primary, #e8e6e3);
	}
	.select {
		padding: 6px 10px;
		font-size: 13px;
		background: var(--surface-1, #1f2425);
		border: 1px solid var(--border-1, #2c3134);
		border-radius: 6px;
		color: var(--text-primary, #e8e6e3);
	}
</style>
