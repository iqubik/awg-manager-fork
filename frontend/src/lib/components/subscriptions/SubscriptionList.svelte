<script lang="ts">
	import type { Subscription } from '$lib/types';
	import SubscriptionCard from './SubscriptionCard.svelte';
	import { Button } from '$lib/components/ui';
	import CreateIcon from '$lib/components/ui/icons/CreateIcon.svelte';

	interface Props {
		subscriptions: Subscription[];
		onAdd: () => void;
		ondelete?: (id: string) => void;
	}
	let { subscriptions, onAdd, ondelete }: Props = $props();
</script>

{#snippet createIcon()}
	<CreateIcon />
{/snippet}

{#if subscriptions.length === 0}
	<div class="empty">
		<div class="ehead">Нет подписок</div>
		<div class="esub">
			Добавьте подписку — мастер скачает список серверов и создаст selector-туннель.
		</div>
		<Button variant="primary" size="md" onclick={onAdd} iconBefore={createIcon}>
			Добавить подписку
		</Button>
	</div>
{:else}
	<div class="list">
		{#each subscriptions as sub (sub.id)}
			<SubscriptionCard subscription={sub} {ondelete} />
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
	.list { display: flex; flex-direction: column; gap: 0.6rem; }
</style>
