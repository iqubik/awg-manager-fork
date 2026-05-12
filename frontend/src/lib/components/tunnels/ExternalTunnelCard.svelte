<!-- frontend/src/lib/components/ExternalTunnelCard.svelte -->
<script lang="ts">
	import type { ExternalTunnel } from '$lib/types';
	import { formatBytes } from '$lib/utils/format';
	import { Button } from '$lib/components/ui';

	interface Props {
		tunnel: ExternalTunnel;
		view?: 'cards' | 'compact' | 'list';
		onadopt?: (interfaceName: string) => void;
	}

	let { tunnel, view = 'cards', onadopt }: Props = $props();

	function handleAdopt(): void {
		onadopt?.(tunnel.interfaceName);
	}
</script>

{#if view === 'list'}
	<div class="card ext-card list-card">
		<div class="list-cell list-cell-primary">
			<h3 class="tunnel-name">{tunnel.interfaceName}</h3>
			<div class="flex items-center gap-2 flex-wrap">
				<span class="iface-name">WG туннель</span>
				<span class="version-badge badge-external">Внешний</span>
			</div>
			<div class="list-note">Наружный интерфейс WireGuard</div>
		</div>

		<div class="list-cell list-cell-status">
			<span class="list-label">Статус</span>
			{#if tunnel.lastHandshake}
				<span class="status-badge status-active">
					<span class="led-dot"></span>
					Подключён
				</span>
			{:else}
				<span class="status-badge status-inactive">
					<span class="led-dot"></span>
					Неактивен
				</span>
			{/if}
		</div>

		<div class="list-cell list-cell-endpoint">
			<span class="list-label">Endpoint</span>
			<span class="detail-value truncate">{tunnel.endpoint || '—'}</span>
		</div>

		<div class="list-cell list-cell-traffic">
			<span class="list-label">Трафик</span>
			<div class="list-note">↓ {formatBytes(tunnel.rxBytes)} · ↑ {formatBytes(tunnel.txBytes)}</div>
		</div>

		<div class="list-cell list-cell-stats">
			<span class="list-label">Handshake</span>
			<div class="list-note">{tunnel.lastHandshake || '—'}</div>
		</div>

		<div class="list-cell list-cell-actions">
			<Button variant="primary" size="sm" onclick={handleAdopt}>
				Взять под управление
			</Button>
		</div>
	</div>
{:else}
	<div
		class="card ext-card flex flex-col gap-4"
		class:view-compact={view === 'compact'}
	>
		<div class="header flex justify-between items-start gap-3">
			<div class="flex flex-col gap-1 min-w-0">
				<h3 class="tunnel-name">{tunnel.interfaceName}</h3>
				<div class="flex items-center gap-2 flex-wrap">
					<span class="iface-name">WG туннель</span>
					<span class="version-badge badge-external">Внешний</span>
				</div>
			</div>
			<div class="shrink-0">
				{#if tunnel.lastHandshake}
					<span class="status-badge status-active">
						<span class="led-dot"></span>
						Подключён
					</span>
				{:else}
					<span class="status-badge status-inactive">
						<span class="led-dot"></span>
						Неактивен
					</span>
				{/if}
			</div>
		</div>

		<div class="details">
			{#if tunnel.endpoint}
				<div class="flex flex-col gap-0.5 min-w-0">
					<span class="detail-label">Endpoint</span>
					<span class="detail-value">{tunnel.endpoint}</span>
				</div>
			{/if}
			{#if tunnel.lastHandshake}
				<div class="flex flex-col gap-0.5 min-w-0">
					<span class="detail-label">Handshake</span>
					<span class="detail-value">{tunnel.lastHandshake}</span>
				</div>
			{/if}
			<div class="flex gap-6">
				<div class="flex flex-col gap-0.5 min-w-0">
					<span class="detail-label">RX</span>
					<span class="detail-value">{formatBytes(tunnel.rxBytes)}</span>
				</div>
				<div class="flex flex-col gap-0.5 min-w-0">
					<span class="detail-label">TX</span>
					<span class="detail-value">{formatBytes(tunnel.txBytes)}</span>
				</div>
			</div>
		</div>

		<div class="actions-wrapper">
			<Button variant="primary" onclick={handleAdopt}>
				{#snippet iconBefore()}
					<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
						<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
						<polyline points="9 12 12 15 16 10"/>
					</svg>
				{/snippet}
				Взять под управление
			</Button>
		</div>
	</div>
{/if}

<style>
	.ext-card {
		border: 1px dashed color-mix(in srgb, var(--warning, #f59e0b) 40%, transparent);
	}

	.list-card {
		display: grid;
		grid-template-columns: minmax(220px, 1.35fr) minmax(160px, 0.8fr) minmax(220px, 1.2fr) minmax(160px, 0.9fr) minmax(140px, 0.85fr) auto;
		gap: 14px;
		align-items: center;
		padding: 12px 14px;
	}

	.list-cell {
		min-width: 0;
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.list-label {
		font-size: 10px;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-muted);
	}

	.list-note {
		font-size: 11px;
		color: var(--text-muted);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.ext-card.view-compact {
		gap: 12px;
		padding: 12px 14px;
	}

	.tunnel-name {
		font-size: 1rem;
		font-weight: 600;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.ext-card.view-compact .tunnel-name {
		font-size: 0.95rem;
	}

	.iface-name {
		font-size: 12px;
		font-family: var(--font-mono, monospace);
		color: var(--text-muted);
	}

	.version-badge {
		display: inline-flex;
		align-items: center;
		padding: 2px 8px;
		font-size: 11px;
		font-weight: 500;
		border-radius: 10px;
	}

	.badge-external {
		background: rgba(245, 158, 11, 0.15);
		color: var(--warning, #f59e0b);
	}

	.status-badge {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 2px 10px;
		font-size: 12px;
		font-weight: 500;
		border-radius: 10px;
	}

	.status-active {
		background: rgba(16, 185, 129, 0.15);
		color: var(--success, #10b981);
	}

	.status-inactive {
		background: rgba(148, 163, 184, 0.15);
		color: var(--text-muted);
	}

	.led-dot {
		width: 6px;
		height: 6px;
		border-radius: 50%;
		background: currentColor;
		flex-shrink: 0;
	}

	.details {
		display: flex;
		flex-direction: column;
		gap: 12px;
		padding-top: 12px;
		border-top: 1px solid var(--border);
	}

	.ext-card.view-compact .details {
		gap: 10px;
		padding-top: 10px;
	}

	.detail-label {
		font-size: 11px;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--text-muted);
	}

	.detail-value {
		font-size: 13px;
		font-family: var(--font-mono, monospace);
		color: var(--text-secondary);
	}

	.actions-wrapper {
		padding-top: 12px;
		border-top: 1px solid var(--border);
	}

	@media (max-width: 1080px) {
		.list-card {
			grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
		}

		.list-cell-actions {
			grid-column: 1 / -1;
		}

	}

	@media (max-width: 720px) {
		.list-card {
			grid-template-columns: minmax(0, 1fr);
		}
	}
</style>
