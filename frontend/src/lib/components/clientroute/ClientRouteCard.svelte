<script lang="ts">
	import type { ClientRoute } from '$lib/types';
	import { Toggle } from '$lib/components/ui';
	import RoutingTargetBadges from '$lib/components/routing/RoutingTargetBadges.svelte';
	import NdmsIconTile from '$lib/components/ui/NdmsIconTile.svelte';
	import { DEFAULT_ICON_TILE_BG } from '$lib/utils/icon-tile-background';
	import {
		ndmsIconTileInnerSize,
		NDMS_ICON_TILE_SIZE,
	} from '$lib/utils/ndms-icon-tile';

	interface Props {
		route: ClientRoute;
		tunnelName: string;
		ontoggle: (enabled: boolean) => void;
		onedit: () => void;
		ondelete: () => void;
		toggleLoading?: boolean;
		selectable?: boolean;
		selected?: boolean;
		onselect?: () => void;
	}

	let {
		route,
		tunnelName,
		ontoggle,
		onedit,
		ondelete,
		toggleLoading = false,
		selectable = false,
		selected = false,
		onselect,
	}: Props = $props();

	let clientLabel = $derived(route.clientHostname || route.clientIp);
	let iconSize = $derived(ndmsIconTileInnerSize(NDMS_ICON_TILE_SIZE));
</script>

<div
	class="dns-card"
	class:enabled={route.enabled}
	class:selected={selectable && selected}
>
	<div class="card-main">
		{#if selectable}
			<input
				type="checkbox"
				class="select-check"
				checked={selected}
				onchange={() => onselect?.()}
			/>
		{/if}
		<NdmsIconTile background={DEFAULT_ICON_TILE_BG} size={NDMS_ICON_TILE_SIZE}>
			<svg
				class="device-icon"
				width={iconSize}
				height={iconSize}
				viewBox="0 0 24 24"
				fill="none"
				stroke="currentColor"
				stroke-width="1.75"
				stroke-linecap="round"
				stroke-linejoin="round"
				aria-hidden="true"
			>
				<rect x="5" y="7" width="14" height="12" rx="2" />
				<line x1="12" y1="7" x2="12" y2="3" />
				<circle cx="12" cy="2" r="1" fill="currentColor" stroke="none" />
				<circle cx="9" cy="13" r="1" fill="currentColor" stroke="none" />
				<circle cx="15" cy="13" r="1" fill="currentColor" stroke="none" />
			</svg>
		</NdmsIconTile>
		<div class="card-info">
			<div class="card-title">
				<span
					class="led"
					class:led-green={route.enabled}
					class:led-gray={!route.enabled}
				></span>
				<h3>{route.clientHostname || route.clientIp}</h3>
			</div>
			{#if route.clientHostname}
				<span class="card-stat">IP: {route.clientIp}</span>
			{/if}
			<span class="card-stat">{route.fallback === 'drop' ? 'Fallback: блокировать' : 'Fallback: напрямую'}</span>
			<div class="card-route">
				<RoutingTargetBadges labels={[tunnelName]} overflowNoun="туннелей" />
			</div>
		</div>
	</div>
	<div class="card-actions">
		<Toggle
			checked={route.enabled}
			onchange={(checked) => ontoggle(checked)}
			loading={toggleLoading}
			ariaLabel={`Переключить клиентское правило "${clientLabel || 'без названия'}"`}
			size="sm"
		/>
		<div class="action-row">
			<button
				type="button"
				class="route-action-btn"
				title={`Изменить VPN-маршрут устройства «${clientLabel}»`}
				onclick={() => onedit()}
			>
				<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
					<path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
				</svg>
			</button>
			<button
				type="button"
				class="route-action-btn danger"
				title={`Удалить VPN-маршрут устройства «${clientLabel}»`}
				onclick={() => ondelete()}
			>
				<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
					<polyline points="3 6 5 6 21 6"/>
					<path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
				</svg>
			</button>
		</div>
	</div>
</div>

<style>
	.dns-card {
		display: flex;
		justify-content: space-between;
		border-radius: 8px;
		padding: 14px;
		background: var(--bg-secondary);
		border: 1px solid var(--border);
		transition: border-color 0.2s;
	}

	.dns-card:hover {
		border-color: var(--border-hover);
	}

	.dns-card.selected {
		border-color: var(--accent);
	}

	.dns-card:not(.enabled) {
		opacity: 0.4;
	}

	.card-main {
		display: flex;
		gap: 10px;
		min-width: 0;
	}

	.device-icon {
		display: block;
	}

	.card-info {
		display: flex;
		flex-direction: column;
		gap: 1px;
		min-width: 0;
	}

	.card-title {
		display: flex;
		align-items: center;
		gap: 6px;
	}

	.card-title h3 {
		font-size: 0.875rem;
		font-weight: 600;
		margin: 0;
	}

	.card-stat {
		font-size: 0.6875rem;
		color: var(--text-muted);
	}

	.card-actions {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: 8px;
		flex-shrink: 0;
		margin-left: 8px;
		align-self: stretch;
	}

	.action-row {
		display: flex;
		gap: 4px;
		align-items: center;
		margin-top: auto;
	}

	.led {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		flex-shrink: 0;
	}

	.led-green {
		background: var(--success);
		box-shadow: 0 0 6px var(--success);
	}

	.led-gray {
		background: var(--text-muted);
	}

	.select-check {
		accent-color: var(--accent);
		width: 16px;
		height: 16px;
		cursor: pointer;
		flex-shrink: 0;
		margin-top: 10px;
	}
</style>
