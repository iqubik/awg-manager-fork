<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { Tabs, Button } from '$lib/components/ui';
	import { singboxStatus } from '$lib/stores/singbox';
	import JsonConfigDrawer from './JsonConfigDrawer.svelte';

	type SubTab = 'engine' | 'rules' | 'rulesets' | 'dns' | 'deviceproxy';

	const order: SubTab[] = ['engine', 'rules', 'rulesets', 'dns', 'deviceproxy'];

	const labels: Record<SubTab, string> = {
		engine: 'Движок',
		rules: 'Правила',
		rulesets: 'Наборы',
		dns: 'DNS',
		deviceproxy: 'Прокси'
	};

	let active = $state<SubTab>('engine');
	let drawerOpen = $state(false);

	function readSubFromURL(): SubTab {
		const v = $page.url.searchParams.get('sub');
		return order.includes(v as SubTab) ? (v as SubTab) : 'engine';
	}

	function setSub(next: SubTab) {
		if (next === active) return;
		active = next;
		const sp = new URLSearchParams($page.url.search);
		sp.set('sub', next);
		sp.set('tab', 'singbox');
		goto(`?${sp.toString()}`, { replaceState: true, keepFocus: true, noScroll: true });
	}

	// Subscribe to the cold-tier sing-box status polling store so the
	// header badge reflects real running/version state. The store is
	// shared with the rest of the app — subscribing here just keeps it
	// hot while this page is open.
	let unsubStatus: (() => void) | undefined;
	onMount(() => {
		unsubStatus = singboxStatus.subscribe(() => {});
	});
	onDestroy(() => {
		unsubStatus?.();
	});

	$effect(() => {
		active = readSubFromURL();
	});

	const status = $derived($singboxStatus.data);
	const running = $derived(status?.running ?? false);
	const version = $derived(status?.version ?? '—');
	const tabsItems = $derived(order.map((id) => ({ id, label: labels[id] })));
</script>

<header class="page-header">
	<div class="header-right">
		<span class="status-badge" class:running>
			<span class="status-dot"></span>
			sing-box · {running ? `v${version}` : 'остановлен'}
		</span>
		<Button size="sm" variant="ghost" onclick={() => (drawerOpen = true)}>Конфиг</Button>
	</div>
</header>

<Tabs tabs={tabsItems} active={active} onchange={(id) => setSub(id as SubTab)} />

<section class="sub-content">
	{#if active === 'engine'}
		<div class="placeholder">Движок — будет в следующей задаче</div>
	{:else if active === 'rules'}
		<div class="placeholder">Правила — будут в следующей задаче</div>
	{:else if active === 'rulesets'}
		<div class="placeholder">Наборы — будут в следующей задаче</div>
	{:else if active === 'dns'}
		<div class="placeholder">DNS — будет в следующей задаче</div>
	{:else if active === 'deviceproxy'}
		<div class="placeholder">Прокси — будет в следующей задаче</div>
	{/if}
</section>

<JsonConfigDrawer open={drawerOpen} onClose={() => (drawerOpen = false)} />

<style>
	.page-header {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		gap: 0.75rem;
		margin-bottom: 0.75rem;
	}
	.header-right {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}
	.status-badge {
		display: inline-flex;
		align-items: center;
		gap: 0.4rem;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', monospace;
		font-size: 12px;
		color: var(--color-text-secondary);
	}
	.status-dot {
		width: 7px;
		height: 7px;
		border-radius: 999px;
		background: var(--color-error);
		box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-error) 22%, transparent);
	}
	.status-badge.running .status-dot {
		background: var(--color-success);
		box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-success) 28%, transparent);
	}
	.sub-content {
		margin-top: 1rem;
	}
	.placeholder {
		padding: 2rem;
		text-align: center;
		color: var(--color-text-secondary);
		border: 1px dashed var(--color-border);
		border-radius: var(--radius-sm);
	}
</style>
