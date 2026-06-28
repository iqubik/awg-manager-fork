<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { pingCheckStatus, pingCheckLogs, loadPingLogs } from '$lib/stores/pingcheck';
	import { singboxStatus, singboxTunnels } from '$lib/stores/singbox';
	import { subscriptionsStore } from '$lib/stores/subscriptions';
	import { usageLevel } from '$lib/stores/settings';
	import { isSectionVisible } from '$lib/types/usageLevel';
	import { resolveSubscriptionMemberTag } from '$lib/utils/subscriptionMember';
	import { groupLogsByTunnel, computeCardStats } from '$lib/utils/pingStats';
	import { WatchdogCard } from '$lib/components/pingcheck';
	import SingboxWatchdogCard, { type SingboxWatchdogCardModel } from './SingboxWatchdogCard.svelte';
	import KernelPingCheckModal from '$lib/components/pingcheck/KernelPingCheckModal.svelte';
	import NativeWGPingCheckModal from '$lib/components/pingcheck/NativeWGPingCheckModal.svelte';
	import { EmptyState } from '$lib/components/layout';
	import { notifications } from '$lib/stores/notifications';
	import type {
		AWGTunnel,
		NativePingCheckConfig,
		NativePingCheckStatus,
		Subscription,
		SubscriptionMember,
		TunnelListItem,
	} from '$lib/types';

	let tunnelMeta = $state<TunnelListItem[]>([]);
	let configs = $state<Record<string, AWGTunnel['pingCheck']>>({});
	let loading = $state(true);

	let editId = $state<string | null>(null);
	let editName = $state('');
	let editBackend = $state<'kernel' | 'nativewg'>('kernel');
	let editNativeStatus = $state<NativePingCheckStatus | null>(null);
	let unsubSingboxStatus: (() => void) | undefined;
	let unsubSingboxTunnels: (() => void) | undefined;
	let unsubSubscriptions: (() => void) | undefined;
	let singboxAutoDelayCheckNonce = $state(0);
	let lastSingboxAutoCheckKey = '';
	const URLTEST_POLL_MS = 5000;
	let liveActives = $state<Record<string, string>>({});

	const statuses = $derived($pingCheckStatus.data ?? []);
	const logsByTunnel = $derived(groupLogsByTunnel($pingCheckLogs));
	const singboxSectionVisible = $derived(isSectionVisible($usageLevel, 'singboxTunnels'));
	const singboxState = $derived($singboxStatus);
	const singboxTunnelsState = $derived($singboxTunnels);
	const subscriptionsState = $derived($subscriptionsStore);
	const singboxInstalled = $derived(singboxState.data?.installed === true);
	const singboxRunning = $derived(singboxState.data?.running === true);
	const singboxList = $derived(singboxTunnelsState.data ?? []);
	const subscriptions = $derived(subscriptionsState.data ?? []);

	$effect(() => {
		if (!singboxSectionVisible || !singboxInstalled) {
			liveActives = {};
			return;
		}

		const urltestSubs = subscriptions.filter(
			(subscription) =>
				subscription.enabled &&
				subscription.mode === 'urltest' &&
				(subscription.members?.length ?? 0) > 0,
		);

		if (urltestSubs.length === 0) {
			liveActives = {};
			return;
		}

		let cancelled = false;

		const tick = async (): Promise<void> => {
			try {
				const results = await Promise.all(
					urltestSubs.map((subscription) =>
						api
							.getSubscriptionActiveNow(subscription.id)
							.then((result) => [subscription.id, result.now] as const)
							.catch(() => [subscription.id, ''] as const),
					),
				);

				if (cancelled) return;

				const next: Record<string, string> = {};
				for (const [id, now] of results) {
					if (now) next[id] = now;
				}
				liveActives = next;
			} catch {
				// Keep previous known active pointers.
			}
		};

		void tick();
		const timer = setInterval(() => void tick(), URLTEST_POLL_MS);

		return () => {
			cancelled = true;
			clearInterval(timer);
		};
	});

	async function loadConfigs(ids: string[]) {
		const next: Record<string, AWGTunnel['pingCheck']> = {};
		await Promise.all(
			ids.map(async (id) => {
				try {
					const tunnel = await api.getTunnel(id);
					if (tunnel.pingCheck) next[id] = tunnel.pingCheck;
				} catch {
					// Tunnel may disappear between refreshes.
				}
			}),
		);
		configs = next;
	}

	onMount(async () => {
		unsubSingboxStatus = singboxStatus.subscribe(() => {});
		unsubSingboxTunnels = singboxTunnels.subscribe(() => {});
		unsubSubscriptions = subscriptionsStore.subscribe(() => {});

		try {
			await loadPingLogs();
			const snap = await api.getTunnelsAll();
			tunnelMeta = snap.tunnels ?? [];
		} catch {
			notifications.error('Не удалось загрузить мониторинг watchdog');
		} finally {
			loading = false;
		}
	});

	onDestroy(() => {
		unsubSingboxStatus?.();
		unsubSingboxTunnels?.();
		unsubSubscriptions?.();
	});

	$effect(() => {
		const ids = statuses.map((status) => status.tunnelId);
		if (ids.length > 0) void loadConfigs(ids);
	});

	function metaFor(id: string): TunnelListItem | undefined {
		return tunnelMeta.find((tunnel) => tunnel.id === id);
	}

	function configLine(cfg: AWGTunnel['pingCheck'] | undefined, method: string, failThreshold: number): string {
		if (!cfg) return `${method.toUpperCase()} · порог ${failThreshold}`;
		return `${(cfg.method || method).toUpperCase()} → ${cfg.target} · ${cfg.interval}с · порог ${cfg.failThreshold}`;
	}

	const awgCards = $derived.by(() => {
		const pcIds = new Set(statuses.map((status) => status.tunnelId));
		const enabled = statuses.map((status) => {
			const cfg = configs[status.tunnelId];
			const meta = metaFor(status.tunnelId);
			const isWatchdog = status.enabled === true;
			return {
				kind: 'pc' as const,
				id: status.tunnelId,
				name: status.tunnelName,
				backend: status.backend,
				awgVersion: meta?.awgVersion,
				statusKind: status.status,
				isWatchdog,
				configured: isWatchdog || !!cfg,
				configLine: configLine(cfg, status.method, status.failThreshold),
				stats: computeCardStats(logsByTunnel.get(status.tunnelId) ?? [], status),
			};
		});
		const noPc = tunnelMeta
			.filter((tunnel) => !pcIds.has(tunnel.id))
			.map((tunnel) => ({
				kind: 'note' as const,
				id: tunnel.id,
				name: tunnel.name,
				backend: (tunnel as { backend?: 'kernel' | 'nativewg' }).backend ?? 'kernel',
				awgVersion: tunnel.awgVersion,
			}));
		return [...enabled, ...noPc];
	});

	function buildSubscriptionCard(subscription: Subscription): SingboxWatchdogCardModel | null {
		if (!subscription.enabled || (subscription.members?.length ?? 0) === 0) return null;

		const activeTag = resolveSubscriptionMemberTag(subscription, liveActives[subscription.id] || null);
		if (!activeTag) return null;

		const activeMember: SubscriptionMember | undefined =
			subscription.members.find((member) => member.tag === activeTag) ?? subscription.members[0];

		if (!activeMember) return null;

		const selectorTag = subscription.selectorTag?.trim() || '';
		const delayCheckTag = selectorTag || activeMember.tag;
		const proxyInterface = subscription.proxyIndex >= 0 ? `Proxy${subscription.proxyIndex}` : '';
		const kernelInterface = subscription.proxyIndex >= 0 ? `t2s${subscription.proxyIndex}` : '';
		const modeLabel = subscription.mode === 'urltest' ? 'Подписка · URLTest' : 'Подписка · Selector';

		return {
			id: `subscription:${subscription.id}`,
			name: subscription.label?.trim() || activeMember.label?.trim() || activeMember.tag,
			routeHref: `/subscriptions/${encodeURIComponent(subscription.id)}`,
			source: 'subscription',
			sourceLabel: modeLabel,
			tag: activeMember.tag,
			delayCheckTag,
			primaryHistoryTag: delayCheckTag,
			fallbackHistoryTag: activeMember.tag,
			trafficTag: activeMember.tag,
			protocol: activeMember.protocol,
			security: activeMember.security,
			transport: activeMember.transport,
			proxyInterface,
			kernelInterface,
			running: singboxRunning,
		};
	}

	const singboxCards = $derived.by(() => {
		const rawCards: SingboxWatchdogCardModel[] = singboxList.map((tunnel) => ({
			id: `tunnel:${tunnel.tag}`,
			name: tunnel.tag,
			routeHref: `/singbox/${encodeURIComponent(tunnel.tag)}`,
			source: 'tunnel',
			sourceLabel: 'Sing-box',
			tag: tunnel.tag,
			delayCheckTag: tunnel.tag,
			primaryHistoryTag: tunnel.tag,
			trafficTag: tunnel.tag,
			protocol: tunnel.protocol,
			security: tunnel.security,
			transport: tunnel.transport,
			proxyInterface: tunnel.proxyInterface,
			kernelInterface: tunnel.kernelInterface,
			running: tunnel.running === true,
		}));

		const subscriptionCards = subscriptions
			.map((subscription) => buildSubscriptionCard(subscription))
			.filter((card): card is SingboxWatchdogCardModel => card !== null);

		return [...subscriptionCards, ...rawCards];
	});

	const singboxAutoCheckOrder = $derived.by(() => {
		const seenTags = new Set<string>();
		const order = new Map<string, number>();

		for (const card of singboxCards) {
			const tag = card.delayCheckTag.trim();
			if (!card.running || !tag || seenTags.has(tag)) continue;

			seenTags.add(tag);
			order.set(card.id, order.size);
		}

		return order;
	});

	const singboxLoading = $derived.by(() => {
		if (!singboxSectionVisible) return false;
		if (!singboxState.data && (singboxState.status === 'idle' || singboxState.status === 'loading')) return true;
		if (!singboxInstalled) return false;
		const tunnelsLoading =
			!singboxTunnelsState.data &&
			(singboxTunnelsState.status === 'idle' || singboxTunnelsState.status === 'loading');
		const subscriptionsLoading =
			!subscriptionsState.data &&
			(subscriptionsState.status === 'idle' || subscriptionsState.status === 'loading');
		return tunnelsLoading || subscriptionsLoading;
	});

	const singboxErrorMessage = $derived.by(() => {
		if (!singboxSectionVisible) return '';
		if (!singboxState.data && singboxState.status === 'error') return 'Не удалось загрузить Sing-box.';
		if (singboxInstalled && !singboxTunnelsState.data && singboxTunnelsState.status === 'error') {
			return 'Не удалось загрузить Sing-box туннели.';
		}
		if (singboxInstalled && !subscriptionsState.data && subscriptionsState.status === 'error') {
			return 'Не удалось загрузить подписки Sing-box.';
		}
		return '';
	});

	$effect(() => {
		if (!singboxSectionVisible || singboxLoading || !singboxInstalled) return;
		const tags = [...new Set(
			singboxCards
				.filter((card) => card.running && card.delayCheckTag.trim())
				.map((card) => card.delayCheckTag.trim()),
		)]
			.sort()
			.join(',');
		if (!tags) return;
		const key = `watchdog:${tags}`;
		if (key === lastSingboxAutoCheckKey) return;
		lastSingboxAutoCheckKey = key;
		singboxAutoDelayCheckNonce += 1;
	});

	async function openConfig(id: string, name: string, backend: 'kernel' | 'nativewg') {
		editName = name;
		editBackend = backend;
		editNativeStatus = null;
		if (backend === 'nativewg') {
			try {
				editNativeStatus = await api.getNativePingCheckStatus(id);
			} catch {
				editNativeStatus = null;
			}
		}
		editId = id;
	}

	function closeConfig() {
		editId = null;
	}

	function afterSave() {
		editId = null;
		pingCheckStatus.refetch();
		void loadPingLogs();
	}

	async function checkNow() {
		try {
			await api.triggerPingCheck();
			notifications.success('Проверка запущена');
		} catch {
			notifications.error('Не удалось запустить проверку');
		}
	}

	async function enablePingcheck(id: string, name: string, backend: 'kernel' | 'nativewg') {
		try {
			const tunnel = await api.getTunnel(id);
			const pingCheck = tunnel.pingCheck;
			if (!pingCheck) {
				notifications.error('Нет сохранённых настроек мониторинга');
				return;
			}
			if (backend === 'nativewg') {
				const cfg: NativePingCheckConfig = {
					host: pingCheck.target,
					mode: pingCheck.method as 'icmp' | 'connect' | 'tls',
					updateInterval: pingCheck.interval,
					maxFails: pingCheck.failThreshold,
					minSuccess: pingCheck.minSuccess,
					timeout: pingCheck.timeout,
					restart: pingCheck.restart,
				};
				if (pingCheck.port) cfg.port = pingCheck.port;
				await api.configureNativePingCheck(id, cfg);
			} else {
				pingCheck.enabled = true;
				await api.updateTunnel(id, tunnel);
			}
			notifications.success(`Watchdog включён: ${name}`);
			pingCheckStatus.refetch();
			void loadPingLogs();
		} catch {
			notifications.error('Не удалось включить watchdog');
		}
	}

	async function disablePingcheck(id: string, name: string, backend: 'kernel' | 'nativewg') {
		try {
			if (backend === 'nativewg') {
				await api.removeNativePingCheck(id);
			} else {
				const tunnel = await api.getTunnel(id);
				if (tunnel.pingCheck) tunnel.pingCheck.enabled = false;
				await api.updateTunnel(id, tunnel);
			}
			notifications.success(`Watchdog выключен: ${name}`);
			pingCheckStatus.refetch();
			void loadPingLogs();
		} catch {
			notifications.error('Не удалось выключить watchdog');
		}
	}
</script>

{#if loading}
	<div class="wd-grid">
		{#each Array(4) as _, i (i)}
			<div class="wd-skel"></div>
		{/each}
	</div>
{:else}
	{#if awgCards.length === 0 && !singboxSectionVisible}
		<EmptyState title="Нет туннелей" description="Создайте туннель, чтобы видеть мониторинг." />
	{:else}
		{#if awgCards.length > 0}
			<div class="wd-grid">
				{#each awgCards as card (card.id)}
					{#if card.kind === 'pc'}
						<WatchdogCard
							name={card.name}
							backend={card.backend}
							awgVersion={card.awgVersion}
							statusKind={card.statusKind}
							hasPingcheck={card.configured}
							isWatchdog={card.isWatchdog}
							configLine={card.configLine}
							stats={card.stats}
							onConfigure={() => openConfig(card.id, card.name, card.backend)}
							onCheckNow={checkNow}
							onDisable={() => disablePingcheck(card.id, card.name, card.backend)}
							onEnable={() => enablePingcheck(card.id, card.name, card.backend)}
						/>
					{:else}
						<WatchdogCard
							name={card.name}
							backend={card.backend}
							awgVersion={card.awgVersion}
							statusKind="disabled"
							hasPingcheck={false}
							isWatchdog={false}
							configLine=""
							stats={null}
							onConfigure={() => openConfig(card.id, card.name, card.backend)}
							onCheckNow={checkNow}
							onDisable={() => {}}
							onEnable={() => {}}
						/>
					{/if}
				{/each}
			</div>
		{/if}

		{#if singboxSectionVisible}
			<section class="wd-section">
				<div class="wd-section-head">
					<h3>Sing-box{#if !singboxLoading && singboxInstalled} · {singboxCards.length}{/if}</h3>
				</div>

				{#if singboxLoading}
					<div class="wd-notice">Загрузка данных Sing-box…</div>
				{:else if singboxErrorMessage}
					<div class="wd-notice wd-notice--error">{singboxErrorMessage}</div>
				{:else if !singboxInstalled}
					<div class="wd-notice">Sing-box не установлен.</div>
				{:else if singboxCards.length === 0}
					<div class="wd-notice">Нет Sing-box туннелей.</div>
				{:else}
					<div class="wd-grid">
						{#each singboxCards as card (card.id)}
							<SingboxWatchdogCard
								{card}
								autoDelayCheckNonce={singboxAutoCheckOrder.has(card.id) ? singboxAutoDelayCheckNonce : 0}
								autoDelayCheckDelayMs={(singboxAutoCheckOrder.get(card.id) ?? 0) * 180}
							/>
						{/each}
					</div>
				{/if}
			</section>
		{/if}
	{/if}
{/if}

{#if editId && editBackend === 'kernel'}
	<KernelPingCheckModal open={true} tunnelId={editId} tunnelName={editName} onclose={closeConfig} onSaved={afterSave} />
{:else if editId && editBackend === 'nativewg'}
	<NativeWGPingCheckModal
		open={true}
		tunnelId={editId}
		tunnelName={editName}
		status={editNativeStatus}
		onclose={closeConfig}
		onSaved={afterSave}
		onRemoved={afterSave}
	/>
{/if}

<style>
	.wd-grid {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: 16px;
		padding-top: 16px;
	}

	.wd-section {
		display: flex;
		flex-direction: column;
		gap: 12px;
		padding-top: 20px;
	}

	.wd-section-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 12px;
	}

	.wd-section-head h3 {
		margin: 0;
		font-size: 14px;
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.wd-notice {
		padding: 14px 16px;
		border: 1px dashed var(--color-border);
		border-radius: 12px;
		color: var(--color-text-muted);
		background: color-mix(in srgb, var(--color-border) 10%, transparent);
	}

	.wd-notice--error {
		border-color: var(--color-error-border);
		color: var(--color-error);
		background: var(--color-error-tint);
	}

	.wd-skel {
		height: 220px;
		border-radius: 12px;
		background: var(--color-surface-2, rgba(255, 255, 255, 0.03));
		animation: wd-pulse 1.4s ease-in-out infinite;
	}

	@keyframes wd-pulse {
		0%, 100% { opacity: 0.4; }
		50% { opacity: 0.7; }
	}

	@media (max-width: 640px) {
		.wd-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
