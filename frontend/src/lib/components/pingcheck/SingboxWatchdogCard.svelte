<script lang="ts" module>
	export type SingboxWatchdogSource = 'tunnel' | 'subscription';

	export interface SingboxWatchdogCardModel {
		id: string;
		name: string;
		routeHref: string;
		source: SingboxWatchdogSource;
		sourceLabel?: string;
		/**
		 * Display tag: raw outbound tag or active subscription member tag.
		 */
		tag: string;
		/**
		 * Tag passed to /singbox/tunnels/delay-check.
		 * For raw tunnel: tunnel.tag.
		 * For subscription: selectorTag when available, else active member tag.
		 */
		delayCheckTag: string;
		/**
		 * Preferred tag for reading singboxDelayHistory.
		 * For subscription this is usually selectorTag.
		 */
		primaryHistoryTag: string;
		/**
		 * Fallback history tag.
		 * For subscription this should be activeMember.tag.
		 */
		fallbackHistoryTag?: string;
		/**
		 * Tag used for traffic history/rates.
		 * For subscription this should be activeMember.tag.
		 */
		trafficTag: string;
		protocol?: string;
		security?: string;
		transport?: string;
		proxyInterface?: string;
		kernelInterface?: string;
		running: boolean;
	}
</script>

<script lang="ts">
	import { goto } from '$app/navigation';
	import { untrack } from 'svelte';
	import { api } from '$lib/api/client';
	import { Badge, Button, StatusDot } from '$lib/components/ui';
	import {
		TunnelDelaySparkBars,
		TunnelListTrafficCell,
		TunnelMetaText,
		TunnelTitleRow,
	} from '$lib/components/tunnels';
	import { notifications } from '$lib/stores/notifications';
	import { singboxDelayHistory } from '$lib/stores/singbox';
	import { getTrafficRates, loadHistory, subscribeTraffic } from '$lib/stores/traffic';
	import type { SingboxDelayState } from '$lib/utils/singboxDelay';
	import { singboxDelayFromHistory } from '$lib/utils/singboxDelay';
	import { singboxDelayStatusDot } from '$lib/utils/statusDot';
	import { getSingboxProtocolLabel } from '$lib/utils/singboxPresentation';

	interface Props {
		card: SingboxWatchdogCardModel;
		autoDelayCheckNonce?: number;
		autoDelayCheckDelayMs?: number;
	}

	let { card, autoDelayCheckNonce = 0, autoDelayCheckDelayMs = 0 }: Props = $props();

	let checking = $state(false);
	let rxRates = $state<number[]>([]);
	let txRates = $state<number[]>([]);

	const primaryHistory = $derived(
		card.primaryHistoryTag ? ($singboxDelayHistory.get(card.primaryHistoryTag) ?? []) : [],
	);
	const fallbackHistory = $derived(
		card.fallbackHistoryTag ? ($singboxDelayHistory.get(card.fallbackHistoryTag) ?? []) : [],
	);
	const history = $derived(primaryHistory.length > 0 ? primaryHistory : fallbackHistory);
	const delayPresentation = $derived(
		singboxDelayFromHistory(history, { running: card.running }),
	);
	const latest = $derived(delayPresentation.latest);
	const positiveHistory = $derived(history.filter((value) => value > 0));
	const average = $derived(
		positiveHistory.length > 0
			? Math.round(positiveHistory.reduce((sum, value) => sum + value, 0) / positiveHistory.length)
			: 0,
	);
	const statusDot = $derived(singboxDelayStatusDot(delayPresentation.state, card.running));
	const protocolLabel = $derived(getSingboxProtocolLabel(card.protocol));
	const latestLabel = $derived.by(() => {
		if (!card.running) return 'stopped';
		if (latest === undefined) return '—';
		if (latest <= 0) return 'timeout';
		return `${latest}ms`;
	});

	const trafficSparkSeries = $derived.by(() => {
		const n = Math.min(rxRates.length, txRates.length);
		if (n === 0) return { rx: [] as number[], tx: [] as number[] };
		const take = Math.min(28, n);
		const start = n - take;
		return {
			rx: rxRates.slice(start, n),
			tx: txRates.slice(start, n),
		};
	});

	$effect(() => {
		const tag = card.trafficTag;
		const update = () => {
			const trafficRates = getTrafficRates(tag);
			rxRates = trafficRates.rx;
			txRates = trafficRates.tx;
		};
		update();
		return subscribeTraffic(update);
	});

	let lastLoadedTrafficTag = '';
	$effect(() => {
		const tag = card.trafficTag;
		if (!tag || tag === lastLoadedTrafficTag) return;
		lastLoadedTrafficTag = tag;
		untrack(() => void loadHistory(tag));
	});

	async function runDelayCheck(): Promise<void> {
		const tag = card.delayCheckTag.trim();
		if (checking || !tag) return;
		checking = true;
		try {
			await api.singboxDelayCheck(tag);
		} catch (error) {
			notifications.error(error instanceof Error ? error.message : 'Не удалось проверить задержку sing-box');
		} finally {
			checking = false;
		}
	}

	function openEditor(): void {
		void goto(card.routeHref);
	}

	let lastAutoDelayCheckNonce = 0;
	$effect(() => {
		const nonce = autoDelayCheckNonce;
		if (nonce <= 0 || nonce === lastAutoDelayCheckNonce || !card.running || !card.delayCheckTag.trim()) return;
		lastAutoDelayCheckNonce = nonce;

		const timer = setTimeout(() => {
			untrack(() => void runDelayCheck());
		}, autoDelayCheckDelayMs);
		return () => clearTimeout(timer);
	});

	function badgeVariantForDelay(state: SingboxDelayState): 'success' | 'warning' | 'error' | 'muted' {
		switch (state) {
			case 'ok':
				return 'success';
			case 'slow':
				return 'warning';
			case 'fail':
				return 'error';
			default:
				return 'muted';
		}
	}
</script>

<article class="wd-card sbx-wd-card" aria-label={`${card.source === 'subscription' ? 'Subscription' : 'Sing-box'} watchdog ${card.name}`}>
	<div class="wd-head">
		<div class="wd-head-main">
			<div class="wd-head-top">
				<div class="wd-head-title">
					<StatusDot variant={statusDot.variant} pulse={statusDot.pulse} size="sm" ariaLabel={card.name} />
					<TunnelTitleRow title={card.name} staticTitle showDot={false} />
				</div>
				<div class="wd-head-side">
					{#if card.sourceLabel}
						<Badge variant="accent" size="xs" compact mono pill>{card.sourceLabel}</Badge>
					{/if}
					<Badge variant={badgeVariantForDelay(delayPresentation.state)} size="xs" compact mono pill>{latestLabel}</Badge>
				</div>
			</div>
			<div class="wd-head-subline">
				<TunnelMetaText mono>
					<span>{card.proxyInterface || 'via sing-box'}</span>
					{#if card.kernelInterface}
						<span class="meta-dot" aria-hidden="true">·</span><span>{card.kernelInterface}</span>
					{/if}
					{#if card.tag && card.tag !== card.name}
						<span class="meta-dot" aria-hidden="true">·</span><span>{card.tag}</span>
					{/if}
				</TunnelMetaText>
			</div>
			<div class="sbx-wd-badges">
				<Badge variant="accent" mono pill>{protocolLabel}</Badge>
				{#if card.security === 'reality'}
					<Badge variant="warning" mono pill>Reality</Badge>
				{:else if card.security === 'tls'}
					<Badge variant="info" mono pill>TLS</Badge>
				{/if}
				{#if card.transport}
					<Badge variant="muted" mono pill>{card.transport.toUpperCase()}</Badge>
				{/if}
				<Badge variant={card.running ? 'success' : 'muted'} mono pill>
					{card.running ? 'running' : 'stopped'}
				</Badge>
			</div>
		</div>
	</div>

	<div class="wd-stats">
		<div class="wd-stat">
			<span class="v">{latestLabel}</span>
			<span class="k">latest</span>
		</div>
		<div class="wd-stat">
			<span class="v">{average > 0 ? `${average}ms` : '—'}</span>
			<span class="k">avg</span>
		</div>
	</div>

	<div class="wd-checks">
		<div class="wd-checks-head">
			<span class="wd-checks-title">Задержка</span>
		</div>
		<div class="wd-bars">
			<TunnelDelaySparkBars
				history={history}
				state={delayPresentation.state}
				maxBars={12}
				colorPerBar
				title="Проверить задержку"
				onclick={() => void runDelayCheck()}
			/>
		</div>
	</div>

	<div class="sbx-wd-traffic">
		<TunnelListTrafficCell
			rxRate={rxRates.length > 0 ? rxRates[rxRates.length - 1] : 0}
			txRate={txRates.length > 0 ? txRates[txRates.length - 1] : 0}
			rxData={trafficSparkSeries.rx}
			txData={trafficSparkSeries.tx}
			title="Live traffic rate"
		/>
	</div>

	<div class="wd-foot">
		<Button
			type="button"
			variant="outline-primary"
			size="sm"
			disabled={checking || !card.running}
			loading={checking}
			onclick={() => void runDelayCheck()}
			title={card.running ? 'Проверить задержку' : 'Sing-box не запущен'}
		>
			Проверить
		</Button>
		<Button type="button" variant="secondary" size="sm" onclick={openEditor}>Открыть</Button>
	</div>
</article>

<style>
	.wd-card {
		display: flex;
		flex-direction: column;
		background: var(--color-bg-tertiary);
		border: 1px solid var(--color-border);
		border-radius: 12px;
		overflow: hidden;
		container-type: inline-size;
	}

	.wd-head {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 10px;
		padding: 12px 14px 10px;
		border-bottom: 1px solid var(--color-border);
	}

	.wd-head-main {
		display: flex;
		flex-direction: column;
		gap: 6px;
		min-width: 0;
		flex: 1 1 auto;
	}

	.wd-head-top {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: start;
		gap: 10px;
		min-width: 0;
	}

	.wd-head-title {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
		flex: 1 1 auto;
	}

	.wd-head-side {
		display: flex;
		align-items: center;
		justify-content: flex-end;
		flex-wrap: nowrap;
		gap: 6px;
		flex: 0 0 auto;
		white-space: nowrap;
	}

	.wd-head-subline {
		min-width: 0;
	}

	.wd-stats {
		display: grid;
		border-bottom: 1px solid var(--color-border);
	}

	.wd-stat {
		padding: 9px 6px;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 2px;
		border-right: 1px solid color-mix(in srgb, var(--color-border) 55%, transparent);
	}

	.wd-stat:last-child {
		border-right: none;
	}

	.wd-stat .v {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 3px;
		font-family: var(--font-mono);
		font-size: 15px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.wd-stat .k {
		font-size: 10px;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--color-text-muted);
	}

	.wd-checks {
		padding: 12px 14px 0;
		flex: 1;
	}

	.wd-checks-head {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		margin-bottom: 6px;
	}

	.wd-checks-title {
		font-size: 10px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--color-text-muted);
	}

	.wd-bars {
		display: flex;
		align-items: flex-end;
		height: 38px;
		margin-bottom: 12px;
		padding: 2px;
		background: color-mix(in srgb, var(--color-border) 22%, transparent);
		border-radius: 4px;
	}

	.wd-bars :global(.tunnel-delay-spark) {
		flex: 1;
		height: 100%;
		gap: 2px;
	}

	.wd-foot {
		padding: 10px 14px;
		border-top: 1px solid var(--color-border);
		display: flex;
		justify-content: flex-end;
		gap: 8px;
	}

	.sbx-wd-card {
		gap: 0;
	}

	.sbx-wd-badges {
		display: flex;
		flex-wrap: wrap;
		gap: 5px;
		min-width: 0;
	}

	.sbx-wd-traffic {
		padding: 0 14px 12px;
	}

	.sbx-wd-traffic :global(.tunnel-list-traffic-cell) {
		border-radius: 8px;
		background: color-mix(in srgb, var(--color-border) 14%, transparent);
	}

	.sbx-wd-traffic :global(.traffic-rate) {
		font-family: var(--font-mono);
	}
	.wd-stats {
		grid-template-columns: repeat(2, 1fr);
	}

	@container (max-width: 380px) {
		.wd-head-top {
			grid-template-columns: 1fr;
			gap: 8px;
		}

		.wd-head-side {
			justify-content: flex-start;
		}
	}

	@media (max-width: 640px) {
		.sbx-wd-card .wd-head {
			align-items: stretch;
		}

		.wd-foot {
			flex-wrap: wrap;
		}
	}
</style>
