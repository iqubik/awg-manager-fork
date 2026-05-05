<script lang="ts">
	import type { Subscription } from '$lib/types';
	import SubscriptionCard from './SubscriptionCard.svelte';

	interface Props {
		subscriptions: Subscription[];
		onAdd: () => void;
	}
	let { subscriptions, onAdd }: Props = $props();
</script>

{#if subscriptions.length === 0}
	<div class="empty">
		<div class="ehead">Нет подписок</div>
		<div class="esub">
			Добавьте подписку — мастер скачает список серверов и создаст selector-туннель.
		</div>
		<button type="button" class="btn primary" onclick={onAdd}>
			+ Добавить подписку
		</button>
	</div>
{:else}
	<div class="list">
		{#each subscriptions as sub (sub.id)}
			<SubscriptionCard subscription={sub} />
		{/each}
	</div>
{/if}

<style>
	.empty {
		padding: 3rem 1.5rem;
		text-align: center;
		border: 1px dashed var(--color-border);
		border-radius: 6px;
	}
	.ehead {
		color: var(--color-text-primary);
		font-size: 1.1rem;
		font-weight: 600;
		margin-bottom: 0.4rem;
	}
	.esub {
		color: var(--color-text-muted);
		font-size: 0.88rem;
		margin-bottom: 1.2rem;
	}
	.btn {
		padding: 0.55rem 1.4rem;
		border-radius: 6px;
		font: inherit;
		cursor: pointer;
		border: 1px solid transparent;
	}
	.primary { color: white; background: #238636; border-color: #2ea043; }
	.list { display: flex; flex-direction: column; gap: 0.6rem; }
</style>
