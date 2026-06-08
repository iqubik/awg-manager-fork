<script lang="ts">
	import { onMount } from 'svelte';
	import type { ManagedServer, ManagedPeer, ManagedPeerStats, ManagedServerStats, ASCParams } from '$lib/types';
	import { api } from '$lib/api/client';
	import { notifications } from '$lib/stores/notifications';
	import { servers } from '$lib/stores/servers';
	import { formatBytes } from '$lib/utils/format';
	import { EarthLock, Plus, RefreshCw, Settings, Trash2 } from 'lucide-svelte';
	import { Toggle, Button, Dropdown, ChipMultiSelect, VersionBadge, type DropdownOption } from '$lib/components/ui';
	import {
		EditManagedServerModal,
		AddManagedPeerModal,
		EditManagedPeerModal,
		PeerConfModal,
		PeerSortControls,
		ManagedPeerTable,
	} from '$lib/components/servers';
	import { comparePeerFieldsDirected } from '$lib/utils/peerSort';
	import { peerSort } from '$lib/stores/peerSort';
	import { isStandardAccessPolicyName } from '$lib/utils/accessPolicy';
	import { classifyAwgVersionFromAsc } from '$lib/utils/classifyAwgVersion';

	interface Props {
		server: ManagedServer;
		stats: ManagedServerStats | null;
		routerIP?: string;
		onDeleted?: () => void;
		onUpdated?: () => void;
		onOpenASC: () => void;
		ingressEnabled?: boolean;
		onToggleIngress?: (interfaceName: string, enabled: boolean) => Promise<void>;
		lanSegmentOptions?: { value: string; label: string }[];
	}

	let { server, stats, routerIP = '', onDeleted = () => {}, onUpdated = () => {}, onOpenASC, ingressEnabled = false, onToggleIngress = async () => {}, lanSegmentOptions = [] }: Props = $props();

	let serverId = $derived(server.interfaceName);

	let serverDisplayName = $derived(server.description || server.interfaceName);

	let editServerOpen = $state(false);
	let addPeerOpen = $state(false);
	let editPeerOpen = $state(false);
	let confModalOpen = $state(false);
	let selectedPeer = $state<ManagedPeer | null>(null);
	let confPubkey = $state('');
	let confPeerName = $state('');
	let deleting = $state(false);
	let confirmDelete = $state(false);
	let confirmDeletePeerKey = $state<string | null>(null);

	let searchQuery = $state('');

	function getPeerStats(publicKey: string): ManagedPeerStats | undefined {
		return stats?.peers?.find(p => p.publicKey === publicKey);
	}

	let sortedPeers = $derived.by(() => {
		let peers = server.peers ?? [];

		// Filter (only when search is rendered: 5+ peers)
		if (searchQuery && peers.length >= 5) {
			const q = searchQuery.toLowerCase();
			peers = peers.filter(p =>
				(p.description || '').toLowerCase().includes(q) ||
				p.tunnelIP.toLowerCase().includes(q)
			);
		}

		const sortBy = $peerSort.sortBy;
		if (sortBy === null) return peers;

		const sorted = [...peers].sort((a, b) => {
			const sa = getPeerStats(a.publicKey);
			const sb = getPeerStats(b.publicKey);
			return comparePeerFieldsDirected(
				{
					name: a.description || a.publicKey,
					ip: a.tunnelIP,
					endpoint: sa?.endpoint || '-',
					rxBytes: sa?.rxBytes ?? null,
					txBytes: sa?.txBytes ?? null,
					online: sa?.online ?? null,
					lastHandshake: sa?.lastHandshake ?? null,
				},
				{
					name: b.description || b.publicKey,
					ip: b.tunnelIP,
					endpoint: sb?.endpoint || '-',
					rxBytes: sb?.rxBytes ?? null,
					txBytes: sb?.txBytes ?? null,
					online: sb?.online ?? null,
					lastHandshake: sb?.lastHandshake ?? null,
				},
				sortBy,
				$peerSort.sortAsc,
			);
		});

		return sorted;
	});

	let onlineCount = $derived(stats?.peers?.filter(p => p.online).length ?? 0);
	let isUp = $derived(stats?.status === 'up');
	let totalRx = $derived(stats?.peers?.reduce((sum, p) => sum + p.rxBytes, 0) ?? 0);
	let totalTx = $derived(stats?.peers?.reduce((sum, p) => sum + p.txBytes, 0) ?? 0);

	async function handleDeleteServer() {
		if (!confirmDelete) {
			confirmDelete = true;
			setTimeout(() => { confirmDelete = false; }, 3000);
			return;
		}
		deleting = true;
		try {
			const fresh = await api.deleteManagedServer(serverId);
			servers.applyMutationResponse(fresh);
			notifications.success('Сервер удалён');
			onDeleted();
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Ошибка удаления');
		} finally {
			deleting = false;
			confirmDelete = false;
		}
	}

	async function handleTogglePeer(peer: ManagedPeer) {
		try {
			const fresh = await api.toggleManagedPeer(serverId, peer.publicKey, !peer.enabled);
			servers.applyMutationResponse(fresh);
			onUpdated();
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Ошибка');
		}
	}

	function handleDeletePeerClick(peer: ManagedPeer) {
		if (confirmDeletePeerKey === peer.publicKey) {
			doDeletePeer(peer);
		} else {
			confirmDeletePeerKey = peer.publicKey;
			setTimeout(() => {
				if (confirmDeletePeerKey === peer.publicKey) {
					confirmDeletePeerKey = null;
				}
			}, 3000);
		}
	}

	async function doDeletePeer(peer: ManagedPeer) {
		try {
			confirmDeletePeerKey = null;
			const fresh = await api.deleteManagedPeer(serverId, peer.publicKey);
			servers.applyMutationResponse(fresh);
			notifications.success('Клиент удалён');
			onUpdated();
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Ошибка удаления');
		}
	}

	function openEditPeer(peer: ManagedPeer) {
		selectedPeer = peer;
		editPeerOpen = true;
	}

	function maskToPrefix(mask: string): string {
		if (/^\d+$/.test(mask)) return mask;
		const parts = mask.split('.').map(Number);
		let bits = 0;
		for (const p of parts) {
			bits += (p >>> 0).toString(2).split('1').length - 1;
		}
		return String(bits);
	}

	/** Подсеть вида 10.0.0.X/24 — host-октеты заменены на X по маске. */
	function formatSubnetPlaceholder(address: string, mask: string): string {
		const prefix = Number(maskToPrefix(mask));
		const octets = address.split('.').map((p) => parseInt(p, 10));
		if (octets.length !== 4 || octets.some((n) => Number.isNaN(n)) || Number.isNaN(prefix)) {
			return `${address}/${maskToPrefix(mask)}`;
		}
		const parts = octets.map((value, i) => {
			const bitsBefore = i * 8;
			if (prefix <= bitsBefore) return 'X';
			if (prefix >= bitsBefore + 8) return String(value);
			return 'X';
		});
		return `${parts.join('.')}/${prefix}`;
	}

	const WAN_IP_MASKED = 'показать';

	let wanIP = $state('');
	let showWanIP = $state(false);
	let lanRouterLabel = $derived(routerIP ? ` (${routerIP})` : '');
	let vpnSubnetLabel = $derived(formatSubnetPlaceholder(server.address, server.mask));

	let togglingEnabled = $state(false);
	let restartingServer = $state(false);

	async function handleToggleEnabled() {
		togglingEnabled = true;
		try {
			const fresh = await api.setManagedServerEnabled(serverId, !isUp);
			servers.applyMutationResponse(fresh);
			onUpdated();
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Ошибка переключения');
		} finally {
			togglingEnabled = false;
		}
	}

	async function handleRestartOrStart() {
		if (restartingServer) return;
		restartingServer = true;

		try {
			await api.restartManagedServer(serverId);
			notifications.success(isUp ? 'Команда рестарта отправлена' : 'Команда запуска отправлена');
			servers.invalidate();
		} catch {
			notifications.warning('Команда могла быть отправлена, соединение могло временно прерваться');
		} finally {
			restartingServer = false;
		}
	}

	let togglingNAT = $state(false);
	let togglingIngress = $state(false);

	let natMode = $derived<'full' | 'internet-only' | 'none'>(
		server.natMode ?? (server.natEnabled ? 'full' : 'none')
	);

	const natModeOptions: DropdownOption<'full' | 'internet-only' | 'none'>[] = [
		{ value: 'full', label: 'Полный NAT' },
		{ value: 'internet-only', label: 'NAT только для интернета' },
		{ value: 'none', label: 'Без NAT' },
	];

	async function handleToggleIngress() {
		togglingIngress = true;
		try {
			await onToggleIngress(server.interfaceName, !ingressEnabled);
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Ошибка переключения egress в sing-box');
		} finally {
			togglingIngress = false;
		}
	}

	async function handleSetNATMode(mode: 'full' | 'internet-only' | 'none') {
		if (mode === natMode) return;
		togglingNAT = true;
		try {
			const fresh = await api.setManagedServerNATMode(serverId, mode);
			servers.applyMutationResponse(fresh);
			onUpdated();
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Ошибка изменения режима NAT');
		} finally {
			togglingNAT = false;
		}
	}

	let settingLAN = $state(false);
	async function handleSetLANSegments(next: string[]) {
		if (settingLAN) return;
		settingLAN = true;
		try {
			const fresh = await api.setManagedServerLANSegments(serverId, next);
			servers.applyMutationResponse(fresh);
			onUpdated();
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Ошибка изменения доступа в LAN');
		} finally { settingLAN = false; }
	}

	function openConf(peer: ManagedPeer) {
		confPubkey = peer.publicKey;
		confPeerName = peer.description || 'peer';
		confModalOpen = true;
	}

	let policies = $state<{ id: string; description: string }[]>([]);
	let policyChanging = $state(false);
	// Local mirror of server.policy for the <select>. On error we reset
	// it back to server.policy so the DOM reverts — without this the
	// browser keeps the failed value because no fresh snapshot arrives.
	// Empty initial value is overwritten by the $effect on mount before
	// the select is interactive.
	let selectedPolicy = $state('');
	let ascParams = $state<ASCParams | null>(null);
	let ascLoadedFor = $state('');

	$effect(() => {
		selectedPolicy = server.policy;
	});

	$effect(() => {
		const id = server.interfaceName;
		if (ascLoadedFor === id) return;

		if (ascLoadedFor && ascLoadedFor !== id) {
			ascParams = null;
			ascLoadedFor = '';
		}

		let cancelled = false;

		void (async () => {
			try {
				const params = await api.getManagedServerASC(id);
				if (!cancelled) {
					ascParams = params;
					ascLoadedFor = id;
				}
			} catch {
				if (!cancelled) {
					ascParams = null;
					ascLoadedFor = '';
				}
			}
		})();

		return () => {
			cancelled = true;
		};
	});

	let awgVersion = $derived(classifyAwgVersionFromAsc(ascParams));

	onMount(async () => {
		void api.getWANIP().then((ip) => { wanIP = ip; }).catch(() => {});
		try {
			policies = await api.getManagedServerPolicies();
		} catch {
			policies = [];
		}
	});

	let orphanedPolicy = $derived.by(() => {
		const p = server.policy;
		if (!p || p === 'none' || p === 'permit' || p === 'deny') return null;
		if (policies.some(o => o.id === p)) return null;
		return p;
	});

	let standardPolicies = $derived(policies.filter((p) => isStandardAccessPolicyName(p.id)));

	let policyOptions = $derived<DropdownOption[]>([
		{ value: 'none', label: 'Политика по умолчанию' },
		...(orphanedPolicy ? [{ value: orphanedPolicy, label: `${orphanedPolicy} (отсутствует)` }] : []),
		...standardPolicies.map((p) => ({
			value: p.id,
			label: p.description ? `${p.id} — ${p.description}` : p.id,
		})),
	]);

	async function handlePolicyChange(newPolicy: string) {
		if (newPolicy === server.policy) return;
		policyChanging = true;
		try {
			const fresh = await api.setManagedServerPolicy(serverId, newPolicy);
			servers.applyMutationResponse(fresh);
			notifications.success('Политика обновлена');
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Ошибка изменения политики');
			selectedPolicy = server.policy;
		} finally {
			policyChanging = false;
		}
	}
</script>

<div class="card managed-card" class:status-up={isUp}>
	<!-- Header -->
	<div class="card-header">
		<div class="header-info">
			<div class="title-row">
				<div class="title-main">
					<Toggle
						checked={isUp}
						onchange={handleToggleEnabled}
						disabled={togglingEnabled || restartingServer}
						size="sm"
						spinner="none"
					/>
					<h3 class="card-title">{serverDisplayName}</h3>
				</div>
				<div class="title-badges">
					<span class="badge-managed">Управляемый</span>
					{#if ascParams !== null}
						<VersionBadge kind="awg" value={awgVersion} />
					{/if}
				</div>
			</div>
			<div class="server-meta">
				<span class="meta mono">{server.interfaceName}</span>
				<span class="meta mono">{server.address}/{maskToPrefix(server.mask)}</span>
				<span class="meta mono">:{server.listenPort}</span>
				{#if server.mtu}
					<span class="meta mono">MTU {server.mtu}</span>
				{/if}
				{#if stats && (totalRx > 0 || totalTx > 0)}
					<span class="meta mono">↓{formatBytes(totalRx)} ↑{formatBytes(totalTx)}</span>
				{/if}
			</div>
		</div>
		<div class="header-right">
			<div class="header-actions">
			<Button
				variant="ghost"
				size="sm"
				onclick={handleRestartOrStart}
				disabled={restartingServer || togglingEnabled || deleting}
				loading={restartingServer}
				iconBefore={restartIcon}
				title={isUp
					? `Перезапустить сервер «${serverDisplayName}»`
					: `Запустить сервер «${serverDisplayName}»`}
			>
				{isUp ? 'Рестарт' : 'Запуск'}
			</Button>
			<Button
				variant="ghost"
				size="sm"
				onclick={onOpenASC}
				iconBefore={ascIcon}
				title={`Параметры обфускации сервера «${serverDisplayName}»`}
			>
				Обфускация
			</Button>
			<Button
				variant="ghost"
				size="sm"
				onclick={() => editServerOpen = true}
				iconBefore={settingsIcon}
				title={`Настройки сервера «${serverDisplayName}»`}
			>
				Настройки
			</Button>
			{#if confirmDelete}
				<Button
					variant="danger"
					size="sm"
					onclick={handleDeleteServer}
					loading={deleting}
					title={`Подтвердить удаление сервера «${serverDisplayName}»`}
				>
					Подтвердить?
				</Button>
			{:else}
				<Button
					variant="outline-danger"
					size="sm"
					onclick={handleDeleteServer}
					disabled={deleting}
					iconBefore={deleteIcon}
					title={`Удалить сервер «${serverDisplayName}»`}
				>
					Удалить
				</Button>
			{/if}
			</div>
		</div>
	</div>

	<!-- Settings -->
	<div class="server-settings">
		<div class="setting-row">
			<div class="setting-copy">
				<span class="setting-title">NAT</span>
				{#if natMode === 'full'}
					<span class="setting-description">
						Клиент выходит в интернет с внешним IP {@render wanIpButton()}. В LAN виден не как отдельное устройство, а как роутер{lanRouterLabel}.
					</span>
				{:else if natMode === 'internet-only'}
					<span class="setting-description">
						Клиент выходит в интернет с внешним IP {@render wanIpButton()}, в LAN виден со своим VPN-адресом ({vpnSubnetLabel}).
					</span>
				{:else}
					<span class="setting-description">Выхода в интернет для клиента нет (без дополнительной подмены адреса), в LAN виден со своим VPN-адресом ({vpnSubnetLabel}).</span>
				{/if}
				{#if ingressEnabled && natMode === 'full'}
					<span class="setting-description setting-description-warning">NAT для интернета не действует — интернет-трафик идёт через sing-box (туннель); режим NAT влияет только на видимость в LAN</span>
				{/if}
			</div>
			<div class="setting-control">
				<Dropdown
					value={natMode}
					options={natModeOptions}
					disabled={togglingNAT}
					onchange={handleSetNATMode}
					fullWidth
				/>
			</div>
		</div>

		<div class="setting-row">
			<div class="setting-copy">
				<span class="setting-title">Доступ в LAN</span>
				<span class="setting-description">Сегменты LAN, доступные клиентам этого сервера</span>
			</div>
			<div class="setting-control">
				<ChipMultiSelect values={server.lanSegments ?? []} options={lanSegmentOptions} onchange={handleSetLANSegments} disabled={settingLAN} />
			</div>
		</div>

		<div class="setting-row setting-row-toggle">
			<div class="setting-copy">
				<span class="setting-title">Маршрутизация через sing-box</span>
				<span class="setting-description">Заворачивать интернет-трафик клиентов данного сервера в sing-box</span>
			</div>
			<div class="setting-control setting-control-toggle">
				<Toggle checked={ingressEnabled} onchange={handleToggleIngress} disabled={togglingIngress} spinner="after" />
			</div>
		</div>

		<div class="setting-row">
			<div class="setting-copy">
				<span class="setting-title">Политика доступа</span>
				<span class="setting-description">Регулирует выход в интернет для клиентов сервера. Применяется ко всем клиентам этого сервера.</span>
			</div>
			<div class="setting-control">
				<Dropdown
					value={selectedPolicy}
					options={policyOptions}
					disabled={policyChanging}
					onchange={handlePolicyChange}
					fullWidth
				/>
			</div>
		</div>
	</div>

	<!-- Peers -->
	<div class="peers-section">
		<div class="peers-header">
			<span class="peers-title">Клиенты {#if stats}({onlineCount}/{(server.peers ?? []).length} онлайн){:else}({(server.peers ?? []).length}){/if}</span>
			<div class="peers-controls">
				<PeerSortControls
					bind:searchQuery
					showSearch={(server.peers ?? []).length >= 5}
					hideSortOnDesktop
				/>
				<Button variant="secondary" size="sm" onclick={() => addPeerOpen = true} iconBefore={addPeerIcon}>
					Добавить клиента
				</Button>
			</div>
		</div>

		{#if (server.peers ?? []).length === 0}
			<div class="empty-peers">Нет клиентов. Добавьте первого.</div>
		{:else}
			<ManagedPeerTable
				peers={sortedPeers}
				{getPeerStats}
				{confirmDeletePeerKey}
				onTogglePeer={handleTogglePeer}
				onOpenConf={openConf}
				onOpenEditPeer={openEditPeer}
				onDeletePeerClick={handleDeletePeerClick}
			/>
		{/if}
	</div>
</div>

{#snippet wanIpButton()}
	<button
		type="button"
		class="wan-ip-reveal mono"
		onclick={() => (showWanIP = !showWanIP)}
		title={showWanIP && wanIP ? wanIP : 'Показать внешний IP'}
		aria-label={showWanIP ? 'Скрыть внешний IP' : 'Показать внешний IP'}
		aria-pressed={showWanIP}
	>
		({showWanIP && wanIP ? wanIP : WAN_IP_MASKED})
	</button>
{/snippet}

<!-- Modals -->
<EditManagedServerModal
	bind:open={editServerOpen}
	{serverId}
	{server}
	onclose={() => editServerOpen = false}
	onUpdated={onUpdated}
/>

<AddManagedPeerModal
	bind:open={addPeerOpen}
	{serverId}
	{server}
	{routerIP}
	onclose={() => addPeerOpen = false}
	onAdded={onUpdated}
/>

{#if selectedPeer}
	<EditManagedPeerModal
		bind:open={editPeerOpen}
		{serverId}
		peer={selectedPeer}
		{routerIP}
		onclose={() => { editPeerOpen = false; selectedPeer = null; }}
		onUpdated={onUpdated}
	/>
{/if}

<PeerConfModal
	bind:open={confModalOpen}
	{serverId}
	pubkey={confPubkey}
	peerName={confPeerName}
	onclose={() => confModalOpen = false}
/>

{#snippet addPeerIcon()}
	<Plus size={14} strokeWidth={2} aria-hidden="true" />
{/snippet}

{#snippet restartIcon()}
	<RefreshCw size={14} strokeWidth={2} aria-hidden="true" />
{/snippet}

{#snippet ascIcon()}
	<EarthLock size={14} strokeWidth={2} aria-hidden="true" />
{/snippet}

{#snippet settingsIcon()}
	<Settings size={14} strokeWidth={2} aria-hidden="true" />
{/snippet}

{#snippet deleteIcon()}
	<Trash2 size={14} strokeWidth={2} aria-hidden="true" />
{/snippet}


<style>
	.managed-card {
		display: flex;
		flex-direction: column;
		--managed-section-gap: 0.625rem;
		gap: var(--managed-section-gap);
		border-color: var(--accent);
	}

	.card-header {
		display: flex;
		flex-wrap: wrap;
		justify-content: space-between;
		align-items: flex-start;
		gap: 1rem;
		margin-bottom: 0;
		padding-bottom: 0;
		border-bottom: none;
	}

	.header-info {
		display: flex;
		flex-direction: column;
		gap: 0.375rem;
		flex: 1 1 265px;
		min-width: 265px;
		max-width: 100%;
	}

	.title-row {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		column-gap: 0.5rem;
		row-gap: 0.375rem;
		min-width: 0;
	}

	.title-main {
		display: flex;
		align-items: center;
		gap: 0.5rem;
		min-width: 0;
		flex: 0 1 auto;
		max-width: 100%;
	}

	.title-main :global(.toggle-container) {
		flex-shrink: 0;
		display: inline-flex;
		align-items: center;
		align-self: center;
	}

	.title-badges {
		display: inline-flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.375rem;
		flex: 1 1 auto;
		min-width: fit-content;
	}

	.card-title {
		font-size: 1.125rem;
		font-weight: 600;
		min-width: 0;
	}

	.badge-managed {
		display: inline-flex;
		align-items: center;
		padding: 2px 8px;
		font-size: 11px;
		font-weight: 500;
		border-radius: 10px;
		background: rgba(59, 130, 246, 0.15);
		color: var(--accent);
	}

	.server-meta {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		flex-wrap: wrap;
	}

	.meta {
		font-size: 0.75rem;
		color: var(--text-muted);
	}

	.mono {
		font-family: var(--font-mono, monospace);
	}

	.header-right {
		display: flex;
		flex-wrap: nowrap;
		align-items: center;
		justify-content: flex-end;
		gap: 0.5rem;
		flex: 0 0 auto;
		margin-left: auto;
	}

	.header-actions {
		display: flex;
		flex-wrap: nowrap;
		align-items: center;
		justify-content: flex-end;
		gap: 0.25rem;
		flex: 0 0 auto;
	}

	.server-settings {
		background: var(--color-bg-tertiary);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		padding: 0 0.875rem;
		margin-top: 0.5rem;
		min-width: 0;
	}

	.server-settings .setting-row:first-child {
		padding-top: 0.875rem;
	}

	.server-settings .setting-row:last-child {
		padding-bottom: 0.875rem;
	}

	.setting-copy {
		display: flex;
		flex-direction: column;
		gap: 0.125rem;
		min-width: 0;
	}

	.setting-title {
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--text-primary);
	}

	.setting-description-warning {
		color: var(--warning, #f59e0b);
	}

	.wan-ip-reveal {
		display: inline;
		padding: 0;
		margin: 0;
		border: none;
		background: none;
		font: inherit;
		font-size: inherit;
		line-height: inherit;
		color: var(--text-secondary);
		cursor: pointer;
		text-decoration: none;
		vertical-align: baseline;
		-webkit-tap-highlight-color: transparent;
	}

	.wan-ip-reveal:hover {
		color: var(--text-primary);
	}

	.wan-ip-reveal:focus,
	.wan-ip-reveal:focus-visible {
		outline: none;
		box-shadow: none;
	}

	.setting-control {
		width: 100%;
		min-width: 0;
		max-width: 280px;
		justify-self: end;
	}

	.setting-control :global(.dropdown),
	.setting-control :global(.field),
	.setting-control :global(.picker) {
		width: 100%;
		min-width: 0;
	}

	.setting-control-toggle {
		width: auto;
		max-width: none;
		justify-self: end;
		align-self: center;
	}

	.setting-row-toggle {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: center;
		gap: 0.75rem;
	}

	.setting-row-toggle .setting-copy {
		min-width: 0;
	}

	.server-settings :global(.field .trigger) {
		background: var(--color-settings-surface-bg);
		border-color: var(--color-border);
	}

	.server-settings :global(.field .trigger:hover:not(:disabled)) {
		background: var(--color-bg-hover);
	}

	.server-settings :global(.picker .chips) {
		background: var(--color-settings-surface-bg);
		border-color: var(--color-border);
		border-radius: var(--radius-sm);
	}

	.server-settings :global(.toggle-container .toggle-slider) {
		background: var(--color-settings-surface-bg);
	}

	.server-settings :global(.toggle-container:hover input:not(:checked) ~ .toggle-slider) {
		background: var(--color-bg-hover);
	}

	@media (min-width: 641px) {
		.server-settings .setting-row {
			display: grid;
			grid-template-columns: minmax(0, 1fr) minmax(12rem, 280px);
			align-items: start;
			gap: 0.75rem;
		}

		.server-settings .setting-row-toggle {
			grid-template-columns: minmax(0, 1fr) auto;
			align-items: center;
		}

		.setting-control {
			max-width: none;
		}
	}

	.peers-section {
		--peers-divider-gap: 1rem;
		border-top: 1px solid var(--border);
		padding-top: var(--peers-divider-gap);
		margin-top: calc(var(--peers-divider-gap) - var(--managed-section-gap));
	}

	.peers-header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 0.75rem;
	}

	.peers-controls {
		display: flex;
		align-items: center;
		gap: 0.375rem;
	}

	.peers-title {
		font-size: 0.875rem;
		font-weight: 600;
		color: var(--text-secondary);
	}

	.empty-peers {
		padding: 1.5rem;
		text-align: center;
		font-size: 0.8125rem;
		color: var(--text-muted);
	}

	@media (max-width: 640px) {
		.peers-header {
			flex-direction: column;
			align-items: stretch;
			gap: 0.5rem;
		}

		.peers-controls {
			flex-wrap: wrap;
		}

		.peers-controls :global(.btn) {
			width: 100%;
		}

		.card-header {
			flex-direction: column;
		}

		.header-info {
			flex: 1 1 auto;
			min-width: 0;
		}

		.header-right {
			width: 100%;
			margin-left: 0;
			flex-direction: column;
			align-items: stretch;
			gap: 0.5rem;
		}

		.setting-control {
			max-width: none;
		}

		.header-actions {
			align-self: stretch;
			display: grid;
			grid-template-columns: repeat(2, minmax(0, 1fr));
			gap: 0.5rem;
			width: 100%;
		}

		.header-actions :global(.btn) {
			width: 100%;
			min-width: 0;
			justify-content: center;
		}

	}

	@media (max-width: 640px) {
		.managed-card {
			overflow: hidden;
		}

		.peers-controls {
			display: grid;
			grid-template-columns: 1fr;
			width: 100%;
			gap: 0.4rem;
		}

		.peers-controls :global(.peer-sort-controls) {
			width: 100%;
		}

		.peers-controls :global(.btn) {
			width: 100%;
		}
	}
</style>
