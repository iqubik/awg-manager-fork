<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import {
		deviceProxyConfig,
		deviceProxyInstances,
		deviceProxyOutbounds,
		deviceProxyRuntime,
		deviceProxyMissingTarget,
	} from '$lib/stores/deviceproxy';
	import { configFromInstance, mergeInstanceConfig, newDeviceProxyInstance } from '$lib/utils/deviceProxyInstance';
	import { SideDrawer, Button } from '$lib/components/ui';
	import ActiveTunnelCard from '$lib/components/deviceproxy/ActiveTunnelCard.svelte';
	import SettingsCard from '$lib/components/deviceproxy/SettingsCard.svelte';
	import DeviceProxyStatRow from '$lib/components/deviceproxy/DeviceProxyStatRow.svelte';
	import DeviceProxyClientInfoCard from '$lib/components/deviceproxy/DeviceProxyClientInfoCard.svelte';
	import { api } from '$lib/api/client';
	import { notifications } from '$lib/stores/notifications';
	import type {
		DeviceProxyConfig,
		DeviceProxyInstance,
		DeviceProxyInstanceIPCheckResult,
		DeviceProxyRuntime
	} from '$lib/types';

	interface ListenChoices {
		lanIP: string;
		bridges: { id: string; label: string; ip: string }[];
		singboxRunning: boolean;
	}

	type RuntimeById = Record<string, DeviceProxyRuntime>;
	type ToggleById = Record<string, boolean>;
	type CollapsedById = Record<string, boolean>;
	type ExternalIPById = Record<string, DeviceProxyInstanceIPCheckResult | null>;
	type ExternalIPErrorById = Record<string, string>;
	type ExternalIPLoadingById = Record<string, boolean>;
	type ExternalIPCheckedAtById = Record<string, number>;

	let unsubConfig: (() => void) | null = null;
	let unsubInstances: (() => void) | null = null;
	let unsubOutbounds: (() => void) | null = null;
	let unsubRuntime: (() => void) | null = null;

	let choices = $state<ListenChoices | null>(null);
	let settingsDrawerOpen = $state(false);
	let selectedInstanceId = $state<string>('default');
	let runtimes = $state<RuntimeById>({});
	let toggling = $state<ToggleById>({});
	let deletingId = $state<string | null>(null);
	let runtimeIdsKey = $state('');
	let runtimeRefreshKey = $state('');
	let externalIPInitKey = $state('');
	let externalIPTimers = $state<ReturnType<typeof setTimeout>[]>([]);
	let collapsedById = $state<CollapsedById>({});
	let externalIPById = $state<ExternalIPById>({});
	let externalIPErrorById = $state<ExternalIPErrorById>({});
	let externalIPLoadingById = $state<ExternalIPLoadingById>({});
	let externalIPCheckedAtById = $state<ExternalIPCheckedAtById>({});
	const collapseStorageKey = 'deviceProxyCollapsedById';

	onMount(() => {
		// Keep legacy stores subscribed while frontend is being migrated.
		unsubConfig = deviceProxyConfig.subscribe(() => {});
		unsubInstances = deviceProxyInstances.subscribe(() => {});
		unsubOutbounds = deviceProxyOutbounds.subscribe(() => {});
		unsubRuntime = deviceProxyRuntime.subscribe(() => {});
		api.getDeviceProxyListenChoices().then((v) => {
			choices = v;
		}).catch(() => {});

		try {
			const raw = localStorage.getItem(collapseStorageKey);
			if (raw) {
				const parsed = JSON.parse(raw) as CollapsedById;
				if (parsed && typeof parsed === 'object') {
					collapsedById = parsed;
				}
			}
		} catch {
			// Ignore storage errors.
		}
	});

	onDestroy(() => {
		unsubConfig?.();
		unsubInstances?.();
		unsubOutbounds?.();
		unsubRuntime?.();
		for (const timer of externalIPTimers) clearTimeout(timer);
	});

	const instancesSnap = $derived($deviceProxyInstances);
	const outboundsSnap = $derived($deviceProxyOutbounds);
	const runtimeSnap = $derived($deviceProxyRuntime);

	const instances = $derived<DeviceProxyInstance[]>(instancesSnap.data ?? []);
	const outbounds = $derived(outboundsSnap.data ?? []);
	const missingTag = $derived($deviceProxyMissingTarget);

	const noTunnels = $derived(outbounds.length <= 1);

	const selectedInstance = $derived.by(() => {
		return instances.find((in_) => in_.id === selectedInstanceId) ?? null;
	});

	const bridgeInterfaces = $derived(
		(choices?.bridges ?? [{ id: 'Bridge0', label: 'Bridge0' }]).map((b) => ({ id: b.id, label: b.label })),
	);

	$effect(() => {
		const ids = instances.map((in_) => in_.id).sort();
		const nextKey = ids.join('|');
		if (nextKey === runtimeIdsKey) return;

		runtimeIdsKey = nextKey;
		for (const id of ids) {
			void refreshRuntime(id);
		}
	});

	$effect(() => {
		const ready = instances
			.filter((in_) => {
				const rt = runtimeFor(in_.id);
				const tag = rt.activeTag || rt.defaultTag || in_.selectedOutbound;
				return in_.enabled && rt.alive && !!tag;
			})
			.map((in_) => {
				const rt = runtimeFor(in_.id);
				const tag = rt.activeTag || rt.defaultTag || in_.selectedOutbound;
				return `${in_.id}:${in_.port}:${tag}`;
			})
			.sort();
		const nextKey = ready.join('|');
		if (!nextKey) {
			externalIPInitKey = '';
			return;
		}
		if (nextKey === externalIPInitKey) return;
		externalIPInitKey = nextKey;
		for (const in_ of instances) {
			const rt = runtimeFor(in_.id);
			const tag = rt.activeTag || rt.defaultTag || in_.selectedOutbound;
			if (in_.enabled && rt.alive && tag) {
				scheduleExternalIPAutoCheck(in_.id);
			}
		}
	});

	$effect(() => {
		const nextKey = String(runtimeSnap.lastFetchedAt);
		if (nextKey === runtimeRefreshKey) return;

		runtimeRefreshKey = nextKey;
		if (runtimeSnap.lastFetchedAt === 0 || instances.length === 0) return;

		void refreshAllRuntimes();
	});

	function runtimeFor(id: string): DeviceProxyRuntime {
		return runtimes[id] ?? { alive: false, activeTag: '', defaultTag: '' };
	}

	async function refreshRuntime(id: string) {
		try {
			const runtime = await api.getDeviceProxyInstanceRuntime(id);
			runtimes = { ...runtimes, [id]: runtime };
		} catch {
			// Instance may have been deleted between list refresh and runtime request.
		}
	}

	async function refreshAllRuntimes() {
		for (const in_ of instances) {
			await refreshRuntime(in_.id);
		}
	}

	function externalIPFor(id: string): DeviceProxyInstanceIPCheckResult | null {
		return externalIPById[id] ?? null;
	}

	function externalIPErrorFor(id: string): string {
		return externalIPErrorById[id] ?? '';
	}

	function externalIPLoadingFor(id: string): boolean {
		return !!externalIPLoadingById[id];
	}

	function externalIPCheckedAtFor(id: string): number | null {
		return externalIPCheckedAtById[id] ?? null;
	}

	function scheduleExternalIPAutoCheck(id: string, delay = 1800) {
		const timer = setTimeout(() => {
			externalIPTimers = externalIPTimers.filter((t) => t !== timer);
			const in_ = instances.find((x) => x.id === id);
			const rt = runtimeFor(id);
			const tag = rt.activeTag || rt.defaultTag || in_?.selectedOutbound;
			if (!in_?.enabled || !rt.alive || !tag) return;
			void refreshExternalIP(id, { silentAuto: true });
		}, delay);
		externalIPTimers = [...externalIPTimers, timer];
	}

	function humanExternalIPError(message: string): string {
		const lower = message.toLowerCase();
		if (
			lower.includes('failed to get ip through proxy') ||
			lower.includes('connection refused') ||
			lower.includes('request failed') ||
			lower.includes('socks')
		) {
			return 'Прокси ещё не ответил. Подождите несколько секунд и нажмите «Проверить».';
		}
		if (lower.includes('sing-box') || lower.includes('singbox')) {
			return 'Sing-box ещё не готов. Подождите несколько секунд и повторите проверку.';
		}
		return 'Не удалось проверить внешний IP через прокси.';
	}

	async function refreshExternalIP(
		id: string,
		opts: { silentAuto?: boolean } = {},
	) {
		externalIPLoadingById = { ...externalIPLoadingById, [id]: true };
		if (!opts.silentAuto) {
			externalIPErrorById = { ...externalIPErrorById, [id]: '' };
		}
		try {
			const data = await api.checkDeviceProxyInstanceExternalIP(id);
			externalIPById = { ...externalIPById, [id]: data };
			externalIPCheckedAtById = { ...externalIPCheckedAtById, [id]: Date.now() };
		} catch (e) {
			externalIPById = { ...externalIPById, [id]: null };
			externalIPCheckedAtById = { ...externalIPCheckedAtById, [id]: 0 };
			if (opts.silentAuto) {
				externalIPErrorById = { ...externalIPErrorById, [id]: '' };
			} else {
				externalIPErrorById = { ...externalIPErrorById, [id]: humanExternalIPError((e as Error).message) };
			}
		} finally {
			externalIPLoadingById = { ...externalIPLoadingById, [id]: false };
		}
	}

	async function refreshExternalIPAfterApply(id: string) {
		await refreshExternalIP(id);
		if (externalIPErrorFor(id)) {
			await new Promise((resolve) => setTimeout(resolve, 1200));
			await refreshExternalIP(id);
		}
	}

	function bridgeLabelFor(config: DeviceProxyConfig): string {
		if (!choices) return '';
		const match = choices.bridges.find((b) => b.id === config.listenInterface);
		return match?.label ?? config.listenInterface;
	}

	function resolvedListenIPFor(config: DeviceProxyConfig): string {
		if (!choices) return '';
		if (config.listenAll) return choices.lanIP || '';
		const match = choices.bridges.find((b) => b.id === config.listenInterface);
		return match?.ip ?? '';
	}

	function activeLabelFor(runtime: DeviceProxyRuntime): string {
		return runtime.activeTag || runtime.defaultTag;
	}

	function upsertInstanceCache(saved: DeviceProxyInstance) {
		const exists = instances.some((in_) => in_.id === saved.id);
		const next = exists
			? instances.map((in_) => (in_.id === saved.id ? saved : in_))
			: [...instances, saved];
		deviceProxyInstances.applyMutationResponse(next);
	}

	function removeInstanceCache(id: string) {
		const next = instances.filter((in_) => in_.id !== id);
		deviceProxyInstances.applyMutationResponse(next);
	}


	async function createInstance() {
		const in_ = newDeviceProxyInstance(instances);
		try {
			const saved = await api.saveDeviceProxyInstance(in_);
			upsertInstanceCache(saved);
			selectedInstanceId = saved.id;
			settingsDrawerOpen = true;
			deviceProxyConfig.invalidate();
			deviceProxyRuntime.invalidate();
			await refreshRuntime(saved.id);
			notifications.success('Прокси создан');
		} catch (e) {
			notifications.error(`Не удалось создать прокси: ${(e as Error).message}`);
		}
	}

	async function deleteInstance(in_: DeviceProxyInstance) {
		if (in_.id === 'default') return;
		if (!confirm(`Удалить "${in_.name || in_.id}"?`)) return;

		deletingId = in_.id;
		try {
			await api.deleteDeviceProxyInstance(in_.id);
			const remaining = instances.filter((x) => x.id !== in_.id);
			removeInstanceCache(in_.id);
			selectedInstanceId = remaining[0]?.id ?? 'default';
			deviceProxyConfig.invalidate();
			deviceProxyRuntime.invalidate();
			notifications.success('Прокси удалён');
		} catch (e) {
			notifications.error(`Не удалось удалить: ${(e as Error).message}`);
		} finally {
			deletingId = null;
		}
	}

	async function handleToggleEnabled(in_: DeviceProxyInstance) {
		if (toggling[in_.id]) return;

		toggling = { ...toggling, [in_.id]: true };
		const next = !in_.enabled;
		try {
			const saved = await api.saveDeviceProxyInstance({ ...in_, enabled: next });
			upsertInstanceCache(saved);
			deviceProxyConfig.invalidate();
			deviceProxyRuntime.invalidate();
			await refreshRuntime(in_.id);
			notifications.success(next ? 'Прокси включён' : 'Прокси выключен');
		} catch (e) {
			notifications.error(`Не удалось переключить: ${(e as Error).message}`);
		} finally {
			toggling = { ...toggling, [in_.id]: false };
		}
	}

	function openSettings(in_: DeviceProxyInstance) {
		selectedInstanceId = in_.id;
		settingsDrawerOpen = true;
	}

	function handleSavedInstance(saved: DeviceProxyInstance) {
		upsertInstanceCache(saved);
		selectedInstanceId = saved.id;
		deviceProxyConfig.invalidate();
		deviceProxyRuntime.invalidate();
		void refreshRuntime(saved.id);
		settingsDrawerOpen = false;
	}

	async function saveSelectedConfig(cfg: DeviceProxyConfig): Promise<DeviceProxyConfig> {
		if (!selectedInstance) throw new Error('Прокси не выбран');
		const saved = await api.saveDeviceProxyInstance(mergeInstanceConfig(selectedInstance, cfg));
		handleSavedInstance(saved);
		return configFromInstance(saved);
	}

	async function selectRuntime(in_: DeviceProxyInstance, tag: string) {
		await api.selectDeviceProxyInstanceRuntime(in_.id, tag);
		await refreshRuntime(in_.id);
		deviceProxyRuntime.invalidate();
	}

	async function applyNow(in_: DeviceProxyInstance) {
		const runtime = runtimeFor(in_.id);
		const active = runtime.activeTag || runtime.defaultTag;
		if (active && active !== in_.selectedOutbound) {
			const saved = await api.saveDeviceProxyInstance({ ...in_, selectedOutbound: active });
			upsertInstanceCache(saved);
		}

		await api.applyDeviceProxyInstances();
		await refreshAllRuntimes();
		await refreshExternalIPAfterApply(in_.id);
		deviceProxyInstances.invalidate();
		deviceProxyConfig.invalidate();
		deviceProxyRuntime.invalidate();
	}

	function handleSwitched(in_: DeviceProxyInstance) {
		void refreshRuntime(in_.id);
		deviceProxyRuntime.invalidate();
	}

	function isCollapsed(id: string): boolean {
		return !!collapsedById[id];
	}

	function toggleCollapsed(id: string) {
		const next = { ...collapsedById, [id]: !collapsedById[id] };
		collapsedById = next;
		try {
			localStorage.setItem(collapseStorageKey, JSON.stringify(next));
		} catch {
			// Ignore storage errors.
		}
	}

</script>

{#if missingTag}
	<div class="banner banner-error">
		Прокси отключён: выбранный туннель "{missingTag}" был удалён. Проверьте настройки нужного инстанса.
	</div>
{/if}

{#if noTunnels && !missingTag}
	<div class="banner banner-info">
		Добавьте хотя бы один туннель в разделе <a href="/tunnels">Туннели</a>, чтобы направлять трафик через VPN.
	</div>
{/if}

<div class="toolbar">
	<div>
		<h2 class="page-title">Прокси</h2>
		<p class="page-desc">Можно создать несколько SOCKS5 / HTTP прокси с разными портами и outbound.</p>
	</div>
	<Button variant="primary" size="sm" onclick={createInstance}>Добавить прокси</Button>
</div>

{#if instancesSnap.status === 'loading'}
	<p>Загрузка…</p>
{:else if instances.length === 0}
	<div class="banner banner-info">
		Прокси ещё не настроены.
		<button type="button" class="link-btn" onclick={createInstance}>Создать прокси</button>
	</div>
{:else}
	<div class="instances">
		{#each instances as in_ (in_.id)}
			{@const cfg = configFromInstance(in_)}
			{@const runtime = runtimeFor(in_.id)}
			{@const bridgeLabel = bridgeLabelFor(cfg)}
			{@const resolvedListenIP = resolvedListenIPFor(cfg)}
			{@const activeLabel = activeLabelFor(runtime)}

			<section class="instance-card">
				<div class="instance-header">
					<div>
						<h3>{in_.name || in_.id}</h3>
						<div class="instance-meta">
							<span>{in_.id}</span>
							<span class="auth-meta" class:auth-meta-enabled={in_.auth.enabled}>
								{#if in_.auth.enabled}
									<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
										<rect x="4" y="11" width="16" height="9" rx="2" ry="2"/>
										<path d="M8 11V8a4 4 0 0 1 8 0v3"/>
									</svg>
									<span>с паролем</span>
								{:else}
									<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
										<rect x="4" y="11" width="16" height="9" rx="2" ry="2"/>
										<path d="M16 11V8a4 4 0 0 0-7.2-2.4"/>
									</svg>
									<span>без пароля</span>
								{/if}
							</span>
						</div>
					</div>
					<div class="instance-actions">
						<Button
							variant="ghost"
							size="sm"
							onclick={() => toggleCollapsed(in_.id)}
						>
							{isCollapsed(in_.id) ? 'Развернуть' : 'Свернуть'}
						</Button>
						<Button variant="ghost" size="sm" onclick={() => openSettings(in_)}>Настройки</Button>
						{#if in_.id !== 'default'}
							<Button
								variant="ghost"
								size="sm"
								loading={deletingId === in_.id}
								onclick={() => deleteInstance(in_)}
							>
								Удалить
							</Button>
						{/if}
					</div>
				</div>

				<DeviceProxyStatRow
					config={cfg}
					{runtime}
					{bridgeLabel}
					{activeLabel}
					toggling={!!toggling[in_.id]}
					onToggleEnabled={() => handleToggleEnabled(in_)}
				/>

				{#if !isCollapsed(in_.id) && in_.enabled}
					<div class="dashboard-grid">
						<div class="dashboard-left">
							<ActiveTunnelCard
								{outbounds}
								{runtime}
								radioName={`device-proxy-active-tunnel-${in_.id}`}
								onSwitched={() => handleSwitched(in_)}
								onSelectRuntime={(tag) => selectRuntime(in_, tag)}
								onApplyNow={() => applyNow(in_)}
							/>
						</div>
						<div class="dashboard-right">
							<DeviceProxyClientInfoCard
								config={cfg}
								{resolvedListenIP}
								{bridgeLabel}
								externalIP={externalIPFor(in_.id)}
								externalIPError={externalIPErrorFor(in_.id)}
								externalIPLoading={externalIPLoadingFor(in_.id)}
								externalIPCheckedAt={externalIPCheckedAtFor(in_.id)}
								onRefreshExternalIP={() => refreshExternalIP(in_.id)}
								onOpenSettings={() => openSettings(in_)}
							/>
						</div>
					</div>
				{:else if !isCollapsed(in_.id)}
					<div class="banner banner-info disabled-banner">
						<span>Прокси выключен.</span>
						<button type="button" class="link-btn" onclick={() => openSettings(in_)}>
							Открыть настройки
						</button>
					</div>
				{/if}
			</section>
		{/each}
	</div>

	{#if selectedInstance}
		<SideDrawer
			open={settingsDrawerOpen}
			onClose={() => (settingsDrawerOpen = false)}
			title={`Настройки: ${selectedInstance.name || selectedInstance.id}`}
			width={560}
		>
			<SettingsCard
				config={configFromInstance(selectedInstance)}
				{outbounds}
				{bridgeInterfaces}
				title={`Настройки: ${selectedInstance.name || selectedInstance.id}`}
				description="Эти значения относятся только к выбранному proxy instance."
				onSaveConfig={saveSelectedConfig}
				onSaved={() => {}}
				onCancel={() => (settingsDrawerOpen = false)}
			/>
		</SideDrawer>
	{/if}
{/if}

<style>
	.banner {
		padding: 0.75rem 1rem;
		border-radius: var(--radius);
		margin-bottom: 0.75rem;
		font-size: 0.875rem;
	}

	.banner-error {
		border: 1px solid var(--color-error);
		background: rgba(247, 118, 142, 0.08);
		color: var(--color-error);
	}

	.banner-info {
		border: 1px solid var(--color-border);
		background: var(--color-bg-secondary);
		color: var(--color-text-secondary);
	}

	.toolbar {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 1rem;
	}

	.page-title {
		margin: 0 0 0.25rem 0;
		font-size: 1.125rem;
		font-weight: 600;
	}

	.page-desc {
		margin: 0;
		font-size: 0.8125rem;
		color: var(--color-text-secondary);
	}

	.instances {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.instance-card {
		padding: 1rem;
		border: 1px solid var(--color-border);
		border-radius: var(--radius);
		background: var(--color-bg-primary);
	}

	.instance-header {
		display: flex;
		align-items: flex-start;
		justify-content: space-between;
		gap: 1rem;
		margin-bottom: 0.875rem;
	}

	.instance-header h3 {
		margin: 0 0 0.25rem 0;
		font-size: 1rem;
		font-weight: 600;
	}

	.instance-meta {
		display: flex;
		gap: 0.625rem;
		flex-wrap: wrap;
		font-family: var(--font-mono);
		font-size: 0.6875rem;
		color: var(--color-text-muted);
	}

	.auth-meta {
		display: inline-flex;
		align-items: center;
		gap: 0.25rem;
	}

	.auth-meta-enabled {
		color: var(--color-text-secondary);
	}

	.instance-actions {
		display: flex;
		gap: 0.5rem;
		flex-wrap: wrap;
		justify-content: flex-end;
	}

	.disabled-banner {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
		margin-bottom: 0;
	}

	.link-btn {
		background: none;
		border: none;
		color: var(--color-accent);
		font-size: inherit;
		font-family: inherit;
		cursor: pointer;
		padding: 0;
		text-decoration: underline;
	}

	.dashboard-grid {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 1rem;
		align-items: start;
	}

	.dashboard-left,
	.dashboard-right {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	@media (max-width: 900px) {
		.toolbar,
		.instance-header {
			flex-direction: column;
		}

		.instance-actions {
			justify-content: flex-start;
		}

		.dashboard-grid {
			grid-template-columns: 1fr;
		}
	}
</style>
