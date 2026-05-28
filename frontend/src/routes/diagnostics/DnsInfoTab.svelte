<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import type { DnsProxyInfo } from '$lib/types';
	import { Button, Card } from '$lib/components/ui';
	import { EmptyState } from '$lib/components/layout';
	import {
		UpstreamsTable,
		PolicyStatRow,
		StaticRecordsCard,
		RebindCard,
	} from '$lib/components/diagnostics';

	let info = $state<DnsProxyInfo | null>(null);
	let loading = $state(false);
	let errored = $state(false);

	// Upstreams/static/rebind are router-wide; show the first proxy's copy once.
	const shared = $derived(info?.proxies?.[0] ?? null);

	async function load() {
		loading = true;
		errored = false;
		try {
			info = await api.getDnsProxyInfo();
		} catch {
			errored = true;
		} finally {
			loading = false;
		}
	}

	onMount(load);
</script>

{#snippet refreshIcon()}
	<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
		<path d="M21 12a9 9 0 1 1-2.64-6.36M21 4v6h-6" stroke-linecap="round" stroke-linejoin="round" />
	</svg>
{/snippet}

<div class="toolbar">
	<Button variant="secondary" size="sm" onclick={load} loading={loading} iconBefore={refreshIcon}>Обновить</Button>
</div>

{#if loading && !info}
	<p class="hint">Загрузка сведений о DNS…</p>
{:else if errored}
	<p class="hint warn">Не удалось загрузить сведения о DNS.</p>
{:else if info && info.proxies.length > 0}
	<div class="dns-sections">
		{#if shared}
			<Card>
				<div class="card-label">Апстрим-серверы <span class="hint-inline">общие для роутера</span></div>
				<UpstreamsTable upstreams={shared.upstreams} />
			</Card>
		{/if}

		<Card>
			<div class="card-label">Статистика по политикам</div>
			{#each info.proxies as p, i}
				<PolicyStatRow proxy={p} open={i === 0} />
			{/each}
		</Card>

		{#if shared}
			<Card><StaticRecordsCard records={shared.staticRecords} /></Card>
			<Card><RebindCard rebind={shared.rebind} /></Card>
		{/if}
	</div>
{:else}
	<EmptyState title="Нет данных DNS-прокси" />
{/if}

<style>
	.toolbar {
		display: flex;
		gap: 0.5rem;
		margin-bottom: 0.75rem;
	}
	.hint { font-size: 0.8125rem; color: var(--text-muted); margin: 0 0 0.75rem; }
	.warn { color: var(--warning); }
	.dns-sections { display: flex; flex-direction: column; gap: 16px; }
	.card-label { font-size: 11px; font-weight: 700; letter-spacing: .06em; text-transform: uppercase; color: var(--text-muted); margin-bottom: 12px; }
	.hint-inline { font-size: 11px; font-weight: 400; text-transform: none; letter-spacing: 0; margin-left: 8px; opacity: .8; }

	@media (max-width: 640px) {
		.toolbar {
			width: 100%;
		}

		.toolbar :global(.btn) {
			width: 100%;
			justify-content: center;
		}
	}
</style>
