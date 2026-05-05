<script lang="ts">
	import type { Subscription } from '$lib/types';
	import { goto } from '$app/navigation';

	interface Props {
		subscription: Subscription;
	}
	let { subscription }: Props = $props();

	function open(): void {
		goto(`/subscriptions/${subscription.id}`);
	}

	const status = $derived(
		subscription.lastError ? 'error' : subscription.lastFetched ? 'ok' : 'pending',
	);
	const lastFetchedHuman = $derived(
		subscription.lastFetched ? formatRelative(subscription.lastFetched) : '—',
	);

	function formatRelative(iso: string): string {
		const d = new Date(iso);
		const diff = Date.now() - d.getTime();
		const hours = Math.floor(diff / 3_600_000);
		if (hours < 1) return 'только что';
		if (hours < 24) return `${hours}ч назад`;
		return `${Math.floor(hours / 24)}д назад`;
	}
</script>

<button type="button" class="card" class:err={status === 'error'} onclick={open}>
	<div class="head">
		<div class="label">{subscription.label || subscription.url}</div>
		<div class="badge {status}">
			{#if status === 'ok'}OK{:else if status === 'error'}Ошибка{:else}—{/if}
		</div>
	</div>
	<div class="meta mono">{subscription.inboundTag} · :{subscription.listenPort}</div>
	<div class="info">
		{subscription.memberTags.length} серверов
		{#if subscription.activeMember}· активен <span class="mono">{subscription.activeMember}</span>{/if}
		· обновлено {lastFetchedHuman}
		{#if subscription.refreshHours > 0}· auto {subscription.refreshHours}ч{/if}
	</div>
	{#if subscription.lastError}
		<div class="err-msg mono">{subscription.lastError}</div>
	{/if}
</button>

<style>
	.card {
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
		padding: 0.85rem 1rem;
		background: var(--color-bg-secondary);
		border: 1px solid var(--color-border);
		border-radius: 6px;
		font: inherit;
		text-align: left;
		color: var(--color-text-primary);
		cursor: pointer;
	}
	.card.err { border-color: #f85149; }
	.head { display: flex; justify-content: space-between; align-items: center; }
	.label { font-weight: 600; font-size: 0.95rem; }
	.badge { font-size: 0.72rem; padding: 0.15rem 0.5rem; border-radius: 999px; }
	.badge.ok { background: rgba(63, 185, 80, 0.15); color: #3fb950; }
	.badge.error { background: rgba(248, 81, 73, 0.15); color: #f85149; }
	.badge.pending { background: var(--color-bg-tertiary); color: var(--color-text-muted); }
	.meta { font-size: 0.75rem; color: var(--color-text-muted); }
	.info { font-size: 0.82rem; color: var(--color-text-muted); }
	.err-msg { font-size: 0.78rem; color: #f85149; margin-top: 0.3rem; }
	.mono { font-family: var(--font-mono, ui-monospace, monospace); }
</style>
