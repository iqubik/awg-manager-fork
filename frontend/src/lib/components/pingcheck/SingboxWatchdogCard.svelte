<script lang="ts" module>
	import type {
		SingboxWatchdogLogEntry,
		SingboxWatchdogStatus,
		SingboxWatchdogStatusKind,
	} from '$lib/types';

	export type SingboxWatchdogSource = 'tunnel' | 'subscription';

	export interface SingboxWatchdogCardModel {
		id: string;
		name: string;
		routeHref: string;
		source: SingboxWatchdogSource;
		sourceLabel?: string;
		tag: string;
		delayCheckTag: string;
		primaryHistoryTag: string;
		fallbackHistoryTag?: string;
		trafficTag: string;
		protocol?: string;
		security?: string;
		transport?: string;
		proxyInterface?: string;
		kernelInterface?: string;
		running: boolean;
		configured: boolean;
		enabled: boolean;
		statusKind: SingboxWatchdogStatusKind;
		failCount: number;
		failThreshold: number;
		restartCount: number;
		switchCount: number;
		lastError?: string;
		lastRecovery?: string;
		status?: SingboxWatchdogStatus;
		logs?: SingboxWatchdogLogEntry[];
	}
</script>

<script lang="ts">
	import { goto } from '$app/navigation';
	import { api } from '$lib/api/client';
	import { Badge, Button, StatusDot, TrafficChartModal } from '$lib/components/ui';
	import WatchdogLogNavigator from '$lib/components/monitoring/WatchdogLogNavigator.svelte';
	import {
		TunnelDelaySparkBars,
		TunnelListTrafficCell,
		TunnelMetaText,
		TunnelTitleRow,
	} from '$lib/components/tunnels';
	import { getTrafficRates, loadHistory, subscribeTraffic } from '$lib/stores/traffic';
	import { singboxDelayHistory } from '$lib/stores/singbox';
	import { singboxDelayFromHistory } from '$lib/utils/singboxDelay';
	import { singboxDelayStatusDot } from '$lib/utils/statusDot';
	import { getSingboxProtocolLabel } from '$lib/utils/singboxPresentation';
	import {
		normalizeSingboxWatchdogLogEntry,
		sortWatchdogEntries,
		type WatchdogCheckEntry,
	} from '$lib/utils/watchdogHistory';
	import type { StatusDotVariant, BadgeVariant } from '$lib/components/ui';
	import type { SingboxDelayState } from '$lib/utils/singboxDelay';

	interface Props {
		card: SingboxWatchdogCardModel;
		onConfigure: () => void;
		onCheckNow: () => void;
		onDisable: () => void;
		onEnable: () => void;
	}

	let { card, onConfigure, onCheckNow, onDisable, onEnable }: Props = $props();

	let checking = $state(false);
	let delayChecking = $state(false);
	let trafficOpen = $state(false);
	let rxRates = $state<number[]>([]);
	let txRates = $state<number[]>([]);

	const STATUS: Record<
		SingboxWatchdogStatusKind,
		{ dot: StatusDotVariant; pulse: boolean; label: string; badge: BadgeVariant }
	> = {
		alive: { dot: 'success', pulse: false, label: 'активен', badge: 'success' },
		warming: { dot: 'muted', pulse: true, label: 'ожидание', badge: 'muted' },
		recovering: { dot: 'warning', pulse: true, label: 'восстановление', badge: 'warning' },
		dead: { dot: 'error', pulse: true, label: 'сбой', badge: 'error' },
		disabled: { dot: 'muted', pulse: false, label: 'выключен', badge: 'muted' },
		stopped: { dot: 'muted', pulse: false, label: 'остановлен', badge: 'muted' },
	};

	const primaryHistory = $derived(
		card.primaryHistoryTag ? ($singboxDelayHistory.get(card.primaryHistoryTag) ?? []) : [],
	);
	const fallbackHistory = $derived(
		card.fallbackHistoryTag ? ($singboxDelayHistory.get(card.fallbackHistoryTag) ?? []) : [],
	);
	const history = $derived(primaryHistory.length > 0 ? primaryHistory : fallbackHistory);
	const delayPresentation = $derived(singboxDelayFromHistory(history, { running: card.running }));
	const latest = $derived(delayPresentation.latest);
	const positiveHistory = $derived(history.filter((value) => value > 0));
	const average = $derived(
		positiveHistory.length > 0
			? Math.round(positiveHistory.reduce((sum, value) => sum + value, 0) / positiveHistory.length)
			: 0,
	);
	const protocolLabel = $derived(getSingboxProtocolLabel(card.protocol));
	const watchdogStatus = $derived(STATUS[card.statusKind]);
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
	const logEntries = $derived<WatchdogCheckEntry[]>(
		sortWatchdogEntries((card.logs ?? []).map(normalizeSingboxWatchdogLogEntry)),
	);
	const lossPct = $derived.by(() => {
		const logs = card.logs ?? [];
		if (logs.length === 0) return 0;
		const failed = logs.filter((entry) => !entry.success).length;
		return Math.round((failed / logs.length) * 100);
	});
	const delayBadgeVariant = $derived(badgeVariantForDelay(delayPresentation.state));

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
		void loadHistory(tag);
	});

	function openEditor(): void {
		void goto(card.routeHref);
	}

	async function checkNow(): Promise<void> {
		if (checking || (!card.running && !card.enabled)) return;
		checking = true;
		try {
			await onCheckNow();
		} finally {
			checking = false;
		}
	}

	async function checkDelay(): Promise<void> {
		if (delayChecking || !card.delayCheckTag) return;
		delayChecking = true;
		try {
			await api.singboxDelayCheck(card.delayCheckTag);
		} finally {
			delayChecking = false;
		}
	}

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

	const latestLabel = $derived.by(() => {
		if (card.statusKind === 'dead') return 'timeout';
		if (!card.running) return 'stopped';
		if (card.status?.lastLatency && card.status.lastLatency > 0) return `${card.status.lastLatency}ms`;
		if (latest === undefined) return '—';
		if (latest <= 0) return 'timeout';
		return `${latest}ms`;
	});

	const countLabel = $derived(card.source === 'subscription' ? 'переключения' : 'рестарты');
	const countValue = $derived(card.source === 'subscription' ? card.switchCount : card.restartCount);
	const trafficModalTitle = $derived(`Watchdog · ${card.name}`);
</script>

<article class="wd-card sbx-wd-card" aria-label={`${card.source === 'subscription' ? 'Subscription' : 'Sing-box'} watchdog ${card.name}`}>
	<div class="wd-head">
		<div class="wd-head-main">
			<div class="wd-head-top">
				<div class="wd-head-title">
					<StatusDot
						variant={watchdogStatus.dot}
						pulse={watchdogStatus.pulse}
						size="sm"
						ariaLabel={card.name}
					/>
					<TunnelTitleRow title={card.name} staticTitle showDot={false} />
				</div>
				<div class="wd-head-side">
					{#if card.sourceLabel}
						<Badge variant="accent" size="xs" compact mono pill>{card.sourceLabel}</Badge>
					{/if}
					<Badge variant={watchdogStatus.badge} size="xs" compact mono pill>{watchdogStatus.label}</Badge>
					<Badge variant={delayBadgeVariant} size="xs" compact mono pill>{latestLabel}</Badge>
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

	{#if !card.configured}
		<div class="wd-note">
			<div class="wd-note-title">Watchdog не настроен для этой Sing-box цели.</div>
			<div class="wd-note-text">Сохраните настройки, чтобы включить периодические проверки и recovery.</div>
		</div>
		<div class="wd-foot">
			<Button type="button" variant="outline-primary" size="sm" onclick={onConfigure}>Включить</Button>
			<Button type="button" variant="secondary" size="sm" onclick={openEditor}>Открыть</Button>
		</div>
	{:else if !card.enabled}
		<div class="wd-note">
			<div class="wd-note-title">Watchdog выключен.</div>
			<div class="wd-note-text">Настройки сохранены. Можно включить мониторинг без повторной настройки.</div>
		</div>
		<div class="wd-foot">
			<Button type="button" variant="outline-primary" size="sm" onclick={onEnable}>Включить</Button>
			<Button type="button" variant="outline-primary" size="sm" onclick={onConfigure}>Настроить</Button>
			<Button type="button" variant="secondary" size="sm" onclick={openEditor}>Открыть</Button>
		</div>
	{:else}
		<div class="wd-stats">
			<div class="wd-stat">
				<span class="v">{latestLabel}</span>
				<span class="k">latest</span>
			</div>
			<div class="wd-stat">
				<span class="v">{average > 0 ? `${average}ms` : '—'}</span>
				<span class="k">avg</span>
			</div>
			<div class="wd-stat">
				<span class="v">{card.failCount}/{card.failThreshold}</span>
				<span class="k">fails</span>
			</div>
			<div class="wd-stat">
				<span class="v">{countValue}</span>
				<span class="k">{countLabel}</span>
			</div>
		</div>

		<div class="wd-checks">
			<div class="wd-checks-head">
				<span class="wd-checks-title">Задержка</span>
				<span class="wd-minmax">loss {lossPct}%</span>
			</div>
			<div class="wd-bars">
				<TunnelDelaySparkBars
					history={history}
					state={delayPresentation.state}
					maxBars={12}
					colorPerBar
					title="Проверить задержку"
					onclick={() => void checkDelay()}
				/>
			</div>

			{#if card.lastError}
				<div class="wd-error">{card.lastError}</div>
			{/if}

			<WatchdogLogNavigator entries={logEntries} />
		</div>

		<div class="sbx-wd-traffic">
			<TunnelListTrafficCell
				rxRate={rxRates.length > 0 ? rxRates[rxRates.length - 1] : 0}
				txRate={txRates.length > 0 ? txRates[txRates.length - 1] : 0}
				rxData={trafficSparkSeries.rx}
				txData={trafficSparkSeries.tx}
				onclick={card.trafficTag ? () => (trafficOpen = true) : undefined}
				title={card.trafficTag ? 'Открыть график скорости' : 'Нет данных скорости'}
			/>
		</div>

		<div class="wd-foot">
			<Button
				type="button"
				variant="outline-primary"
				size="sm"
				disabled={checking}
				loading={checking}
				onclick={() => void checkNow()}
			>
				Проверка watchdog
			</Button>
			<Button type="button" variant="outline-danger" size="sm" onclick={onDisable}>Выключить</Button>
			<Button type="button" variant="outline-primary" size="sm" onclick={onConfigure}>Настроить</Button>
			<Button type="button" variant="secondary" size="sm" onclick={openEditor}>Открыть</Button>
		</div>
	{/if}
</article>

{#if trafficOpen && card.trafficTag}
	<TrafficChartModal
		open={true}
		source={{
			kind: 'traffic-store',
			key: card.trafficTag,
			title: trafficModalTitle,
			ifaceName: card.proxyInterface || 'sing-box',
		}}
		onclose={() => (trafficOpen = false)}
	/>
{/if}

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

	.sbx-wd-badges {
		display: flex;
		flex-wrap: wrap;
		gap: 5px;
		min-width: 0;
	}

	.wd-note {
		padding: 14px;
		display: flex;
		flex-direction: column;
		gap: 6px;
		color: var(--color-text-muted);
	}

	.wd-note-title {
		color: var(--color-text-primary);
		font-weight: 600;
	}

	.wd-stats {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
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

	.wd-minmax {
		font-family: var(--font-mono);
		font-size: 11px;
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

	.wd-error {
		margin-bottom: 10px;
		padding: 8px 10px;
		border-radius: 8px;
		background: color-mix(in srgb, var(--color-error) 14%, transparent);
		color: var(--color-error);
		font-size: 12px;
	}

	.sbx-wd-traffic {
		padding: 0 14px 12px;
	}

	.sbx-wd-traffic :global(.tunnel-list-traffic-cell) {
		border-radius: 8px;
		background: color-mix(in srgb, var(--color-border) 14%, transparent);
	}

	.wd-foot {
		padding: 10px 14px;
		border-top: 1px solid var(--color-border);
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		flex-wrap: wrap;
	}

	@container (max-width: 460px) {
		.wd-foot {
			display: grid;
			grid-template-columns: repeat(2, minmax(0, 1fr));
			align-items: stretch;
			justify-content: stretch;
		}

		.wd-foot > :global(.btn) {
			width: 100%;
			min-width: 0;
			justify-content: center;
		}

		.wd-foot > :global(.btn):last-child:nth-child(odd) {
			grid-column: 1 / -1;
		}
	}

	@container (max-width: 380px) {
		.wd-head-top {
			grid-template-columns: 1fr;
			gap: 8px;
		}

		.wd-head-side {
			justify-content: flex-start;
			flex-wrap: wrap;
		}

		.wd-stats {
			grid-template-columns: repeat(2, 1fr);
		}
	}
</style>
