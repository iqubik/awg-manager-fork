<script lang="ts">
	import type { Subscription, SubscriptionMember } from '$lib/types';
	import { api } from '$lib/api/client';
	import { Button } from '$lib/components/ui';
	import SubscriptionMemberCard from './SubscriptionMemberCard.svelte';

	interface Props {
		subscription: Subscription;
		onUpdated: () => void;
	}
	let { subscription, onUpdated }: Props = $props();

	let refreshing = $state(false);
	let switching = $state<string | null>(null);
	let lastError = $state('');

	// Derive member list from members[] when available; fall back to stubs
	// built from memberTags[] for subscriptions persisted before this change.
	const memberList = $derived<SubscriptionMember[]>(
		subscription.members && subscription.members.length > 0
			? subscription.members
			: subscription.memberTags.map((tag) => ({
					tag,
					protocol: '?',
					server: tag,
					port: 0,
			  })),
	);

	async function refresh(): Promise<void> {
		refreshing = true;
		lastError = '';
		try {
			await api.refreshSubscription(subscription.id);
			onUpdated();
		} catch (e) {
			lastError = e instanceof Error ? e.message : 'Не удалось обновить';
		} finally {
			refreshing = false;
		}
	}

	async function pickActive(memberTag: string): Promise<void> {
		if (memberTag === subscription.activeMember) return;
		switching = memberTag;
		lastError = '';
		try {
			await api.setSubscriptionActiveMember(subscription.id, memberTag);
			onUpdated();
		} catch (e) {
			lastError = e instanceof Error ? e.message : 'Не удалось переключить';
		} finally {
			switching = null;
		}
	}
</script>

<header class="head">
	<div class="head-info">
		<div class="lbl">Selector</div>
		<div class="val mono">{subscription.selectorTag}</div>
	</div>
	<Button variant="primary" size="sm" disabled={refreshing} loading={refreshing} onclick={refresh}>
		{refreshing ? 'Обновляем...' : 'Обновить сейчас'}
	</Button>
</header>

{#if lastError}
	<div class="err">{lastError}</div>
{/if}

{#if memberList.length === 0}
	<div class="empty">Подписка ещё не загружена. Нажмите «Обновить сейчас».</div>
{:else}
	<div class="hint">Выберите активный сервер. Selector направит трафик в выбранный outbound.</div>
	<div class="grid">
		{#each memberList as member (member.tag)}
			<SubscriptionMemberCard
				{member}
				active={member.tag === subscription.activeMember}
				switching={switching === member.tag}
				disabled={switching !== null}
				onclick={() => pickActive(member.tag)}
			/>
		{/each}
	</div>
{/if}

{#if subscription.orphanTags.length > 0}
	<section class="orphans">
		<div class="lbl warn">Orphan ({subscription.orphanTags.length})</div>
		<div class="hint">
			Серверы из прошлой версии подписки, не вернувшиеся при последнем refresh.
			Удалить можно из настроек.
		</div>
		<div class="grid">
			{#each subscription.orphanTags as tag (tag)}
				<div class="orphan-card mono">{tag}</div>
			{/each}
		</div>
	</section>
{/if}

<style>
	.head {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 1rem;
		margin-bottom: 1rem;
	}
	.head-info { display: flex; flex-direction: column; gap: 0.2rem; }
	.lbl {
		font-size: 0.7rem;
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.5px;
	}
	.lbl.warn { color: #d29922; }
	.val { color: var(--color-text-primary); font-size: 0.85rem; }
	.err { color: #f85149; font-size: 0.85rem; margin-bottom: 0.6rem; }
	.hint { color: var(--color-text-muted); font-size: 0.82rem; margin-bottom: 0.8rem; }
	.empty {
		padding: 2rem;
		text-align: center;
		color: var(--color-text-muted);
		border: 1px dashed var(--color-border);
		border-radius: 6px;
	}
	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
		gap: 0.7rem;
	}
	.orphans {
		margin-top: 1.5rem;
		padding-top: 1rem;
		border-top: 1px solid var(--color-border);
	}
	.orphan-card {
		padding: 14px 16px;
		border: 1px dashed var(--color-border);
		border-radius: 10px;
		font-size: 0.8rem;
		color: var(--color-text-muted);
	}
	.mono { font-family: var(--font-mono, ui-monospace, monospace); }
</style>
