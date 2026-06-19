<script lang="ts">
	import { STATUS_LABEL, type PeerRowProps } from '$lib/utils/peerRowVM';
	import { Toggle } from '$lib/components/ui';
	import { Download, SquarePen, Trash2 } from 'lucide-svelte';

	let { peer, vm, showToggle, showDownload, showActions, toggling, onToggle, onConf, onEdit, onDelete, onCopy }: PeerRowProps = $props();

	function isInsideInlineToggle(event: Event): boolean {
		return event.target instanceof HTMLElement && !!event.target.closest('.peer-inline-toggle');
	}

	function toggleFromNameCell(event: Event): void {
		if (!showToggle || toggling || isInsideInlineToggle(event)) return;
		onToggle(peer);
	}

	function keydownNameCell(event: KeyboardEvent): void {
		if (!showToggle || toggling || isInsideInlineToggle(event)) return;
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			onToggle(peer);
		}
	}
</script>

<tr class="peer-row" class:peer-disabled={!vm.enabled}>
	<td
		class="col-name peer-name-cell"
		class:peer-name-cell-readonly={!showToggle}
		role={showToggle ? 'button' : undefined}
		tabindex={showToggle ? 0 : undefined}
		onclick={toggleFromNameCell}
		onkeydown={keydownNameCell}
	>
		<div class="name-cell">
			{#if showToggle}
				<span class="peer-inline-toggle">
					<Toggle checked={vm.enabled} onchange={() => onToggle(peer)} disabled={toggling} size="sm" spinner="none" />
				</span>
			{/if}
			<div class="peer-name-block">
				<span class="peer-name">{vm.name}</span>
				<span class="peer-status status-{vm.status}">
					<span class="status-dot" class:dot-online={vm.status === 'online'} class:dot-offline={vm.status === 'offline'} class:dot-disabled={vm.status === 'disabled'}></span>
					{STATUS_LABEL[vm.status]}
				</span>
				<span class="peer-handshake-sub mono tech-value">
					{#if vm.handshake}{vm.handshake.main}{:else}-{/if}
				</span>
			</div>
		</div>
	</td>
	<td class="col-ip">
		<button type="button" class="cell-copy mono tech-value" onclick={() => onCopy(vm.ip, 'IP')} title={`Скопировать IP ${vm.ip}`}>
			{vm.ip}
		</button>
	</td>
	<td class="col-endpoint">
		<button
			type="button"
			class="cell-copy endpoint-copy mono tech-value"
			onclick={() => onCopy(vm.endpoint, 'Endpoint')}
			title={vm.endpoint !== '—' ? `Скопировать Endpoint ${vm.endpoint}` : 'Endpoint отсутствует'}
		>
			<span class="endpoint-text">{vm.endpointHost}</span>
			{#if vm.endpointPort}
				<span class="endpoint-port">{vm.endpointPort}</span>
			{/if}
		</button>
	</td>
	<td class="col-traffic">
		<div class="traffic-cell mono tech-value">
			<span class="traffic-rx">RX: {vm.rx}</span>
			<span class="traffic-tx">TX: {vm.tx}</span>
		</div>
	</td>
	{#if showDownload || showActions}
		<td class="col-actions">
			<div class="peer-actions">
				{#if showDownload}
					<button class="peer-action-btn" onclick={() => onConf(peer)} title={`Скачать .conf для «${vm.name}»`}>
						<Download size={18} strokeWidth={2} aria-hidden="true" />
					</button>
				{/if}
				{#if showActions}
					<button class="peer-action-btn" onclick={() => onEdit(peer)} title={`Редактировать «${vm.name}»`}>
						<SquarePen size={18} strokeWidth={2} aria-hidden="true" />
					</button>
					<button class="peer-action-btn peer-action-btn-danger" onclick={() => onDelete(peer)} title={`Удалить «${vm.name}»`}>
						<Trash2 size={18} strokeWidth={2} aria-hidden="true" />
					</button>
				{/if}
			</div>
		</td>
	{/if}
</tr>

<style>
	.name-cell {
		display: flex;
		align-items: flex-start;
		gap: 0.5rem;
		min-width: 0;
	}
	.peer-name-block {
		display: flex;
		flex-direction: column;
		gap: 0.125rem;
		min-width: 0;
	}
	.peer-name {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.peer-handshake-sub {
		color: var(--color-text-muted);
		font-size: 0.6875rem;
		line-height: 1.2;
	}
	.peer-status {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		font-size: 0.6875rem;
		letter-spacing: 0.04em;
	}
	.status-online { color: var(--color-success); }
	.status-offline { color: var(--color-text-muted); }
	.status-disabled { color: var(--color-text-muted); }
	.status-dot { width: 6px; height: 6px; border-radius: 50%; flex-shrink: 0; }
	.dot-online { background: var(--color-success); }
	.dot-offline { background: var(--color-text-muted); }
	.dot-disabled { background: var(--color-border); }
	.endpoint-copy {
		display: inline-flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 0.125rem;
		max-width: 100%;
	}
	.endpoint-text,
	.endpoint-port {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.endpoint-port {
		font-size: 0.6875rem;
		color: var(--color-text-muted);
	}
	.traffic-cell {
		display: inline-flex;
		flex-direction: column;
		align-items: center;
		gap: 0.125rem;
	}
	.peer-actions { display: flex; gap: 0.25rem; justify-content: flex-end; }
</style>
