<script lang="ts">
	import type { ManagedPeer, ManagedPeerStats } from '$lib/types';
	import { ConfirmModal } from '$lib/components/ui';
	import { notifications } from '$lib/stores/notifications';
	import { copyToClipboard } from '$lib/utils/clipboard';
	import { peerSort } from '$lib/stores/peerSort';
	import { peerAriaSort } from '$lib/utils/peerSort';
	import { buildPeerRowVM } from '$lib/utils/peerRowVM';
	import PeerTableSortHeader from './PeerTableSortHeader.svelte';
	import ManagedPeerRow from './ManagedPeerRow.svelte';
	import ManagedPeerCard from './ManagedPeerCard.svelte';

	interface Props {
		peers: ManagedPeer[];
		getPeerStats: (publicKey: string) => ManagedPeerStats | undefined;
		showPeerActions?: boolean;
		showPeerDownload?: boolean;
		showPeerToggle?: boolean;
		onTogglePeer: (peer: ManagedPeer) => void;
		onOpenConf: (peer: ManagedPeer) => void;
		onOpenEditPeer: (peer: ManagedPeer) => void;
		onDeletePeer: (peer: ManagedPeer) => void | Promise<void>;
		isPeerToggling?: (publicKey: string) => boolean;
	}

	let {
		peers,
		getPeerStats,
		showPeerActions = true,
		showPeerDownload = true,
		showPeerToggle = true,
		onTogglePeer,
		onOpenConf,
		onOpenEditPeer,
		onDeletePeer,
		isPeerToggling = () => false,
	}: Props = $props();

	let deletePeerTarget = $state<ManagedPeer | null>(null);
	let deletingPeer = $state(false);

	let rows = $derived(peers.map((peer) => ({ peer, vm: buildPeerRowVM(peer, getPeerStats(peer.publicKey)) })));

	function requestDeletePeer(peer: ManagedPeer) {
		deletePeerTarget = peer;
	}

	async function confirmDeletePeer() {
		if (!deletePeerTarget || deletingPeer) return;
		deletingPeer = true;
		try {
			await onDeletePeer(deletePeerTarget);
			deletePeerTarget = null;
		} catch {
			// оставить модалку открытой при ошибке
		} finally {
			deletingPeer = false;
		}
	}

	async function copyCellValue(value: string, label: string): Promise<void> {
		if (!value || value === '—' || value === '-') {
			notifications.warning(`${label} отсутствует`, { duration: 2000 });
			return;
		}
		if (await copyToClipboard(value)) {
			notifications.success(`${label} скопирован: ${value}`, { duration: 2000 });
		} else {
			notifications.error(`Не удалось скопировать ${label.toLowerCase()}`);
		}
	}

	let showActionsCol = $derived(showPeerDownload || showPeerActions);
</script>

<div class="peer-views">
<div class="desktop-peer-table">
	<div class="table-wrap">
		<table class="managed-peer-table">
			<colgroup>
				<col class="col-name-col" />
				<col class="col-ip-col" />
				<col class="col-endpoint-col" />
				<col class="col-traffic-col" />
				{#if showActionsCol}
					<col class="col-actions-col" />
				{/if}
			</colgroup>
			<thead>
				<tr>
					<th class="col-name" aria-sort={peerAriaSort($peerSort, 'name')}>
						<PeerTableSortHeader label="Имя" sortKey="name" />
					</th>
					<th class="col-ip" aria-sort={peerAriaSort($peerSort, 'ip')}>
						<PeerTableSortHeader label="IP" sortKey="ip" />
					</th>
					<th class="col-endpoint" aria-sort={peerAriaSort($peerSort, 'endpoint')}>
						<PeerTableSortHeader label="Endpoint" sortKey="endpoint" />
					</th>
					<th class="col-traffic" aria-sort={peerAriaSort($peerSort, 'traffic')}>
						<PeerTableSortHeader label="Трафик" sortKey="traffic" />
					</th>
					{#if showActionsCol}
						<th class="col-actions">Действия</th>
					{/if}
				</tr>
			</thead>
			<tbody>
				{#each rows as { peer, vm } (peer.publicKey)}
					<ManagedPeerRow
						{peer}
						{vm}
						showToggle={showPeerToggle}
						showDownload={showPeerDownload}
						showActions={showPeerActions}
						toggling={isPeerToggling(peer.publicKey)}
						onToggle={onTogglePeer}
						onConf={onOpenConf}
						onEdit={onOpenEditPeer}
						onDelete={requestDeletePeer}
						onCopy={copyCellValue}
					/>
				{/each}
			</tbody>
		</table>
	</div>
</div>

<div class="mobile-peer-list">
	{#each rows as { peer, vm } (peer.publicKey)}
		<ManagedPeerCard
			{peer}
			{vm}
			showToggle={showPeerToggle}
			showDownload={showPeerDownload}
			showActions={showPeerActions}
			toggling={isPeerToggling(peer.publicKey)}
			onToggle={onTogglePeer}
			onConf={onOpenConf}
			onEdit={onOpenEditPeer}
			onDelete={requestDeletePeer}
			onCopy={copyCellValue}
		/>
	{/each}
</div>
</div>

{#if deletePeerTarget}
	<ConfirmModal
		open={true}
		title="Удаление клиента"
		message={`Удалить клиента «${deletePeerTarget.description || deletePeerTarget.publicKey.slice(0, 8) + '...'}»?`}
		secondary={`Туннельный IP: ${deletePeerTarget.tunnelIP}. Конфигурация и ключи будут удалены без возможности восстановления.`}
		confirmLabel="Удалить"
		busy={deletingPeer}
		onConfirm={confirmDeletePeer}
		onClose={() => { if (!deletingPeer) deletePeerTarget = null; }}
	/>
{/if}

<style>
	.table-wrap {
		overflow-x: auto;
	}

	.managed-peer-table {
		width: max-content;
		min-width: 100%;
		border-collapse: collapse;
		font-size: 12px;
		table-layout: auto;
	}

	.managed-peer-table th {
		text-align: center;
		background: var(--bg-tertiary, var(--color-bg-tertiary));
		color: var(--text-muted, var(--color-text-muted));
		font-weight: 600;
		padding: 0.65rem 0.75rem;
		line-height: 1.2;
		border-bottom: 1px solid var(--border, var(--color-border));
		white-space: nowrap;
	}

	.managed-peer-table :global(td) {
		padding: 0.55rem 0.5rem;
		border-bottom: 1px solid var(--border, var(--color-border));
		vertical-align: middle;
		transition: background-color 0.15s ease;
	}

	.managed-peer-table :global(tbody tr:hover td),
	.managed-peer-table :global(tbody tr:focus-within td) {
		background: color-mix(in srgb, var(--bg-hover) 70%, transparent);
	}

	.managed-peer-table :global(tbody tr.peer-disabled:hover td) {
		background: color-mix(in srgb, var(--bg-hover) 45%, transparent);
	}

	.managed-peer-table :global(.peer-disabled) {
		opacity: 0.5;
	}

	.managed-peer-table :global(.cell-copy),
	.managed-peer-table :global(.endpoint-copy) {
		background: transparent;
	}

	.managed-peer-table :global(td.peer-name-cell) {
		text-align: left;
		cursor: pointer;
	}

	.managed-peer-table :global(td.peer-name-cell:focus-visible) {
		outline: 2px solid var(--color-accent);
		outline-offset: -2px;
	}

	.managed-peer-table :global(td.peer-name-cell-readonly) {
		cursor: default;
	}

	.col-name-col { width: 1%; }
	.col-ip-col { width: 1%; }
	.col-endpoint-col { width: auto; }
	.col-traffic-col { width: 8.5rem; }
	.col-actions-col { width: 1%; }

	.managed-peer-table :global(.col-name) { width: 1%; }
	.managed-peer-table :global(.col-ip) { white-space: nowrap; }
	.managed-peer-table :global(.col-endpoint) { white-space: nowrap; }
	.managed-peer-table :global(.col-traffic) { white-space: nowrap; }
	.managed-peer-table :global(.col-actions) { white-space: nowrap; }

	.managed-peer-table :global(td.col-ip),
	.managed-peer-table :global(td.col-endpoint),
	.managed-peer-table :global(td.col-traffic),
	.managed-peer-table :global(td.col-actions) {
		text-align: center;
	}

	.peer-views {
		container-type: inline-size;
	}

	.desktop-peer-table {
		display: block;
	}

	.mobile-peer-list {
		display: none;
	}

	@container (max-width: 819px) {
		.desktop-peer-table {
			display: none;
		}

		.mobile-peer-list {
			display: flex;
			flex-direction: column;
			gap: 0.5rem;
		}
	}

	@media (max-width: 760px) {
		.desktop-peer-table {
			display: none;
		}

		.mobile-peer-list {
			display: flex;
			flex-direction: column;
			gap: 0.5rem;
		}
	}
</style>
