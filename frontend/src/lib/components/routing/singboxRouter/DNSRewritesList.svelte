<script lang="ts">
	import { api } from '$lib/api/client';
	import { notifications } from '$lib/stores/notifications';
	import { Button, ConfirmModal } from '$lib/components/ui';
	import { Trash2 } from 'lucide-svelte';
	import CreateIcon from '$lib/components/ui/icons/CreateIcon.svelte';
	import type { SingboxRouterDNSRewrite } from '$lib/types';
	import DNSRewriteEditModal from './DNSRewriteEditModal.svelte';

	interface Props {
		rewrites: SingboxRouterDNSRewrite[];
		onChange: () => Promise<void> | void;
		/** Показывать встроенный заголовок (счётчик + кнопка «Добавить»). */
		showHeader?: boolean;
		/** Режим добавления — bindable, чтобы триггерить add из родителя. */
		addMode?: boolean;
	}
	let { rewrites, onChange, showHeader = true, addMode = $bindable(false) }: Props = $props();

	let editIndex = $state<number | null>(null);
	let deleteIndex = $state<number | null>(null);
	let deleteBusy = $state(false);

	function requestDelete(i: number): void {
		deleteIndex = i;
	}

	async function confirmDelete(): Promise<void> {
		if (deleteIndex === null) return;
		deleteBusy = true;
		try {
			await api.singboxRouterDeleteDNSRewrite(deleteIndex);
			deleteIndex = null;
			await onChange();
		} catch (e) {
			notifications.error((e as Error).message);
		} finally {
			deleteBusy = false;
		}
	}
</script>

{#snippet createIcon()}
	<CreateIcon />
{/snippet}

{#if showHeader}
	<div class="header">
		<div class="hint">{rewrites.length} перезаписей</div>
		<Button variant="primary" size="sm" onclick={() => (addMode = true)} iconBefore={createIcon}>
			Добавить
		</Button>
	</div>
{/if}

{#if rewrites.length === 0}
	<div class="empty-mild">
		Перезаписей нет. «Перезапись» возвращает заданный IP для домена/паттерна.
	</div>
{:else}
	<div class="col-header">
		<div>Шаблон</div>
		<div></div>
		<div>IP-адреса</div>
		<div></div>
		<div></div>
	</div>
	<div class="rows">
		{#each rewrites as rw, i (i)}
			<div class="row">
				<code class="pat mono">{rw.pattern}</code>
				<span class="arrow">→</span>
				<span class="ips mono">{rw.ips.join(', ')}</span>
				<button class="icon-btn" onclick={() => (editIndex = i)} aria-label="Редактировать">✎</button>
				<button class="icon-btn danger" onclick={() => requestDelete(i)} aria-label="Удалить" title="Удалить"><Trash2 size={14} /></button>
			</div>
		{/each}
	</div>
{/if}

{#if addMode}
	<DNSRewriteEditModal
		onClose={() => (addMode = false)}
		onSave={async (rw) => {
			await api.singboxRouterAddDNSRewrite(rw);
			addMode = false;
			await onChange();
		}}
	/>
{/if}

{#if editIndex !== null}
	{@const idx = editIndex}
	<DNSRewriteEditModal
		rewrite={rewrites[idx]}
		onClose={() => (editIndex = null)}
		onSave={async (rw) => {
			await api.singboxRouterUpdateDNSRewrite(idx, rw);
			editIndex = null;
			await onChange();
		}}
	/>
{/if}

<ConfirmModal
	open={deleteIndex !== null}
	title="Удалить перезапись"
	message={deleteIndex !== null ? `Удалить перезапись «${rewrites[deleteIndex]?.pattern ?? ''}»?` : ''}
	busy={deleteBusy}
	onConfirm={confirmDelete}
	onClose={() => { if (!deleteBusy) deleteIndex = null; }}
/>

<style>
	.header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 0.75rem;
	}
	.hint {
		color: var(--muted-text);
		font-size: 0.85rem;
	}
	.empty-mild {
		padding: 0.6rem 0.9rem;
		background: var(--surface-bg);
		border-radius: 4px;
		color: var(--muted-text);
		font-size: 0.85rem;
	}
	.col-header {
		display: grid;
		grid-template-columns: 1fr 16px 1fr 24px 24px;
		gap: 0.4rem;
		padding: 0.25rem 0.75rem;
		font-size: 0.65rem;
		letter-spacing: 0.5px;
		text-transform: uppercase;
		color: var(--muted-text);
	}
	.rows {
		display: grid;
		gap: 0.2rem;
	}
	.row {
		display: grid;
		grid-template-columns: 1fr 16px 1fr 24px 24px;
		gap: 0.4rem;
		align-items: center;
		background: var(--surface-bg);
		padding: 0.5rem 0.75rem;
		border-radius: 4px;
	}
	.mono {
		font-family: ui-monospace, monospace;
		font-size: 0.8rem;
	}
	.pat {
		color: var(--text);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.arrow {
		opacity: 0.5;
		text-align: center;
	}
	.ips {
		color: var(--success, #22c55e);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.icon-btn {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		background: transparent;
		border: none;
		color: var(--muted-text);
		cursor: pointer;
		font-size: 0.9rem;
		padding: 0.15rem;
		border-radius: 4px;
	}
	.icon-btn:hover {
		color: var(--text);
		background: var(--surface-bg);
	}
	.icon-btn.danger:hover {
		color: var(--danger, #dc2626);
	}
</style>
