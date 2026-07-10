<script lang="ts">
	import type {
		SingboxStatus,
		HydraRouteStatus,
		IntegrationRestoreResponse,
	} from '$lib/types';
	import { api } from '$lib/api/client';
	import Button from '$lib/components/ui/Button.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';
	import SideDrawer from '$lib/components/ui/SideDrawer.svelte';
	import StatusDot from '$lib/components/ui/StatusDot.svelte';
	import SettingsSectionLabel from './SettingsSectionLabel.svelte';
	import { notifications } from '$lib/stores/notifications';
	import { copyToClipboard } from '$lib/utils/clipboard';
	import { hydraRouteInstallProgress } from '$lib/stores/hydrarouteInstall';
	import { singboxInstallProgress } from '$lib/stores/singboxInstall';
	import { formatBytes } from '$lib/utils/format';
	import { stripAnsi } from '$lib/utils/ansi';
	import { Blocks } from 'lucide-svelte';

	interface Props {
		expanded?: boolean;
		onToggleExpanded?: () => void;
		singboxStatus: SingboxStatus | null;
		singboxStatusLoading?: boolean;
		hydraStatus: HydraRouteStatus | null;
		hydraStatusLoading?: boolean;
		hydraStatusError?: string | null;
		hydraInstalling?: boolean;
		hydraUpdating?: boolean;
		hydraInstallError?: string | null;
		hydraUpdateError?: string | null;
		singboxInstalling: boolean;
		singboxUpdating?: boolean;
		singboxInstallError: string | null;
		singboxUpdateError?: string | null;
		oninstallSingbox: () => void;
		onupdateSingbox?: () => void;
		oninstallHydra?: () => void;
		onupdateHydra?: () => void;
		showSingbox?: boolean;
		showHydra?: boolean;
	}

	let {
		expanded = true,
		onToggleExpanded,
		singboxStatus,
		singboxStatusLoading = false,
		hydraStatus,
		hydraStatusLoading = false,
		hydraStatusError = null,
		hydraInstalling = false,
		hydraUpdating = false,
		hydraInstallError = null,
		hydraUpdateError = null,
		singboxInstalling,
		singboxUpdating = false,
		singboxInstallError,
		singboxUpdateError = null,
		oninstallSingbox,
		onupdateSingbox,
		oninstallHydra,
		onupdateHydra,
		showSingbox = true,
		showHydra = true,
	}: Props = $props();

	const singboxInstalled = $derived(singboxStatus?.installed ?? false);
	const singboxRunning = $derived(singboxStatus?.running ?? false);
	const singboxNeedsUpdate = $derived(singboxStatus?.updateAvailable ?? false);
	const singboxCustomBuild = $derived(singboxStatus?.customBuild ?? false);
	const hydraInstalled = $derived(hydraStatus?.installed ?? false);
	const hydraRunning = $derived(hydraStatus?.running ?? false);
	const hydraNeedsUpdate = $derived(hydraStatus?.updateAvailable ?? false);
	const hydraCustomBuild = $derived(hydraStatus?.customBuild ?? false);
	const hydraManaged = $derived(hydraStatus?.managed ?? false);
	const hydraLegacy = $derived(hydraStatus?.legacy ?? false);
	const hydraInstallSupported = $derived(hydraStatus?.installSupported ?? false);
	const hydraNoSpace = $derived(
		hydraStatus?.installState === 'missing_no_space' || hydraStatus?.installState === 'outdated_no_space'
	);
	const hydraProcessState = $derived(
		hydraStatus?.processState ?? (hydraStatus?.running ? 'running' : hydraStatus?.installed ? 'stopped' : 'not_installed')
	);
	const singboxFatalLines = $derived.by(() => {
		const raw = stripAnsi(singboxStatus?.lastError ?? '').trim();
		if (!raw) return '';
		// Match backend stderrLineIndicatesSingBoxFatal: real sing-box text
		// fatals start with "+TZO YYYY-MM-DD …" or contain "FATAL[" — avoid
		// JSON keys like "type":"fatal" polluting the settings card.
		const fatal = raw.split('\n').filter((l) => {
			const u = l.toUpperCase();
			if (!u.includes('FATAL')) return false;
			if (u.includes('FATAL[')) return true;
			return /^\s*\+[0-9]{1,4}\s+\d{4}-\d{2}-\d{2}\b/.test(l);
		});
		return fatal.join('\n');
	});

	const installProgress = $derived($singboxInstallProgress);
	const hydraInstallProgress = $derived($hydraRouteInstallProgress);
	const installPhaseLabel = $derived.by(() => {
		const p = installProgress;
		if (!p) return '';
		switch (p.phase) {
			case 'download':
				if (p.total > 0) {
					const pct = Math.min(100, Math.round((p.downloaded / p.total) * 100));
					return `Скачивание ${pct}% (${formatBytes(p.downloaded)} / ${formatBytes(p.total)})`;
				}
				return `Скачивание (${formatBytes(p.downloaded)})`;
			case 'activate':
				return 'Установка…';
			case 'stop':
				return 'Остановка sing-box…';
			case 'start':
				return 'Запуск sing-box…';
			case 'done':
				return 'Готово';
			case 'error':
				return p.error ? `Ошибка: ${p.error}` : 'Ошибка';
			default:
				return '';
		}
	});
	const installProgressPct = $derived.by(() => {
		const p = installProgress;
		if (!p || p.phase !== 'download' || p.total <= 0) return null;
		return Math.min(100, Math.round((p.downloaded / p.total) * 100));
	});
	const hydraInstallPhaseLabel = $derived.by(() => {
		const p = hydraInstallProgress;
		if (!p) return '';
		switch (p.phase) {
			case 'prepare':
				return 'Подготовка репозитория…';
			case 'download':
				return 'Подготовка репозитория…';
			case 'activate':
			case 'install':
				return 'Установка пакета…';
			case 'upgrade':
				return 'Обновление пакета…';
			case 'stop':
				return 'Остановка HydraRoute…';
			case 'start':
				return 'Запуск HydraRoute…';
			case 'done':
				return 'Готово';
			case 'error':
				return p.error ? `Ошибка: ${p.error}` : 'Ошибка';
			default:
				return '';
		}
	});
	const hydraInstallProgressPct = $derived.by(() => {
		const p = hydraInstallProgress;
		if (!p || p.phase !== 'download' || p.total <= 0) return null;
		return Math.min(100, Math.round((p.downloaded / p.total) * 100));
	});
	const activeErrorDetails = $derived(
		hydraUpdateError ?? hydraInstallError ?? singboxUpdateError ?? singboxInstallError ?? ''
	);
	const errorModalTitle = $derived.by(() => {
		if (hydraUpdateError) return 'Не удалось обновить HydraRoute';
		if (hydraInstallError) return 'Не удалось установить HydraRoute';
		if (singboxUpdateError) return 'Не удалось обновить sing-box';
		return 'Не удалось установить sing-box';
	});

	let errorModalOpen = $state(false);
	let restoreFileInput: HTMLInputElement | null = $state(null);
	let restoreComponent = $state<'singbox' | 'hydraroute' | null>(null);
	let restoreBusy = $state(false);
	let backupBusy = $state<'singbox' | 'hydraroute' | null>(null);
	let previewModalOpen = $state(false);
	let restoreFile = $state<File | null>(null);
	let restorePreview = $state<IntegrationRestoreResponse | null>(null);

	function restoreActionLabel(action: string): string {
		switch (action) {
			case 'planned':
				return 'План';
			case 'planned_delete':
				return 'План удаления';
			case 'restored':
				return 'Восстановлено';
			case 'deleted':
				return 'Удалено';
			case 'skipped':
				return 'Пропущено';
			case 'conflict':
				return 'Конфликт';
			case 'rollback':
				return 'Откат';
			default:
				return action;
		}
	}

	function showErrorDetails() {
		errorModalOpen = true;
	}

	async function copyError() {
		if (activeErrorDetails) {
			await copyToClipboard(activeErrorDetails);
		}
	}

	// Auto-close modal when the upstream error is cleared (e.g. successful retry).
	$effect(() => {
		if (
			singboxInstallError === null &&
			singboxUpdateError === null &&
			hydraInstallError === null &&
			hydraUpdateError === null
		) {
			errorModalOpen = false;
		}
	});

	function openRestorePicker(component: 'singbox' | 'hydraroute') {
		restoreComponent = component;
		restoreFileInput?.click();
	}

	async function handleRestoreFileSelected(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		if (!file || !restoreComponent) return;
		restoreBusy = true;
		restoreFile = file;
		restorePreview = null;
		try {
			restorePreview = restoreComponent === 'singbox'
				? await api.previewSingboxRestore(file)
				: await api.previewHydraRouteRestore(file);
			previewModalOpen = true;
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Не удалось проверить архив резервной копии');
			restoreFile = null;
		} finally {
			restoreBusy = false;
		}
	}

	async function applyRestore() {
		if (!restoreFile || !restoreComponent) return;
		restoreBusy = true;
		try {
			const result = restoreComponent === 'singbox'
				? await api.restoreSingboxBackup(restoreFile)
				: await api.restoreHydraRouteBackup(restoreFile);
			const rolledBack = result.outcomes.some((item) => item.action === 'rollback');
			if (rolledBack) {
				notifications.error('Восстановление завершилось откатом');
			} else {
				notifications.success(
					restoreComponent === 'singbox'
						? 'Резервная копия sing-box восстановлена'
						: 'Резервная копия HydraRoute восстановлена'
				);
			}
			previewModalOpen = false;
			restoreFile = null;
			restorePreview = null;
			restoreComponent = null;
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Не удалось восстановить резервную копию');
		} finally {
			restoreBusy = false;
		}
	}

	async function downloadBackup(component: 'singbox' | 'hydraroute') {
		backupBusy = component;
		try {
			if (component === 'singbox') {
				await api.downloadSingboxBackup();
			} else {
				await api.downloadHydraRouteBackup();
			}
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Не удалось скачать резервную копию');
		} finally {
			backupBusy = null;
		}
	}
</script>

{#if showSingbox || showHydra}
	<div class="settings-block">
		<div class="card">
		<button
			type="button"
			class="settings-card-toggle"
			aria-expanded={expanded}
			aria-controls="integrations-card-body"
			onclick={() => onToggleExpanded?.()}
		>
			<span class="settings-card-toggle-label">
				<SettingsSectionLabel label="Интеграции" icon={Blocks} tone="purple" inline />
			</span>
			<span class="settings-card-toggle-meta">
				<span class="settings-card-meta-text">
					{#if singboxInstalled && hydraInstalled}
						Sing-box · HydraRoute
					{:else if singboxInstalled}
						Sing-box
					{:else if hydraInstalled}
						HydraRoute
					{:else}
						Не установлены
					{/if}
				</span>
				<svg
					class="settings-card-chevron"
					class:open={expanded}
					viewBox="0 0 24 24"
					fill="none"
					stroke="currentColor"
					stroke-width="2"
					aria-hidden="true"
				>
					<polyline points="6 9 12 15 18 9" />
				</svg>
			</span>
		</button>
		{#if expanded}
		<div id="integrations-card-body" class="settings-card-body">
		{#if showSingbox}
			<div class="setting-row">
				<div class="integration-item">
					<StatusDot
						variant={singboxStatusLoading ? 'muted' : (singboxInstalled && singboxRunning ? 'success' : 'muted')}
						size="md"
						ariaLabel={
							singboxStatusLoading
								? 'Sing-box: получение данных'
								: singboxInstalled && singboxRunning
									? 'Sing-box работает'
									: 'Sing-box остановлен'
						}
					/>
					<div class="integration-meta">
						<span class="font-medium">Sing-box</span>
						{#if singboxStatusLoading}
							<span class="integration-sub">получаю данные…</span>
						{:else if singboxInstalled && singboxStatus}
							<span class="integration-sub">
								v{singboxStatus.version ?? singboxStatus.currentVersion ?? '?'}
								{#if singboxRunning && singboxStatus.pid}· pid {singboxStatus.pid}{:else if !singboxRunning}· остановлен{/if}
							</span>
							{#if singboxNeedsUpdate}
								<span class="setting-description warning">
									Требуется обновление: {singboxStatus.currentVersion ?? '—'} → {singboxStatus.requiredVersion}
								</span>
							{:else if singboxCustomBuild}
								<span class="setting-description">
									Установлена отличающаяся сборка sing-box {singboxStatus.currentVersion ?? singboxStatus.version ?? '—'}
								</span>
							{/if}
							{#if singboxFatalLines}
								<span class="setting-description warning" title={singboxFatalLines}>{singboxFatalLines}</span>
							{/if}
							{#if singboxUpdateError}
								<span class="install-error-row">
									<span class="install-error-label">Не удалось обновить</span>
									<Button variant="ghost" size="sm" onclick={showErrorDetails}>
										Подробнее
									</Button>
								</span>
							{/if}
						{:else}
							<span class="setting-description">
								Поддержка VLESS/Reality, Hysteria2, NaiveProxy. Требует Entware на внешнем носителе.
							</span>
							{#if singboxInstallError}
								<span class="install-error-row">
									<span class="install-error-label">Не удалось установить</span>
									<Button variant="ghost" size="sm" onclick={showErrorDetails}>
										Подробнее
									</Button>
								</span>
							{/if}
						{/if}
					</div>
				</div>
				<div class="integration-actions">
					{#if installProgress}
						<div class="progress-widget" class:progress-error={installProgress.phase === 'error'} class:progress-done={installProgress.phase === 'done'}>
							<div class="progress-label">{installPhaseLabel}</div>
							<div class="progress-bar" class:indeterminate={installProgressPct === null && installProgress.phase !== 'done' && installProgress.phase !== 'error'}>
								<div
									class="progress-fill"
									style:width={installProgressPct !== null ? `${installProgressPct}%` : '100%'}
								></div>
							</div>
						</div>
					{:else if singboxInstalled && singboxNeedsUpdate && onupdateSingbox}
						<Button variant="primary" size="sm" fullWidth onclick={onupdateSingbox} loading={singboxUpdating}>
							{singboxUpdating ? 'Обновление...' : 'Обновить'}
						</Button>
					{:else if singboxInstalled}
						<Button
							variant="secondary"
							size="sm"
							fullWidth
							onclick={() => downloadBackup('singbox')}
							loading={backupBusy === 'singbox'}
							disabled={restoreBusy}
						>
							Скачать
						</Button>
						<Button
							variant="secondary"
							size="sm"
							fullWidth
							onclick={() => openRestorePicker('singbox')}
							loading={restoreBusy && restoreComponent === 'singbox'}
						>
							Восстановить
						</Button>
						<Button variant="secondary" size="sm" fullWidth href="/?tab=singbox">Открыть</Button>
					{:else if singboxStatusLoading}
						<Button variant="secondary" size="sm" fullWidth disabled>Ожидание…</Button>
					{:else}
						<Button variant="primary" size="sm" fullWidth onclick={oninstallSingbox} loading={singboxInstalling}>
							{singboxInstalling ? 'Установка...' : 'Установить'}
						</Button>
					{/if}
				</div>
			</div>
		{/if}

		{#if showHydra}
			<div class="setting-row">
				<div class="integration-item">
					<StatusDot
						variant={hydraStatusLoading ? 'muted' : (hydraInstalled && hydraRunning ? 'success' : 'muted')}
						size="md"
						ariaLabel={
							hydraStatusLoading
								? 'HydraRoute: получение данных'
								: hydraProcessState === 'dead'
									? 'HydraRoute: stale pid'
									: hydraInstalled && hydraRunning
									? 'HydraRoute работает'
									: 'HydraRoute остановлен'
						}
					/>
					<div class="integration-meta">
						<span class="font-medium">HydraRoute Neo</span>
						{#if hydraStatusLoading}
							<span class="integration-sub">получаю данные…</span>
						{:else if hydraInstalled}
							<span class="integration-sub">
								v{hydraStatus?.currentVersion ?? hydraStatus?.version ?? '?'}
								{#if hydraRunning && hydraStatus?.pid}
									· pid {hydraStatus.pid}
								{:else if hydraProcessState === 'dead' && hydraStatus?.stalePid}
									· dead pid {hydraStatus.stalePid}
								{:else}
									· остановлен
								{/if}
							</span>
						{:else}
							<span class="integration-sub">не установлен</span>
						{/if}
						{#if hydraInstalled && hydraNeedsUpdate}
							<span class="setting-description warning">
								Требуется обновление: {hydraStatus?.currentVersion ?? '—'} → {hydraStatus?.requiredVersion ?? '—'}
							</span>
						{:else if hydraInstalled && hydraLegacy && hydraInstallSupported}
							<span class="setting-description">
								Обнаружена внешняя или нестандартная установка HydraRoute Neo. AWGM не заменяет её автоматически.
							</span>
						{:else if hydraInstalled && hydraCustomBuild}
							<span class="setting-description">
								{hydraManaged
									? `Установлен отличный от ожидаемого пакет HydraRoute ${hydraStatus?.currentVersion ?? hydraStatus?.version ?? '—'}`
									: `Установлена внешняя сборка HydraRoute ${hydraStatus?.currentVersion ?? hydraStatus?.version ?? '—'}`}
							</span>
						{:else if !hydraInstalled && hydraInstallSupported}
							<span class="setting-description">
								HydraRoute Neo не установлен. Можно установить официальный пакет из репозитория.
							</span>
						{:else if !hydraInstalled}
							<span class="setting-description">
								Для этой архитектуры или окружения установка HydraRoute Neo из AWGM пока недоступна.
							</span>
						{/if}
						{#if hydraNoSpace}
							<span class="setting-description warning">
								Недостаточно места:
								нужно {formatBytes(hydraStatus?.requiredBytes ?? 0)},
								доступно {formatBytes(hydraStatus?.freeBytes ?? 0)}
							</span>
						{/if}
						{#if !hydraRunning && hydraStatus?.lastError}
							<span class="setting-description warning" title={hydraStatus.lastError}>{hydraStatus.lastError}</span>
						{/if}
						{#if !hydraStatusLoading && !hydraStatus && hydraStatusError}
							<span class="setting-description warning">нет ответа: {hydraStatusError}</span>
						{/if}
						{#if hydraUpdateError}
							<span class="install-error-row">
								<span class="install-error-label">Не удалось обновить</span>
								<Button variant="ghost" size="sm" onclick={showErrorDetails}>
									Подробнее
								</Button>
							</span>
						{:else if hydraInstallError}
							<span class="install-error-row">
								<span class="install-error-label">Не удалось установить</span>
								<Button variant="ghost" size="sm" onclick={showErrorDetails}>
									Подробнее
								</Button>
							</span>
						{/if}
					</div>
				</div>
			<div class="integration-actions">
				{#if hydraInstallProgress}
					<div class="progress-widget" class:progress-error={hydraInstallProgress.phase === 'error'} class:progress-done={hydraInstallProgress.phase === 'done'}>
						<div class="progress-label">{hydraInstallPhaseLabel}</div>
						<div class="progress-bar" class:indeterminate={hydraInstallProgressPct === null && hydraInstallProgress.phase !== 'done' && hydraInstallProgress.phase !== 'error'}>
							<div class="progress-fill" style:width={hydraInstallProgressPct !== null ? `${hydraInstallProgressPct}%` : '100%'}></div>
						</div>
					</div>
				{:else if hydraInstalled && hydraManaged && hydraInstallSupported && hydraNeedsUpdate && !hydraInstalling && onupdateHydra}
					<Button variant="primary" size="sm" fullWidth onclick={onupdateHydra} loading={hydraUpdating}>
						{hydraUpdating ? 'Обновление...' : 'Обновить'}
					</Button>
				{:else if hydraInstalled && hydraLegacy && hydraInstallSupported && !hydraInstalling && oninstallHydra}
					<Button variant="primary" size="sm" fullWidth onclick={oninstallHydra} loading={hydraInstalling}>
						{hydraInstalling ? 'Установка...' : 'Установить официально'}
					</Button>
				{:else if hydraInstalled && !hydraUpdating}
					<Button
						variant="secondary"
						size="sm"
						fullWidth
						onclick={() => downloadBackup('hydraroute')}
						loading={backupBusy === 'hydraroute'}
						disabled={restoreBusy}
					>
						Скачать
					</Button>
					<Button
						variant="secondary"
						size="sm"
						fullWidth
						onclick={() => openRestorePicker('hydraroute')}
						loading={restoreBusy && restoreComponent === 'hydraroute'}
					>
						Восстановить
					</Button>
					<Button variant="secondary" size="sm" fullWidth href="/routing?tab=hrneo">Открыть</Button>
				{:else if hydraStatusLoading}
					<Button variant="secondary" size="sm" fullWidth disabled>Ожидание…</Button>
				{:else if hydraInstallSupported && !hydraNoSpace && oninstallHydra}
					<Button variant="primary" size="sm" fullWidth onclick={oninstallHydra} loading={hydraInstalling}>
						{hydraInstalling ? 'Установка...' : 'Установить'}
					</Button>
				{:else}
					<Button variant="secondary" size="sm" fullWidth disabled>
						Недоступно
					</Button>
				{/if}
			</div>
			</div>
		{/if}
		</div>
		{/if}
		</div>
	</div>
{/if}

<SideDrawer
	open={errorModalOpen}
	title={errorModalTitle}
	width={640}
	onClose={() => (errorModalOpen = false)}
>
	<pre class="error-pre">{activeErrorDetails}</pre>
	{#snippet footer()}
		<Button variant="ghost" size="sm" onclick={copyError}>Скопировать</Button>
		<Button variant="primary" size="sm" onclick={() => (errorModalOpen = false)}>
			Закрыть
		</Button>
	{/snippet}
</SideDrawer>

<input
	bind:this={restoreFileInput}
	class="sr-only"
	type="file"
	accept=".zip,application/zip,application/octet-stream"
	onchange={handleRestoreFileSelected}
/>

<Modal
	open={previewModalOpen}
	title={restoreComponent === 'singbox' ? 'Восстановление резервной копии sing-box' : 'Восстановление резервной копии HydraRoute'}
	size="lg"
	onclose={() => {
		if (restoreBusy) return;
		previewModalOpen = false;
		restoreFile = null;
		restorePreview = null;
		restoreComponent = null;
	}}
>
	<div class="restore-preview">
		<div class="setting-description warning">
			Резервная копия может содержать приватные ключи, UUID, серверы, пароли и маршруты.
			Восстановление изменит настройки компонента. Архив уже проверен, но применять его
			стоит только если вы доверяете источнику.
		</div>
		{#if restoreFile}
			<p class="setting-description">Файл: {restoreFile.name}</p>
		{/if}
		{#if (restorePreview?.outcomes ?? []).some((item) => item.action === 'planned_delete')}
			<div class="setting-description warning">
				Будут удалены файлы, отсутствующие в резервной копии.
			</div>
		{/if}
		{#if restorePreview?.warnings?.length}
			<div class="restore-warning-list">
				{#each restorePreview.warnings as warning}
					<div class="setting-description warning">{warning}</div>
				{/each}
			</div>
		{/if}
		<div class="restore-outcomes">
			{#each restorePreview?.outcomes ?? [] as outcome}
				<div class="restore-outcome">
					<span class="font-medium">{restoreActionLabel(outcome.action)}</span>
					<span class="restore-path">{outcome.path}</span>
					{#if outcome.error}
						<span class="setting-description warning">{outcome.error}</span>
					{/if}
				</div>
			{/each}
		</div>
	</div>
	{#snippet actions()}
		<Button
			variant="ghost"
			size="sm"
			onclick={() => {
				previewModalOpen = false;
				restoreFile = null;
				restorePreview = null;
				restoreComponent = null;
			}}
			disabled={restoreBusy}
		>
			Отмена
		</Button>
		<Button variant="primary" size="sm" onclick={applyRestore} loading={restoreBusy}>
			Восстановить
		</Button>
	{/snippet}
</Modal>

<style>
	.card {
		container-type: inline-size;
	}

	.settings-card-toggle {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.875rem;
		width: 100%;
		padding: 0 0 0.625rem;
		border: 0;
		border-bottom: 1px solid var(--color-border);
		background: transparent;
		color: inherit;
		text-align: left;
		cursor: pointer;
	}

	.settings-card-toggle:focus-visible {
		outline: 2px solid color-mix(in srgb, var(--color-accent) 55%, transparent);
		outline-offset: 0.25rem;
		border-radius: 0.75rem;
	}

	.settings-card-toggle-label,
	.settings-card-toggle-meta {
		display: inline-flex;
		align-items: center;
		gap: 0.625rem;
		min-width: 0;
	}

	.integration-actions {
		grid-column: 1 / -1;
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(0, 1fr));
		gap: 0.5rem;
		width: 100%;
		min-width: 0;
	}

	.restore-preview {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	.restore-outcomes {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		max-height: 22rem;
		overflow: auto;
	}

	.restore-outcome {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
		padding: 0.625rem 0.75rem;
		border: 1px solid var(--color-border);
		border-radius: 0.75rem;
		background: var(--color-surface-2);
	}

	.restore-path {
		font-family: var(--font-mono, monospace);
		font-size: 0.82rem;
		word-break: break-all;
	}

	.settings-card-toggle-label {
		flex: 1 1 auto;
	}

	.settings-card-toggle-meta {
		flex: 0 0 auto;
		color: var(--color-text-secondary);
	}

	.settings-card-meta-text {
		max-width: 12rem;
		font-size: 0.75rem;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.settings-card-chevron {
		width: 1rem;
		height: 1rem;
		flex-shrink: 0;
		transition: transform var(--t-normal) ease;
	}

	.settings-card-chevron.open {
		transform: rotate(180deg);
	}

	.settings-card-body {
		margin-top: 0.75rem;
	}

	.setting-row {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		align-items: start;
		gap: 0.75rem;
	}

	.integration-item {
		display: flex;
		align-items: center;
		gap: 0.625rem;
		min-width: 0;
		flex: 1;
	}

	.integration-meta {
		display: flex;
		flex-direction: column;
		gap: 0.125rem;
		min-width: 0;
	}

	.integration-meta .setting-description {
		min-width: 0;
	}

	.integration-meta .setting-description.warning {
		white-space: pre-wrap;
	}

	.integration-sub {
		font-size: 0.6875rem;
		font-family: var(--font-mono);
		color: var(--color-text-muted);
	}
	.warning {
		color: var(--color-warning);
	}
	.install-error-row {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
	}
	.install-error-label {
		color: var(--color-error);
		font-size: 0.8125rem;
	}
	.error-pre {
		margin: 0;
		padding: 0.75rem;
		background: var(--color-settings-control-bg);
		border-radius: var(--radius-sm);
		font-family: var(--font-mono);
		font-size: 0.75rem;
		white-space: pre-wrap;
		word-break: break-word;
		max-height: 50vh;
		overflow: auto;
	}

	.progress-widget {
		display: flex;
		flex-direction: column;
		gap: 0.35rem;
		min-width: 0;
		grid-column: 1 / -1;
	}

	.integration-actions .progress-widget {
		grid-column: 1 / -1;
	}

	.integration-actions :global(.btn) {
		min-width: 0;
	}

	@media (min-width: 901px) {
		.setting-row {
			grid-template-columns: minmax(0, 1fr) auto;
			align-items: start;
			gap: 0.75rem;
		}

		.integration-item {
			display: grid;
			grid-template-columns: 8px minmax(0, 1fr);
			align-items: flex-start;
			column-gap: 0.625rem;
		}

		.integration-item :global(.dot) {
			margin-top: 0.42rem;
		}

		.integration-meta {
			min-width: 0;
		}
	}

	@media (max-width: 640px) {
		.integration-item {
			display: grid;
			grid-template-columns: 8px minmax(0, 1fr);
			align-items: start;
			column-gap: 0.625rem;
		}

		.integration-item :global(.dot) {
			margin-top: 0.42rem;
		}

		@container (max-width: 420px) {
			.setting-row {
				grid-template-columns: minmax(0, 1fr) auto;
				align-items: center;
				gap: 0.625rem;
			}
		}
	}
	.progress-label {
		font-size: 0.78rem;
		color: var(--color-text-primary);
		font-variant-numeric: tabular-nums;
	}
	.progress-bar {
		position: relative;
		height: 6px;
		background: var(--color-settings-control-bg, rgba(0, 0, 0, 0.08));
		border-radius: 3px;
		overflow: hidden;
	}
	.progress-fill {
		position: absolute;
		left: 0;
		top: 0;
		bottom: 0;
		background: var(--color-primary, #3b82f6);
		transition: width 120ms ease-out;
	}
	.progress-bar.indeterminate .progress-fill {
		background: linear-gradient(
			90deg,
			transparent 0%,
			var(--color-primary, #3b82f6) 50%,
			transparent 100%
		);
		background-size: 200% 100%;
		animation: indeterminate-slide 1.2s linear infinite;
		width: 100% !important;
	}
	.progress-widget.progress-error .progress-fill {
		background: var(--color-error, #ef4444);
	}
	.progress-widget.progress-done .progress-fill {
		background: var(--color-success, #10b981);
	}
	@keyframes indeterminate-slide {
		0% { background-position: 200% 0; }
		100% { background-position: -100% 0; }
	}
</style>
