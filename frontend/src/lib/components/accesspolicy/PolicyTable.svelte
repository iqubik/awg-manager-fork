<script lang="ts">
	import type { AccessPolicy } from '$lib/types';
	import { pluralize, DEVICE_WORDS } from '$lib/utils/pluralize';
	import { Badge } from '$lib/components/ui';
	import RoutingTargetBadges from '$lib/components/routing/RoutingTargetBadges.svelte';
	import PolicyIcon from './PolicyIcon.svelte';
	import { isHydraRouteAccessPolicy } from '$lib/utils/accessPolicy';

	interface Props {
		policies: AccessPolicy[];
		onedit: (name: string) => void;
		ondelete: (name: string) => void;
		selectable?: boolean;
		selectedNames?: Set<string>;
		onselect?: (name: string) => void;
	}

	let { policies, onedit, ondelete, selectable, selectedNames, onselect }: Props = $props();
</script>

<div class="policy-grid">
	{#each policies as policy}
		{@const isHrPolicy = isHydraRouteAccessPolicy(policy)}
		{@const policyLabel = policy.description || policy.name}
		<div class="policy-card" class:policy-card-hr={isHrPolicy}>
			{#if selectable}
				<div class="select-cell">
					<input
						type="checkbox"
						class="select-check"
						checked={selectedNames?.has(policy.name)}
						disabled={isHrPolicy}
						onchange={() => onselect?.(policy.name)}
					/>
				</div>
			{/if}
			<div class="card-main">
				<PolicyIcon
					label={policy.description}
					policyName={policy.name}
					isHydraRoute={isHrPolicy}
				/>
				<div class="card-info">
					<div class="card-title">
						<h3 title={policyLabel}>{policyLabel}</h3>
						<div class="routing-badges">
							{#if isHrPolicy}
								<Badge variant="warning" uppercase size="xs" pill>HydraRoute</Badge>
							{/if}
							{#if policy.standalone}
								<Badge variant="accent" uppercase size="xs" pill>standalone</Badge>
							{/if}
						</div>
					</div>
					<span class="card-stat">{pluralize(policy.deviceCount, DEVICE_WORDS)}</span>
					{#if policy.interfaces?.length}
						{@const sortedIfaces = [...policy.interfaces].sort((a, b) => a.order - b.order)}
						<div class="card-route">
							<RoutingTargetBadges
								labels={sortedIfaces.map((iface) => iface.label || iface.name)}
								titles={sortedIfaces.map((iface) => iface.name)}
								overflowNoun="интерфейсов"
							/>
						</div>
					{/if}
				</div>
			</div>
			<div class="card-actions">
				<div class="action-row">
					<button
						type="button"
						class="route-action-btn"
						title={isHrPolicy
							? `Открыть HydraRoute-политику «${policy.description || policy.name}»`
							: `Изменить политику «${policy.description || policy.name}»`}
						onclick={() => onedit(policy.name)}
					>
						<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
							<path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
							<path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
						</svg>
					</button>
					{#if !isHrPolicy}
						<button
							type="button"
							class="route-action-btn danger"
							title={`Удалить политику «${policy.description || policy.name}»`}
							onclick={() => ondelete(policy.name)}
						>
							<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
								<polyline points="3 6 5 6 21 6"/>
								<path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
							</svg>
						</button>
					{/if}
				</div>
			</div>
		</div>
	{/each}
</div>

<style>
	.policy-grid {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 12px;
	}

	.policy-card {
		display: flex;
		justify-content: space-between;
		background: var(--bg-secondary);
		border: 1px solid var(--border);
		border-radius: 8px;
		padding: 14px;
		gap: 10px;
		transition: border-color 0.15s;
	}

	.policy-card:hover {
		border-color: var(--border-hover);
	}

	.policy-card .card-main {
		display: flex;
		gap: 10px;
		min-width: 0;
		align-items: flex-start;
	}

	.policy-card .card-info {
		display: flex;
		flex-direction: column;
		gap: 1px;
		min-width: 0;
	}

	.policy-card .card-title {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 6px;
		min-width: 0;
	}

	.policy-card .card-title h3 {
		font-size: 0.875rem;
		font-weight: 600;
		color: var(--text-primary);
		margin: 0;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		min-width: 0;
	}

	.policy-card .card-stat {
		font-size: 0.6875rem;
		color: var(--text-muted);
	}

	.policy-card-hr {
		border-color: rgba(245, 158, 11, 0.35);
	}

	.policy-card .card-actions {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		flex-shrink: 0;
		margin-left: 8px;
	}

	.policy-card .action-row {
		display: flex;
		gap: 4px;
		align-items: center;
	}

	.select-cell {
		display: flex;
		align-items: center;
		align-self: stretch;
		flex-shrink: 0;
	}

	.select-check {
		accent-color: var(--accent);
		width: 1rem;
		height: 1rem;
		cursor: pointer;
	}

	@media (max-width: 1024px) {
		.policy-grid {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}
	}

	@media (max-width: 768px) {
		.policy-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
