<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { notifications } from '$lib/stores/notifications';
	import { singboxRouter } from '$lib/stores/singboxRouter';
	import type { RouterPolicy, SingboxRouterWANInterface } from '$lib/types';
	import {
		Toggle,
		Dropdown,
		Button,
		Card,
		Input,
		Modal,
		StatusDot,
		StatRow,
	} from '$lib/components/ui';
	import type { DropdownOption, StatTile } from '$lib/components/ui';
	import { NetfilterMissingBanner } from '$lib/components/routing/singboxRouter';

	const statusStore = singboxRouter.status;
	const settingsStore = singboxRouter.settings;
	const rulesStore = singboxRouter.rules;
	const ruleSetsStore = singboxRouter.ruleSets;

	const status = $derived($statusStore);
	const settings = $derived($settingsStore);
	const rules = $derived($rulesStore);
	const ruleSets = $derived($ruleSetsStore);

	let busy = $state(false);
	let policies = $state<RouterPolicy[]>([]);
	let wanInterfaces = $state<SingboxRouterWANInterface[]>([]);
	let creatingPolicy = $state(false);
	let createModalOpen = $state(false);
	let createDescription = $state('awgm-router');

	// Preset configuration — must match backend knownPresets keys
	const PRESET_CONFIG = [
		{
			id: 'l2tp',
			label: 'L2TP / IPsec',
			ports: '500, 4500, 1701 UDP',
			description: 'Корпоративный VPN, операторский L2TP',
		},
		{
			id: 'ntp',
			label: 'NTP',
			ports: '123 UDP',
			description: 'Синхронизация времени',
		},
		{
			id: 'netbios-smb',
			label: 'NetBIOS / SMB',
			ports: '137–139 UDP/TCP, 445 TCP',
			description: 'Локальная сеть, доступ к файлам',
		},
	] as const;

	let advancedOpen = $state(false);
	let localPresets = $state<string[]>([]);
	let localExtraPorts = $state('');
	let extraPortsError = $state('');
	let savingAdvanced = $state(false);

	const advancedHint = $derived.by(() => {
		// When closed, show server-persisted state; when open, reflect the local edits.
		const activePresets = advancedOpen ? localPresets : (settings?.bypassPresets ?? []);
		const activeExtra = advancedOpen ? localExtraPorts : (settings?.bypassExtraPorts ?? '');
		const labels = activePresets
			.map((id) => PRESET_CONFIG.find((p) => p.id === id)?.label)
			.filter(Boolean)
			.join(', ');
		if (labels) return labels;
		if (activeExtra.trim()) return '+ доп. порты';
		return '';
	});

	function toggleAdvanced(): void {
		advancedOpen = !advancedOpen;
		if (advancedOpen && settings) {
			// Sync local state from server on open
			localPresets = [...(settings.bypassPresets ?? [])];
			localExtraPorts = settings.bypassExtraPorts ?? '';
			extraPortsError = '';
		}
	}

	function togglePreset(id: string): void {
		if (localPresets.includes(id)) {
			localPresets = localPresets.filter((p) => p !== id);
		} else {
			localPresets = [...localPresets, id];
		}
	}

	function validateExtraPorts(value: string): boolean {
		if (!value.trim()) return true;
		const entries = value.split(',').map((e) => e.trim()).filter(Boolean);
		return entries.every((e) => /^\d+\s+(UDP|TCP)$/i.test(e));
	}

	async function saveAdvanced(): Promise<void> {
		if (!settings) return;
		extraPortsError = '';
		if (!validateExtraPorts(localExtraPorts)) {
			extraPortsError = 'Формат: «PORT UDP» или «PORT TCP», через запятую. Например: 51820 UDP, 1194 TCP';
			return;
		}
		savingAdvanced = true;
		try {
			await api.singboxRouterPutSettings({
				...settings,
				bypassPresets: localPresets,
				bypassExtraPorts: localExtraPorts.trim(),
			});
			await refresh();
		} catch (e) {
			notifications.error((e as Error).message);
		} finally {
			savingAdvanced = false;
		}
	}

	async function refresh(): Promise<void> {
		await singboxRouter.loadAll();
	}

	async function loadPolicies(): Promise<void> {
		try {
			policies = await api.singboxRouterListPolicies();
		} catch (e) {
			notifications.error((e as Error).message);
			policies = [];
		}
	}

	async function loadWANInterfaces(): Promise<void> {
		try {
			wanInterfaces = await api.singboxRouterListWANInterfaces();
		} catch (e) {
			notifications.error((e as Error).message);
			wanInterfaces = [];
		}
	}

	onMount(() => {
		// Engine sub-tab is the entry point of /routing?tab=singbox; refresh
		// the cold router store on mount so all sub-tabs see fresh data
		// without each one re-fetching.
		refresh();
		loadPolicies();
		loadWANInterfaces();
	});

	async function selectPolicy(name: string): Promise<void> {
		if (!settings) return;
		busy = true;
		try {
			await api.singboxRouterPutSettings({ ...settings, policyName: name });
			await refresh();
		} catch (e) {
			notifications.error((e as Error).message);
		} finally {
			busy = false;
		}
	}

	// Toggle wanAutoDetect. Switching to pinned mode requires also
	// supplying a non-empty wanInterface — backend validator rejects
	// {auto:false, interface:""}. We pick the first WAN as a default
	// so the toggle never leaves the user with an invalid pending state.
	async function setWANAutoDetect(next: boolean): Promise<void> {
		if (!settings) return;
		busy = true;
		try {
			if (next) {
				await api.singboxRouterPutSettings({
					...settings,
					wanAutoDetect: true,
					wanInterface: '',
				});
			} else {
				const first = wanInterfaces[0]?.name ?? '';
				if (!first) {
					notifications.error(
						'Не удалось получить список WAN-интерфейсов с роутера',
					);
					return;
				}
				await api.singboxRouterPutSettings({
					...settings,
					wanAutoDetect: false,
					wanInterface: first,
				});
			}
			await refresh();
		} catch (e) {
			notifications.error((e as Error).message);
		} finally {
			busy = false;
		}
	}

	async function selectWAN(kernelName: string): Promise<void> {
		if (!settings || !kernelName) return;
		busy = true;
		try {
			await api.singboxRouterPutSettings({
				...settings,
				wanAutoDetect: false,
				wanInterface: kernelName,
			});
			await refresh();
		} catch (e) {
			notifications.error((e as Error).message);
		} finally {
			busy = false;
		}
	}

	function openCreateModal(): void {
		createDescription = 'awgm-router';
		createModalOpen = true;
	}

	async function confirmCreatePolicy(): Promise<void> {
		if (creatingPolicy || !settings) return;
		const trimmed = createDescription.trim();
		if (!trimmed) return;
		creatingPolicy = true;
		try {
			const created = await api.singboxRouterCreatePolicy(trimmed);
			await loadPolicies();
			await api.singboxRouterPutSettings({ ...settings, policyName: created.name });
			await refresh();
			createModalOpen = false;
			notifications.success(
				'Политика создана. Отметьте устройства которые должны идти через VPN.',
			);
		} catch (e) {
			notifications.error((e as Error).message);
		} finally {
			creatingPolicy = false;
		}
	}

	async function toggleEnabled(next: boolean): Promise<void> {
		if (busy || !status) return;
		busy = true;
		try {
			if (next) await api.singboxRouterEnable();
			else await api.singboxRouterDisable();
			await refresh();
		} catch (e) {
			notifications.error((e as Error).message);
		} finally {
			busy = false;
		}
	}

	const canEnable = $derived(
		!!status?.netfilterAvailable &&
			!!status?.tproxyTargetAvailable &&
			!!settings?.policyName,
	);

	const enableTooltip = $derived.by(() => {
		if (!status?.netfilterAvailable) return 'Netfilter недоступен';
		if (!status?.tproxyTargetAvailable) return 'TPROXY target недоступен';
		if (!settings?.policyName) return 'Сначала выберите или создайте политику';
		return '';
	});

	// True when settings reference a policy that no longer exists in NDMS
	// (manual deletion, NDMS reset, etc.). Distinct from "never configured":
	// here we have a stale name that needs an explicit recovery action.
	const policyMissing = $derived(
		!!settings?.policyName && status?.policyExists === false,
	);

	const policyOptions = $derived<DropdownOption[]>(
		policies.map((p) => {
			const meta = p.mark
				? `${p.mark} · ${p.deviceCount} устр.`
				: `${p.deviceCount} устр.`;
			return {
				value: p.name,
				label: p.description || p.name,
				description: meta,
			};
		}),
	);

	// WAN dropdown options. Up/down is info-only (description suffix);
	// doesn't gate selection — user picks freely. Surfacing kernel name
	// in the description so the user understands they're picking a
	// stable kernel identifier, not the NDMS ID.
	const wanOptions = $derived<DropdownOption[]>(
		wanInterfaces.map((iface) => ({
			value: iface.name,
			label: iface.label || iface.id || iface.name,
			description: `${iface.id || iface.name} · ${iface.name} · ${iface.up ? 'up' : 'down'}`,
		})),
	);

	const visibleIssues = $derived(status?.issues ?? []);

	const outboundsCount = $derived(status?.outboundCompositeCount ?? 0);

	const issuesCount = $derived(visibleIssues.length);
	const rulesCount = $derived(status?.ruleCount ?? rules.length);
	const ruleSetsCount = $derived(status?.ruleSetCount ?? ruleSets.length);

	const statTiles = $derived<StatTile[]>([
		{ label: 'Правил', value: rulesCount },
		{ label: 'Наборов', value: ruleSetsCount },
		{ label: 'Outbounds', value: outboundsCount },
		{
			label: 'Issues',
			value: issuesCount,
			accent: issuesCount > 0 ? 'warning' : 'default',
		},
	]);
</script>

{#if status}
	{#if !status.netfilterAvailable}
		<NetfilterMissingBanner componentName={status.netfilterComponentName} />
	{/if}

	<!-- Engine state card -->
	<Card padding="md">
		<div class="engine-card">
			<div class="toggle-row" title={enableTooltip}>
				<Toggle
					checked={status.enabled}
					onchange={(v) => toggleEnabled(v)}
					disabled={busy || !canEnable}
					size="md"
				/>
				<span class="toggle-label">
					Движок {status.enabled ? 'включён' : 'выключен'}
				</span>
			</div>

			{#if status.netfilterAvailable && !status.tproxyTargetAvailable}
				<div class="dep-warning">
					<span>
						TPROXY target недоступен в iptables. Убедитесь что модуль
						<code>xt_TPROXY</code> загружен.
					</span>
				</div>
			{/if}

			{#if policyMissing}
				<div class="dep-warning issue-error policy-missing-row">
					<span>
						Policy <strong>«{settings?.policyName}»</strong> не найдена в NDMS —
						возможно, удалена вручную. Маршрутизация не запустится без неё.
					</span>
					<Button
						variant="primary"
						size="sm"
						onclick={openCreateModal}
						disabled={creatingPolicy || busy}
					>
						Создать «awgm-router»
					</Button>
				</div>
			{/if}

			{#if settings}
				<div class="policy-block">
					<div class="control-label">NDMS Access Policy</div>
					<div class="policy-row">
						<div class="policy-dropdown">
							<Dropdown
								value={settings.policyName}
								options={policyOptions}
								placeholder="— выберите политику —"
								disabled={busy || creatingPolicy}
								onchange={(v) => selectPolicy(v)}
								fullWidth
							/>
						</div>
						<Button
							variant="ghost"
							size="md"
							onclick={openCreateModal}
							disabled={creatingPolicy || busy}
						>
							+ Создать новую
						</Button>
					</div>

					{#if status.policyMark}
						<div class="mark-info">
							Текущий mark: <code>{status.policyMark}</code>
						</div>
					{/if}

					{#if settings.policyName}
						<div class="policy-link-row">
							<span class="setting-description">
								Привязка устройств к политике
								<strong>«{settings.policyName}»</strong>
								настраивается на отдельной странице.
							</span>
							<Button variant="ghost" size="sm" href="/routing?tab=policy">
								Управление устройствами →
							</Button>
						</div>
					{/if}
				</div>

				<div class="wan-block">
					<div class="control-label">WAN-интерфейс для outbound трафика</div>
					<div class="wan-row">
						<Toggle
							checked={settings.wanAutoDetect}
							onchange={(v) => setWANAutoDetect(v)}
							disabled={busy}
							size="sm"
						/>
						<span class="wan-toggle-label">
							{settings.wanAutoDetect
								? 'Авто-определение (sing-box выбирает сам)'
								: 'Использовать конкретный WAN'}
						</span>
					</div>
					{#if !settings.wanAutoDetect}
						<div class="wan-dropdown">
							<Dropdown
								value={settings.wanInterface ?? ''}
								options={wanOptions}
								placeholder={wanInterfaces.length === 0
									? '— нет доступных WAN —'
									: '— выберите WAN —'}
								disabled={busy || wanInterfaces.length === 0}
								onchange={(v) => selectWAN(v)}
								fullWidth
							/>
						</div>
						<div class="wan-hint">
							В sing-box передаётся kernel-имя (например, <code>ppp0</code>).
						</div>
					{/if}
				</div>

				<div class="advanced-block">
					<button class="advanced-toggle" onclick={toggleAdvanced}>
						<span>Дополнительно</span>
						{#if advancedHint && !advancedOpen}
							<span class="advanced-hint">{advancedHint}</span>
						{/if}
						<span class="advanced-chevron" class:open={advancedOpen}>›</span>
					</button>

					{#if advancedOpen}
						<div class="advanced-body">
							<div class="control-label">Исключить из перехвата</div>
							<p class="advanced-description">
								Трафик на указанные порты не будет перехватываться sing-box —
								пройдёт напрямую как без политики.
							</p>

							{#each PRESET_CONFIG as preset}
								<label class="preset-row">
									<input
										type="checkbox"
										checked={localPresets.includes(preset.id)}
										onchange={() => togglePreset(preset.id)}
										disabled={savingAdvanced}
									/>
									<div class="preset-info">
										<span class="preset-label">{preset.label}</span>
										<span class="preset-ports">{preset.ports}</span>
										<span class="preset-description">{preset.description}</span>
									</div>
								</label>
							{/each}

							<div class="extra-ports-block">
								<div class="control-label">Дополнительные порты</div>
								<input
									class="extra-ports-input"
									class:error={!!extraPortsError}
									value={localExtraPorts}
									oninput={(e) => { localExtraPorts = (e.target as HTMLInputElement).value; extraPortsError = ''; }}
									placeholder="например: 51820 UDP, 1194 TCP"
									disabled={savingAdvanced}
								/>
								{#if extraPortsError}
									<div class="extra-ports-error">{extraPortsError}</div>
								{/if}
							</div>

							<div class="advanced-actions">
								<Button
									variant="primary"
									size="sm"
									onclick={saveAdvanced}
									disabled={savingAdvanced}
								>
									{savingAdvanced ? 'Сохранение…' : 'Сохранить'}
								</Button>
							</div>
						</div>
					{/if}
				</div>
			{/if}
		</div>
	</Card>

	{#if !status.enabled}
		<div class="disabled-hint">
			Движок выключен. Настройте правила/rule sets/outbounds сейчас —
			они вступят в силу после включения.
		</div>
	{/if}

	<!-- Stat tiles -->
	<div class="stat-row-wrap">
		<StatRow tiles={statTiles} />
	</div>

	<!-- Issues panel -->
	{#if visibleIssues.length > 0}
		<Card padding="md">
			<div class="issues-panel">
				<div class="issues-title">Предупреждения</div>
				{#each visibleIssues as issue}
					<div class="issue-row">
						<StatusDot
							variant={issue.severity === 'error' ? 'error' : 'warning'}
							size="md"
						/>
						<span class="issue-text">{issue.message}</span>
					</div>
				{/each}
			</div>
		</Card>
	{/if}
{:else}
	<div class="loading">Загрузка…</div>
{/if}

<Modal
	open={createModalOpen}
	title="Создать новую access policy"
	size="sm"
	onclose={() => (createModalOpen = false)}
>
	<div class="create-modal-body">
		<Input
			label="Имя политики"
			bind:value={createDescription}
			placeholder="awgm-router"
			disabled={creatingPolicy}
		/>
		<p class="create-hint">
			Политика будет создана в NDMS. К ней будет автоматически привязан текущий
			WAN-интерфейс — это нужно чтобы NDMS начал маркировать трафик связанных
			устройств.
		</p>
	</div>
	{#snippet actions()}
		<Button
			variant="ghost"
			size="md"
			onclick={() => (createModalOpen = false)}
			disabled={creatingPolicy}
		>
			Отмена
		</Button>
		<Button
			variant="primary"
			size="md"
			onclick={confirmCreatePolicy}
			disabled={creatingPolicy || !createDescription.trim()}
		>
			{creatingPolicy ? 'Создание…' : 'Создать'}
		</Button>
	{/snippet}
</Modal>

<style>
	.engine-card {
		display: flex;
		flex-direction: column;
		gap: 0.875rem;
	}
	.toggle-row {
		display: flex;
		align-items: center;
		gap: 0.625rem;
	}
	.toggle-label {
		font-weight: 600;
		color: var(--color-text-primary);
	}
	.dep-warning {
		padding: 0.5rem 0.75rem;
		background: color-mix(in srgb, var(--color-warning) 12%, transparent);
		border-left: 3px solid var(--color-warning);
		border-radius: var(--radius-sm);
		font-size: 0.85rem;
		color: var(--color-text-primary);
	}
	.dep-warning.issue-error {
		background: color-mix(in srgb, var(--color-error) 12%, transparent);
		border-left-color: var(--color-error);
	}
	.dep-warning code {
		background: var(--color-bg-tertiary);
		padding: 1px 6px;
		border-radius: 3px;
		font-size: 0.8rem;
		font-family: var(--font-mono, ui-monospace, monospace);
	}
	.policy-missing-row {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.75rem;
		flex-wrap: wrap;
	}
	.policy-missing-row > span {
		flex: 1 1 auto;
		min-width: 0;
	}
	.policy-block {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.control-label {
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--color-text-muted);
	}
	.policy-row {
		display: flex;
		gap: 0.5rem;
		align-items: flex-start;
	}
	.policy-dropdown {
		flex: 1;
		min-width: 0;
	}
	.mark-info {
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}
	.mark-info code {
		font-family: var(--font-mono, ui-monospace, monospace);
		background: var(--color-bg-tertiary);
		padding: 1px 6px;
		border-radius: 3px;
	}
	.policy-link-row {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		flex-wrap: wrap;
	}
	.setting-description {
		font-size: 0.85rem;
		color: var(--color-text-secondary);
	}
	.wan-block {
		margin-top: 1rem;
		padding-top: 1rem;
		border-top: 1px solid var(--color-border, rgba(255, 255, 255, 0.08));
	}
	.wan-row {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}
	.wan-toggle-label {
		font-size: 0.85rem;
		color: var(--color-text-primary);
	}
	.wan-dropdown {
		margin-top: 0.5rem;
	}
	.wan-hint {
		margin-top: 0.375rem;
		font-size: 0.75rem;
		color: var(--color-text-muted);
		line-height: 1.4;
	}
	.wan-hint code {
		font-family: var(--font-mono, ui-monospace, monospace);
		background: var(--color-bg-tertiary);
		padding: 1px 6px;
		border-radius: 3px;
	}
	.create-modal-body {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}
	.create-hint {
		margin: 0;
		font-size: 0.8rem;
		color: var(--color-text-muted);
		line-height: 1.4;
	}
	.stat-row-wrap {
		margin-top: 1rem;
	}
	.issues-panel {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
	}
	.issues-title {
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--color-text-muted);
		margin-bottom: 0.25rem;
	}
	.issue-row {
		display: flex;
		align-items: flex-start;
		gap: 0.5rem;
		padding: 0.4rem 0;
	}
	.issue-text {
		font-size: 0.875rem;
		color: var(--color-text-primary);
		line-height: 1.4;
	}
	.loading {
		padding: 2rem;
		text-align: center;
		color: var(--color-text-secondary);
	}
	.disabled-hint {
		padding: 0.6rem 0.9rem;
		margin: 0.75rem 0;
		background: rgba(122, 162, 247, 0.08);
		border-left: 3px solid var(--accent, #3b82f6);
		border-radius: 4px;
		color: var(--muted-text);
		font-size: 0.85rem;
		line-height: 1.4;
	}
	.stat-row-wrap + :global(.card) {
		margin-top: 1rem;
	}
	.advanced-block {
		border-top: 1px solid var(--color-border, rgba(255, 255, 255, 0.08));
		padding-top: 0.5rem;
		margin-top: 0.25rem;
	}
	.advanced-toggle {
		background: none;
		border: none;
		cursor: pointer;
		display: flex;
		align-items: center;
		gap: 0.4rem;
		width: 100%;
		padding: 0;
		font-size: 0.8rem;
		color: var(--color-text-secondary);
		text-align: left;
	}
	.advanced-toggle:hover {
		color: var(--color-text-primary);
	}
	.advanced-hint {
		font-size: 0.72rem;
		color: var(--color-text-muted);
		margin-left: 0.25rem;
	}
	.advanced-chevron {
		margin-left: auto;
		font-size: 0.75rem;
		transition: transform 0.2s;
	}
	.advanced-chevron.open {
		transform: rotate(90deg);
	}
	.advanced-body {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		margin-top: 0.75rem;
	}
	.advanced-description {
		margin: 0;
		font-size: 0.75rem;
		color: var(--color-text-muted);
		line-height: 1.4;
	}
	.preset-row {
		display: flex;
		align-items: flex-start;
		gap: 0.5rem;
		cursor: pointer;
	}
	.preset-row input[type='checkbox'] {
		margin-top: 2px;
		flex-shrink: 0;
	}
	.preset-info {
		display: flex;
		flex-direction: column;
		gap: 0.1rem;
	}
	.preset-label {
		font-size: 0.82rem;
		font-weight: 600;
		color: var(--color-text-primary);
	}
	.preset-ports {
		font-size: 0.72rem;
		color: var(--color-text-muted);
		font-family: var(--font-mono, ui-monospace, monospace);
	}
	.preset-description {
		font-size: 0.72rem;
		color: var(--color-text-muted);
	}
	.extra-ports-block {
		display: flex;
		flex-direction: column;
		gap: 0.3rem;
		margin-top: 0.25rem;
	}
	.extra-ports-input {
		width: 100%;
		background: var(--color-bg-secondary);
		border: 1px solid var(--color-border, rgba(255, 255, 255, 0.12));
		border-radius: var(--radius-sm);
		padding: 0.35rem 0.6rem;
		color: var(--color-text-primary);
		font-size: 0.8rem;
		font-family: var(--font-mono, ui-monospace, monospace);
		box-sizing: border-box;
	}
	.extra-ports-input.error {
		border-color: var(--color-error);
	}
	.extra-ports-error {
		font-size: 0.72rem;
		color: var(--color-error);
		line-height: 1.4;
	}
	.advanced-actions {
		display: flex;
		justify-content: flex-end;
		margin-top: 0.25rem;
	}
</style>
