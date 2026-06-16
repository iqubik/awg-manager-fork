<script lang="ts">
	import { api } from '$lib/api/client';
	import { notifications } from '$lib/stores/notifications';
	import { usageLevel } from '$lib/stores/settings';
	import { Modal, Button, SegmentedControl } from '$lib/components/ui';
	import ChangelogModal from './ChangelogModal.svelte';
	import DownloadErrorNotice from '$lib/components/downloads/DownloadErrorNotice.svelte';
	import { downloadErrorToText } from '$lib/utils/downloadError';
	import type { UpdateInfo } from '$lib/types';

	interface Props {
		updateInfo: UpdateInfo | null;
		currentChannel?: 'stable' | 'develop';
		saving?: boolean;
		showChannelSwitch?: boolean;
		onRequestChannel?: (channel: 'stable' | 'develop') => void;
	}

	let {
		updateInfo = $bindable(),
		currentChannel = 'stable',
		saving = false,
		showChannelSwitch = false,
		onRequestChannel,
	}: Props = $props();

	let checking = $state(false);
	let upgrading = $state(false);
	let showConfirm = $state(false);
	let showChangelog = $state(false);
	let upgradePhase = $state<'idle' | 'starting' | 'waiting-restart'>('idle');
	let restartAttempt = $state(0);
	const maxRestartAttempts = 30;

	const manualCheckTitle = $derived(
		updateInfo?.available ? 'Проверить наличие более новой версии' : 'Проверить обновления'
	);
	const manualCheckLabel = $derived(
		checking ? 'Проверка...' : updateInfo?.available ? 'Проверить ещё' : 'Проверить'
	);
	const showUpdateDiagnostics = $derived($usageLevel !== 'basic');
	const updateDiagnostics = $derived.by(() => {
		if (!updateInfo || !showUpdateDiagnostics) return '';

		const parts: string[] = [];
		if (updateInfo.channel) parts.push(`Канал: ${updateInfo.channel}`);
		if (updateInfo.source) parts.push(`Источник: ${updateInfo.source}`);
		if (updateInfo.sourceUrl) parts.push(`URL: ${updateInfo.sourceUrl}`);
		return parts.join(' · ');
	});
	const channelDescription = $derived(
		currentChannel === 'develop'
			? 'Свежие сборки из ветки разработки, могут быть нестабильны.'
			: 'Стабильные релизы из GitHub Release.'
	);
	const upgradeProgress = $derived.by(() => {
		if (!upgrading) return 0;
		if (upgradePhase === 'starting') return 18;
		if (upgradePhase === 'waiting-restart') {
			return Math.min(95, 18 + Math.round((restartAttempt / maxRestartAttempts) * 77));
		}
		return 0;
	});

	async function checkForUpdates() {
		if (checking) return;
		checking = true;
		try {
			updateInfo = await api.checkUpdate(true);
			if (updateInfo.error) {
				notifications.error(`Проверка обновлений: ${downloadErrorToText(updateInfo.error)}`);
			} else if (updateInfo.available) {
				notifications.success(`Доступна версия ${updateInfo.latestVersion}`);
			} else {
				notifications.info('Обновлений нет');
			}
			if (updateInfo.warning) {
				notifications.info(updateInfo.warning);
			}
		} catch (e) {
			notifications.error(`Проверка обновлений: ${downloadErrorToText(e)}`);
		} finally {
			checking = false;
		}
	}

	function confirmUpgrade() {
		if (checking || !updateInfo?.available) return;
		showConfirm = true;
	}

	async function applyUpgrade() {
		if (checking || !updateInfo?.available) return;
		showConfirm = false;
		upgrading = true;
		upgradePhase = 'starting';
		restartAttempt = 0;

		// Capture instanceId before upgrade to detect restart
		let previousInstanceId = '';
		try {
			const status = await api.getBootStatus();
			previousInstanceId = status.instanceId;
		} catch { /* proceed anyway */ }

		try {
			await api.applyUpdate();
		} catch (e) {
			notifications.error(`Запуск обновления: ${downloadErrorToText(e)}`);
			upgrading = false;
			upgradePhase = 'idle';
			restartAttempt = 0;
			return;
		}

		// Poll boot-status (public endpoint — no auth, no connection-lost callbacks).
		// Detect restart via instanceId change, then reload to pick up new frontend.
		upgradePhase = 'waiting-restart';

		for (let i = 0; i < maxRestartAttempts; i++) {
			restartAttempt = i + 1;
			await new Promise(r => setTimeout(r, 2000));
			try {
				const status = await api.getBootStatus();
				if (status.instanceId !== previousInstanceId && !status.initializing) {
					window.location.reload();
					return;
				}
			} catch {
				// Server still down — expected during upgrade
			}
		}

		notifications.error('Сервер не ответил после обновления');
		upgrading = false;
		upgradePhase = 'idle';
		restartAttempt = 0;
	}
</script>

<div class="setting-row update-row">
	<div class="flex flex-col gap-1 update-info">
		{#if upgrading}
			<span class="setting-description update-status">
				Обновление... не закрывайте страницу
			</span>
			<div class="update-progress" aria-label="Прогресс обновления">
				<div class="update-progress-bar" style={`width: ${upgradeProgress}%`}></div>
			</div>
			<span class="setting-description update-progress-caption">
				{#if upgradePhase === 'starting'}
					Запускаем обновление...
				{:else}
					Ожидаем перезапуск сервиса...
				{/if}
			</span>
		{:else if updateInfo?.available}
			<span class="setting-description update-available">
				Доступна версия {updateInfo.latestVersion}
			</span>
		{:else if updateInfo?.error}
			<div class="update-error-notice">
				<DownloadErrorNotice error={updateInfo.error} hideSettingsLink />
			</div>
			{#if updateDiagnostics}
				<span class="setting-description update-diagnostics">
					{updateDiagnostics}
				</span>
			{/if}
		{:else}
			<span class="setting-description">
				Установлена последняя версия
			</span>
		{/if}
		{#if updateInfo?.warning}
			<span class="setting-description update-warning">
				{updateInfo.warning}
			</span>
		{/if}
	</div>
	{#if showChannelSwitch}
		<div class="update-channel">
			<span class="update-channel-label">Канал обновлений</span>
			<span class="setting-description update-channel-description">
				{channelDescription}
			</span>
			<SegmentedControl
				value={currentChannel}
				options={[
					{ value: 'stable', label: 'Стабильный' },
					{ value: 'develop', label: 'Разработка' },
				] satisfies Array<{ value: 'stable' | 'develop'; label: string }>}
				ariaLabel="Канал обновлений"
				disabled={saving || upgrading || checking}
				onchange={(channel) => onRequestChannel?.(channel)}
			/>
		</div>
	{/if}
	<div class="update-actions">
		{#if upgrading}
			<div class="update-spinner"></div>
		{:else}
			{#if updateInfo?.currentVersion}
				<Button
					variant="secondary"
					size="sm"
					onclick={() => (showChangelog = true)}
				>
					Что нового
				</Button>
			{/if}
			<!-- Manual check must stay available even when an update is already cached:
				repo may publish a newer build after the cached result was fetched. -->
			<Button
				variant="secondary"
				size="sm"
				onclick={checkForUpdates}
				loading={checking}
				title={manualCheckTitle}
			>
				{manualCheckLabel}
			</Button>
			{#if updateInfo?.available}
				<Button
					variant="primary"
					size="sm"
					onclick={confirmUpgrade}
					disabled={checking}
				>
					Обновить
				</Button>
			{/if}
		{/if}
	</div>
</div>

<Modal
	open={showConfirm}
	title="Обновление"
	onclose={() => showConfirm = false}
>
	<p class="modal-text">
		Обновить до версии {updateInfo?.latestVersion}? Сервис будет перезапущен.
	</p>

	{#snippet actions()}
		<Button variant="secondary" size="md" onclick={() => showConfirm = false}>Отмена</Button>
		<Button variant="primary" size="md" onclick={applyUpgrade}>Обновить</Button>
	{/snippet}
</Modal>

{#if updateInfo?.currentVersion}
	<ChangelogModal
		open={showChangelog}
		pendingUpdate={Boolean(updateInfo.available && updateInfo.latestVersion)}
		fromVersion={updateInfo.available && updateInfo.latestVersion ? updateInfo.currentVersion : ''}
		toVersion={updateInfo.available && updateInfo.latestVersion ? updateInfo.latestVersion : updateInfo.currentVersion}
		oncheckUpdates={() => {
			showChangelog = false;
			void checkForUpdates();
		}}
		onclose={() => (showChangelog = false)}
	/>
{/if}

<style>
	.update-row.setting-row {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		align-items: start;
		gap: 0.75rem;
	}

	.update-info {
		min-width: 0;
	}

	.update-channel {
		display: grid;
		grid-template-columns: minmax(0, 1fr);
		gap: 0.4rem;
		padding-top: 0.1rem;
	}

	.update-channel-label {
		font-weight: 600;
		font-size: 0.875rem;
		color: var(--text-primary);
	}

	.update-channel-description {
		line-height: 1.4;
	}

	.update-channel :global(.segmented-control) {
		width: 100%;
	}

	.update-actions {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.5rem;
		justify-content: stretch;
		width: 100%;
		flex-shrink: 1;
	}

	@media (max-width: 860px) {
		.update-actions {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}

		.update-actions :global(button) {
			width: 100%;
		}
	}

	.update-actions :global(button) {
		width: 100%;
		min-width: 0;
	}

	.update-actions :global(button:first-child:nth-last-child(3)),
	.update-actions :global(button:first-child:last-child) {
		grid-column: 1 / -1;
	}

	.update-spinner {
		grid-column: 1 / -1;
		justify-self: end;
	}

	@media (min-width: 641px) {
		.update-channel :global(.segmented-control) {
			display: flex;
			width: 100%;
			max-width: none;
			justify-self: stretch;
		}

		.update-channel :global(.segmented-control-btn) {
			flex: 1 1 50%;
			min-width: 0;
		}

		.update-actions {
			justify-self: end;
			max-width: 28rem;
		}
	}

	@media (max-width: 480px) {
		.update-actions {
			grid-template-columns: 1fr;
		}
	}

	.update-available {
		color: var(--success, #22c55e) !important;
		font-weight: 500;
	}

	.update-error-notice {
		min-width: 0;
	}

	.update-warning {
		color: var(--warning, #eab308) !important;
	}

	.update-status {
		color: var(--accent) !important;
	}

	.update-progress {
		width: 100%;
		height: 0.45rem;
		overflow: hidden;
		border-radius: 999px;
		background: color-mix(in srgb, var(--border) 65%, transparent);
	}

	.update-progress-bar {
		height: 100%;
		border-radius: inherit;
		background: var(--accent);
		transition: width 0.35s ease;
	}

	.update-progress-caption {
		font-size: 0.75rem;
		color: var(--text-muted, var(--text-secondary));
	}

	.update-diagnostics {
		color: var(--text-muted, var(--text-secondary));
		word-break: break-word;
	}

	.update-spinner {
		width: 20px;
		height: 20px;
		border: 2px solid var(--border);
		border-top-color: var(--accent);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to { transform: rotate(360deg); }
	}

	.modal-text {
		color: var(--text-secondary);
		margin: 0;
	}
</style>
