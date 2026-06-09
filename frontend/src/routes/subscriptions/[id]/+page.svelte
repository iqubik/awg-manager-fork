<script lang="ts">
	import { page } from '$app/stores';
	import { onMount, onDestroy } from 'svelte';
	import { api } from '$lib/api/client';
	import type { Subscription } from '$lib/types';
	import { PageContainer, LoadingSpinner } from '$lib/components/layout';
	import { Tabs, LayoutViewToggle, BackLink } from '$lib/components/ui';
	import SubscriptionMembersTab from '$lib/components/subscriptions/SubscriptionMembersTab.svelte';
	import SubscriptionSettingsTab from '$lib/components/subscriptions/SubscriptionSettingsTab.svelte';
	import { usageLevel } from '$lib/stores/settings';
	import {
		SINGBOX_LAYOUT_STORAGE_KEY,
		parseSingboxLayoutMode,
		readTunnelMobileLayout,
		subscribeTunnelMobileLayout,
		type SingboxLayoutMode,
	} from '$lib/constants/singboxLayout';
	import { isMockDevMode } from '$lib/env';

	// Poll Clash for the live "now" pointer this often when on members tab in urltest
	// mode. 5s balances responsiveness with Clash API load.
	const URLTEST_POLL_MS = 5000;

	// Show explicit progress bar only for subscriptions where the per-member
	// stream is long enough to be worth the visual. Below this threshold the
	// generic spinner suffices because total render is sub-second.
	const PROGRESS_BAR_THRESHOLD = 5;

	const id = $derived($page.params.id ?? '');
	let subscription = $state<Subscription | null>(null);
	let loading = $state(true);
	let error = $state('');
	let progressTotal = $state(0);
	let progressLoaded = $state(0);

	let active = $state<'members' | 'settings'>('members');
	let membersAutoDelayCheckNonce = $state(0);
	let liveActiveMember = $state<string | null>(null);
	let currentSubscriptionSurface = '';
	let subscriptionSurfaceEntryNonce = $state(0);
	let lastAutoDelayCheckKey = '';

	let singboxLayoutMode = $state<SingboxLayoutMode>('compact');
	let singboxLayoutReady = false;
	let isSingboxMembersMobile = $state(readTunnelMobileLayout());
	const showSingboxListOption = $derived($usageLevel !== 'basic');
	const singboxEffectiveLayout = $derived.by((): SingboxLayoutMode => {
		if (isSingboxMembersMobile || (!showSingboxListOption && singboxLayoutMode === 'list')) {
			return 'compact';
		}
		// Members tab has no dense cards — same grid as compact.
		if (singboxLayoutMode === 'dense') return 'compact';
		return singboxLayoutMode;
	});
	const showSingboxLayoutPicker = $derived(!isSingboxMembersMobile);
	const showSingboxGridListToggle = $derived(showSingboxListOption && showSingboxLayoutPicker);

	let evtSrc: EventSource | null = null;

	function patchSubscriptionEnabled(nextEnabled: boolean): void {
		if (subscription) {
			subscription = { ...subscription, enabled: nextEnabled };
		}
	}

	function loadStream(): void {
		if (!id) return;
		const isMockDev = isMockDevMode();
		progressLoaded = 0;
		progressTotal = 0;
		loading = true;
		error = '';
		subscription = null;
		evtSrc?.close();
		if (isMockDev) {
			void (async () => {
				try {
					const sub = await api.getSubscription(id);
					subscription = sub;
					progressTotal = sub.memberTags?.length ?? sub.members?.length ?? 0;
					progressLoaded = progressTotal;
				} catch {
					error = 'Не удалось загрузить подписку';
				} finally {
					loading = false;
				}
			})();
			return;
		}
		evtSrc = new EventSource(
			`/api/singbox/subscriptions/get-stream?id=${encodeURIComponent(id)}`,
		);
		// Guard against onerror firing right after a clean done — browser emits
		// onerror on the closed connection, but we treat that as success.
		let streamDone = false;
		let fallbackTried = false;

		evtSrc.addEventListener('meta', (e) => {
			const meta = JSON.parse((e as MessageEvent).data);
			subscription = {
				...meta,
				members: [],
				memberTags: [],
				orphanTags: [],
				rejectedMembers: meta.rejectedMembers ?? [],
				infoItems: meta.infoItems ?? [],
				activeMember: '',
			} as Subscription;
			progressTotal = meta.total ?? 0;
		});

		evtSrc.addEventListener('member', (e) => {
			if (!subscription) return;
			const { member } = JSON.parse((e as MessageEvent).data);
			subscription.members = [...(subscription.members ?? []), member];
			subscription.memberTags = [...subscription.memberTags, member.tag];
			progressLoaded += 1;
		});

		evtSrc.addEventListener('done', (e) => {
			if (subscription) {
				const data = JSON.parse((e as MessageEvent).data);
				subscription.orphanTags = data.orphanTags ?? [];
				subscription.activeMember = data.activeMember ?? '';
				subscription.rejectedMembers = data.rejectedMembers ?? subscription.rejectedMembers ?? [];
				subscription.infoItems = data.infoItems ?? subscription.infoItems ?? [];
			}
			streamDone = true;
			loading = false;
			evtSrc?.close();
			evtSrc = null;
		});

		evtSrc.onerror = async () => {
			if (streamDone) return; // already completed cleanly — ignore connection-close error
			// Prism mock backend does not emulate SSE streaming events reliably.
			// Fall back to the regular subscription GET so local mock UI remains usable.
			if (isMockDev && !fallbackTried && progressLoaded === 0) {
				fallbackTried = true;
				try {
					const sub = await api.getSubscription(id);
					subscription = sub;
					progressTotal = sub.memberTags?.length ?? sub.members?.length ?? 0;
					progressLoaded = progressTotal;
					loading = false;
					error = '';
					evtSrc?.close();
					evtSrc = null;
					return;
				} catch {
					// Keep default error handling below.
				}
			}
			// Browser fires onerror on connection drop. Surface partial state
			// if we got members, generic error otherwise.
			if (progressLoaded > 0 && progressTotal > 0) {
				error = `Загружено ${progressLoaded} из ${progressTotal} серверов. Соединение прервалось.`;
			} else {
				error = 'Не удалось загрузить подписку';
			}
			loading = false;
			evtSrc?.close();
			evtSrc = null;
		};
	}

	onMount(() => {
		const sb = localStorage.getItem(SINGBOX_LAYOUT_STORAGE_KEY);
		const parsed = parseSingboxLayoutMode(sb);
		if (parsed) singboxLayoutMode = parsed;
		singboxLayoutReady = true;
		loadStream();
	});

	onMount(() => subscribeTunnelMobileLayout((mobile) => {
		isSingboxMembersMobile = mobile;
	}));

	onDestroy(() => {
		evtSrc?.close();
		evtSrc = null;
	});

	$effect(() => {
		const surface = `${id}:${active}`;
		if (surface === currentSubscriptionSurface) return;
		currentSubscriptionSurface = surface;
		subscriptionSurfaceEntryNonce += 1;
	});

	// Poll Clash live "now" every 5s for urltest mode. Selector mode doesn't need
	// this — its activeMember is whatever the user picked and Clash echoes it.
	$effect(() => {
		const sub = subscription;
		if (loading || error || !sub || sub.mode !== 'urltest' || active !== 'members') {
			liveActiveMember = null;
			return;
		}
		let cancelled = false;
		const tick = async (): Promise<void> => {
			try {
				const res = await api.getSubscriptionActiveNow(sub.id);
				if (!cancelled) liveActiveMember = res.now || null;
			} catch {
				if (!cancelled) liveActiveMember = null;
			}
		};
		void tick();
		const handle = setInterval(() => void tick(), URLTEST_POLL_MS);
		return () => {
			cancelled = true;
			clearInterval(handle);
		};
	});

	$effect(() => {
		const sub = subscription;
		const entryNonce = subscriptionSurfaceEntryNonce;

		if (loading || error || !sub || active !== 'members') return;

		const tags = (sub.members && sub.members.length > 0 ? sub.members.map((m) => m.tag) : sub.memberTags)
			.filter(Boolean)
			.sort()
			.join(',');

		if (!tags) return;

		const key = `${entryNonce}:${sub.id}:${tags}`;
		if (key === lastAutoDelayCheckKey) return;

		lastAutoDelayCheckKey = key;
		membersAutoDelayCheckNonce += 1;
	});

	$effect(() => {
		if (!singboxLayoutReady) return;
		localStorage.setItem(SINGBOX_LAYOUT_STORAGE_KEY, singboxLayoutMode);
	});
</script>

<svelte:head>
	<title>{subscription?.label ?? 'Подписка'} - AWG Manager</title>
</svelte:head>

<PageContainer width="wide">
	{#if !subscription && loading}
		<!-- Initial spinner before meta arrives (any subscription size) -->
		<div class="loading-centered">
			<LoadingSpinner size="md" message="Загружаем подписку..." />
		</div>
	{:else if !subscription && error}
		<div class="err">{error}</div>
	{:else if subscription}
		<div class="subscription-header">
			<div class="subscription-header-left">
				<BackLink href="/?tab=subscriptions" variant="accent" />
				<h1 class="subscription-title">{subscription.label || subscription.url}</h1>
			</div>
		</div>
		<Tabs
			tabs={[
				{ id: 'members', label: `Серверы (${subscription.memberTags.length})` },
				{ id: 'settings', label: 'Настройки' },
			]}
			active={active}
			onchange={(tabId) => (active = tabId as 'members' | 'settings')}
			urlParam="tab"
			defaultTab="members"
		/>
		{#if loading && progressTotal > PROGRESS_BAR_THRESHOLD}
			<div class="loading-progress">
				<div class="progress-text">
					Загружено {progressLoaded} из {progressTotal} серверов
				</div>
				<div class="progress-bar">
					<div class="progress-fill" style="width: {(progressLoaded / progressTotal) * 100}%"></div>
				</div>
			</div>
		{/if}
		{#if error}
			<div class="err">{error}</div>
		{/if}
		<section class="content">
			{#if active === 'members'}
				{#if subscription.memberTags.length > 0 && showSingboxLayoutPicker}
					<div class="members-toolbar">
						<LayoutViewToggle
							value={singboxLayoutMode}
							showListOption={showSingboxGridListToggle}
							showDenseOption={false}
							onchange={(v) => (singboxLayoutMode = v)}
						/>
					</div>
				{/if}
				<SubscriptionMembersTab
					{subscription}
					{liveActiveMember}
					onUpdated={loadStream}
					autoDelayCheckNonce={membersAutoDelayCheckNonce}
					layout={singboxEffectiveLayout}
				/>
			{:else}
				<div class="edit-wrapper">
					<SubscriptionSettingsTab
						{subscription}
						onUpdated={loadStream}
						onEnabledChanged={patchSubscriptionEnabled}
					/>
				</div>
			{/if}
		</section>
	{/if}
</PageContainer>

<style>
	.err { color: #f85149; margin-top: 1rem; }
	.content { margin-top: 1rem; }
	.subscription-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 1rem;
		padding-bottom: 0.75rem;
		border-bottom: 1px solid var(--color-border);
	}

	.subscription-header-left {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		min-width: 0;
	}

	.subscription-title {
		min-width: 0;
		margin: 0;
		font-size: 1.25rem;
		font-weight: 600;
		line-height: 1.2;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.members-toolbar {
		display: flex;
		justify-content: flex-end;
		margin-bottom: 0.75rem;
	}

	@media (max-width: 760px) {
		.members-toolbar {
			display: none;
		}
	}

	@media (max-width: 640px) {
		.subscription-header {
			align-items: stretch;
		}

		.subscription-header-left {
			flex-wrap: wrap;
			gap: 0.625rem;
		}

		.subscription-title {
			font-size: 1.125rem;
		}
	}

	.loading-progress {
		margin: 1rem 0;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.progress-text {
		font-size: 0.9rem;
		color: var(--color-text-muted);
		text-align: center;
	}
	.progress-bar {
		height: 6px;
		background: var(--color-bg-tertiary);
		border-radius: 3px;
		overflow: hidden;
	}
	.progress-fill {
		height: 100%;
		background: var(--color-accent);
		transition: width 200ms ease-out;
	}
</style>
