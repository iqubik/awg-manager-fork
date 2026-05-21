<script lang="ts">
	import { Button, Modal, Toggle } from '$lib/components/ui';
	import { api } from '$lib/api/client';
	import { notifications } from '$lib/stores/notifications';
	import type { ManagedServerBackupFile, RestoreOutcome } from '$lib/types';

	interface Props {
		open: boolean;
		file: ManagedServerBackupFile;
		onclose: () => void;
	}
	let { open = $bindable(false), file, onclose }: Props = $props();

	let allowRenumber = $state(false);
	let importing = $state(false);
	let outcomes = $state<RestoreOutcome[]>([]);

	const peerCount = $derived(file.managedServers.reduce((acc, s) => acc + (s.peers?.length ?? 0), 0));

	async function runImport() {
		importing = true;
		try {
			const resp = await api.managedServerImport({
				...file,
				options: { allowRenumber },
			});
			outcomes = resp.outcomes;
		} catch (e) {
			notifications.error((e as Error).message);
		} finally {
			importing = false;
		}
	}

	function actionStyle(action: RestoreOutcome['action']): string {
		switch (action) {
			case 'created':
			case 'renamed':
				return 'ok';
			case 'merged':
				return 'info';
			case 'conflict':
				return 'warn';
			case 'failed':
				return 'err';
			default:
				return 'muted';
		}
	}
</script>

<Modal {open} title="Импорт резервной копии" size="md" {onclose}>
	{#if outcomes.length === 0}
		<p>Файл содержит {file.managedServers.length} сервер(а/ов), {peerCount} пир(а/ов).</p>
		<Toggle
			checked={allowRenumber}
			onchange={(v) => (allowRenumber = v)}
			label="Если слот занят другим сервером — переименовать на свободный Wireguard<N>"
		/>
	{:else}
		<div class="results">
			{#each outcomes as o, i (`${o.name}:${i}`)}
				<div class="row {actionStyle(o.action)}">
					<span class="iface">
						{o.name}{#if o.newName} → {o.newName}{/if}
					</span>
					<span class="action">
						{o.action}{#if o.addedPeers !== undefined && o.addedPeers > 0} (+{o.addedPeers} peers){/if}
					</span>
					{#if o.conflicts && o.conflicts.length}
						<ul class="reasons">
							{#each o.conflicts as c}<li>{c}</li>{/each}
						</ul>
					{/if}
					{#if o.error}
						<div class="reasons">{o.error}</div>
					{/if}
				</div>
			{/each}
		</div>
	{/if}

	{#snippet actions()}
		{#if outcomes.length === 0}
			<Button variant="secondary" size="md" onclick={onclose}>Отмена</Button>
			<Button variant="outline-primary" size="md" onclick={runImport} loading={importing}>Импортировать</Button>
		{:else}
			<Button variant="secondary" size="md" onclick={onclose}>Закрыть</Button>
		{/if}
	{/snippet}
</Modal>

<style>
	.results {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.row {
		padding: 0.5rem;
		border-radius: 6px;
		background: var(--color-bg-tertiary);
	}
	.row.ok    { border-left: 3px solid var(--color-success); }
	.row.info  { border-left: 3px solid var(--color-accent); }
	.row.warn  { border-left: 3px solid var(--color-warning); }
	.row.err   { border-left: 3px solid var(--color-error); }
	.row.muted { border-left: 3px solid var(--color-text-muted); }
	.iface {
		font-family: var(--font-mono);
		margin-right: 0.5rem;
	}
	.action {
		font-size: 0.875rem;
		color: var(--color-text-secondary);
	}
	.reasons {
		margin: 0.5rem 0 0;
		padding-left: 1rem;
		font-size: 0.8125rem;
		color: var(--color-text-muted);
	}
</style>
