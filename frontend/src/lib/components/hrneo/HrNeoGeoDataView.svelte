<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import type { GeoFileEntry, DownloadOutbound } from '$lib/types';
	import { settings as appSettings, reloadSettings } from '$lib/stores/settings';
	import { ConfirmModal, Button, Dropdown } from '$lib/components/ui';
	import { geoDownloadProgress } from '$lib/stores/geoDownload';

	interface Props {
		files: GeoFileEntry[];
		onrefresh: () => void;
	}

	let { files, onrefresh }: Props = $props();

	let addUrl = $state('');
	let addType = $state<'geoip' | 'geosite'>('geosite');
	let busy = $state<string | null>(null);
	let err = $state('');
	// URL of the in-flight add — captured at submit time so the progress
	// bar in the Add section keeps tracking the original download even
	// if the user starts typing a different URL into the input or the
	// field gets cleared after success. Without this, $geoDownloadProgress
	// lookup keyed by the live `addUrl` value would lose the bar
	// mid-download.
	let inFlightAddUrl = $state<string | null>(null);
	let outbounds = $state<DownloadOutbound[]>([]);
	let routeSettingsReady = $state(false);
	let routeSettingsError = $state('');
	const routeActionsDisabled = $derived(busy !== null || !routeSettingsReady || !!routeSettingsError);
	const GROUND_ZERRO_GEOIP_URL =
		'https://raw.githubusercontent.com/Ground-Zerro/Geo-Aggregator/main/geodat/geoip_GA.dat';
	const GROUND_ZERRO_GEOSITE_URL =
		'https://raw.githubusercontent.com/Ground-Zerro/Geo-Aggregator/main/geodat/geosite_GA.dat';

	// Progress for the currently in-flight add. Keyed by the submitted
	// URL captured at submit time, not the live input value.
	let progress = $derived(inFlightAddUrl ? ($geoDownloadProgress[inFlightAddUrl] ?? null) : null);
	let progressByPath = $derived($geoDownloadProgress);

	type DownloadOperation = {
		kind: 'add' | 'preset' | 'update' | 'sync';
		target: string;
		routeTag: string;
		routeLabel: string;
	};

	type LastDownload = {
		ok: boolean;
		action: string;
		routeLabel: string;
		message: string;
	};

	let activeDownload = $state<DownloadOperation | null>(null);
	let lastDownload = $state<LastDownload | null>(null);

	function maskSensitiveToken(token: string): string {
		if (!token) return token;
		if (token.length <= 6) return '*'.repeat(token.length);
		return `${token.slice(0, 3)}${'*'.repeat(token.length - 6)}${token.slice(-3)}`;
	}

	function maskSensitiveInText(text: string): string {
		if (!text) return text;
		const hostLike = /\b([a-zA-Z0-9-]+(?:\.[a-zA-Z0-9-]+)+|\d{1,3}(?:\.\d{1,3}){3})\b/g;
		return text.replace(hostLike, (m) => maskSensitiveToken(m));
	}

	function kindLabel(kind?: string): string {
		const k = (kind ?? '').trim();
		if (!k) return '';
		if (k.toLowerCase() === 'subscription') return 'SUB';
		return k.toUpperCase();
	}

	function inferKindFromLabel(label: string): string {
		const s = label.toUpperCase();
		if (s.includes('VLESS')) return 'VLESS';
		if (s.includes('AWG') || s.includes('AMNEZIA')) return 'AWG';
		if (s.includes('SUB')) return 'SUB';
		if (s.includes('WIREGUARD') || s.includes('WG')) return 'WG';
		return '';
	}

	function displayRouteName(label: string, kind?: string): string {
		const base = maskSensitiveInText(label);
		const k = kindLabel(kind) || inferKindFromLabel(base);
		if (!k || k.toLowerCase() === 'direct') return base;
		if (base.toUpperCase().includes(`(${k})`) || base.toUpperCase().endsWith(`- ${k}`)) {
			return base;
		}
		return `${base} (${k})`;
	}

	function currentRouteTag(): string {
		return $appSettings?.download?.routeTag?.trim() || 'direct';
	}

	function outboundLabel(tag: string): string {
		const normalized = tag?.trim() || 'direct';
		const ob = outbounds.find((item) => item.tag === normalized);
		if (ob) {
			return `${displayRouteName(ob.label, ob.kind)}${ob.available ? '' : ' (недоступно)'}`;
		}
		if (normalized === 'direct') {
			return 'Direct (WAN) — без туннеля';
		}
		return `Недоступный маршрут: ${maskSensitiveInText(normalized)}`;
	}

	async function loadOutboundsForDisplay() {
		try {
			const list = await api.listDownloadOutbounds();
			outbounds = list.length ? list : [{ tag: 'direct', kind: 'direct', label: 'Direct (WAN)', available: true }];
		} catch {
			outbounds = [{ tag: 'direct', kind: 'direct', label: 'Direct (WAN)', available: true }];
		}
	}

	async function loadRouteDisplayState() {
		routeSettingsReady = false;
		routeSettingsError = '';
		await loadOutboundsForDisplay();
		if (!$appSettings) {
			const loaded = await reloadSettings();
			if (!loaded) {
				routeSettingsError = 'Не удалось загрузить настройки маршрута загрузок';
				return;
			}
		}
		routeSettingsReady = true;
	}

	function captureDownloadOperation(kind: DownloadOperation['kind'], target: string): DownloadOperation {
		const tag = currentRouteTag();
		return {
			kind,
			target,
			routeTag: tag,
			routeLabel: outboundLabel(tag),
		};
	}

	onMount(() => {
		void loadRouteDisplayState();
	});

	function progressFor(url: string) {
		// Progress events are keyed by the source URL; we look up by the
		// entry's stored URL (not the on-disk filename, which may have a
		// '_N' suffix from resolveConflict).
		return progressByPath[url] ?? null;
	}

	function fmtPercent(p: { downloaded: number; total: number }): string {
		if (p.total <= 0) return '';
		return `${Math.min(100, Math.round((p.downloaded / p.total) * 100))}%`;
	}

	async function add() {
		if (!routeSettingsReady || routeSettingsError) return;
		const submitted = addUrl.trim();
		if (!submitted) return;
		const op = captureDownloadOperation('add', submitted);
		busy = 'add';
		err = '';
		lastDownload = null;
		inFlightAddUrl = submitted;
		activeDownload = op;
		try {
			await api.addGeoFile(addType, submitted);
			addUrl = '';
			lastDownload = {
				ok: true,
				action: 'Добавление geo-файла',
				routeLabel: op.routeLabel,
				message: 'Файл скачан',
			};
			onrefresh();
		} catch (e: unknown) {
			const msg = e instanceof Error ? e.message : String(e);
			err = `Не удалось скачать через «${op.routeLabel}»: ${msg}`;
			lastDownload = {
				ok: false,
				action: 'Добавление geo-файла',
				routeLabel: op.routeLabel,
				message: msg,
			};
		} finally {
			busy = null;
			inFlightAddUrl = null;
			activeDownload = null;
		}
	}

	async function addPreset(type: 'geoip' | 'geosite', url: string) {
		if (!routeSettingsReady || routeSettingsError) return;
		const op = captureDownloadOperation('preset', url);
		busy = 'add';
		err = '';
		lastDownload = null;
		inFlightAddUrl = url;
		activeDownload = op;
		try {
			await api.addGeoFile(type, url);
			lastDownload = {
				ok: true,
				action: 'Добавление пресета',
				routeLabel: op.routeLabel,
				message: 'Пресет скачан',
			};
			onrefresh();
		} catch (e: unknown) {
			const msg = e instanceof Error ? e.message : String(e);
			err = `Не удалось скачать через «${op.routeLabel}»: ${msg}`;
			lastDownload = {
				ok: false,
				action: 'Добавление пресета',
				routeLabel: op.routeLabel,
				message: msg,
			};
		} finally {
			busy = null;
			inFlightAddUrl = null;
			activeDownload = null;
		}
	}

	async function update(path: string) {
		if (!routeSettingsReady || routeSettingsError) return;
		const op = captureDownloadOperation('update', path);
		busy = path;
		err = '';
		lastDownload = null;
		activeDownload = op;
		try {
			await api.updateGeoFile(path);
			lastDownload = {
				ok: true,
				action: `Обновление ${fileName(path)}`,
				routeLabel: op.routeLabel,
				message: 'Файл обновлён',
			};
			onrefresh();
		} catch (e: unknown) {
			const msg = e instanceof Error ? e.message : String(e);
			err = `Не удалось обновить через «${op.routeLabel}»: ${msg}`;
			lastDownload = {
				ok: false,
				action: `Обновление ${fileName(path)}`,
				routeLabel: op.routeLabel,
				message: msg,
			};
		} finally {
			busy = null;
			activeDownload = null;
		}
	}

	let pendingDelete = $state<GeoFileEntry | null>(null);
	let pendingTakeControl = $state<GeoFileEntry | null>(null);
	let expandedPaths = $state<Set<string>>(new Set());

	function requestRemove(f: GeoFileEntry) {
		pendingDelete = f;
	}

	async function syncFromHR() {
		if (!routeSettingsReady || routeSettingsError) return;
		const op = captureDownloadOperation('sync', 'all');
		busy = 'sync';
		err = '';
		lastDownload = null;
		activeDownload = op;
		const notes: string[] = [];
		try {
			try {
				await api.rescanGeoFiles();
			} catch (e: unknown) {
				// Нет HR / hrneo.conf — всё равно обновляем уже известные файлы.
				notes.push(e instanceof Error ? e.message : String(e));
			}
			// Список после rescan — HR External видны даже если update упадёт.
			await onrefresh();

			try {
				const upd = await api.updateGeoFile('');
				await onrefresh();
				if (upd.partial && upd.error) {
					notes.push(
						upd.updated > 0
							? `Обновлено ${upd.updated}, ошибки: ${upd.error}`
							: upd.error,
					);
				}
			} catch (e: unknown) {
				await onrefresh();
				notes.push(e instanceof Error ? e.message : String(e));
			}

			if (notes.length > 0) {
				err = notes.join('; ');
				lastDownload = {
					ok: false,
					action: 'Синхронизация geo-файлов',
					routeLabel: op.routeLabel,
					message: notes.join('; '),
				};
			} else {
				lastDownload = {
					ok: true,
					action: 'Синхронизация geo-файлов',
					routeLabel: op.routeLabel,
					message: 'Синхронизация выполнена',
				};
			}
		} finally {
			busy = null;
			activeDownload = null;
		}
	}

	async function confirmTakeControl() {
		if (!pendingTakeControl) return;
		const f = pendingTakeControl;
		busy = f.path;
		err = '';
		try {
			await api.takeGeoFileControl(f.path);
			pendingTakeControl = null;
			onrefresh();
		} catch (e: unknown) {
			err = e instanceof Error ? e.message : String(e);
		} finally {
			busy = null;
		}
	}

	function canUpdate(f: GeoFileEntry): boolean {
		if (f.external) return false;
		return !!(f.url || f.type === 'geoip' || f.type === 'geosite');
	}

	async function confirmRemove() {
		if (!pendingDelete) return;
		const f = pendingDelete;
		busy = f.path;
		err = '';
		try {
			await api.deleteGeoFile(f.path);
			pendingDelete = null;
			onrefresh();
		} catch (e: unknown) {
			err = e instanceof Error ? e.message : String(e);
		} finally {
			busy = null;
		}
	}

	function humanSize(n: number): string {
		if (n < 1024) return `${n} B`;
		if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
		return `${(n / 1024 / 1024).toFixed(1)} MB`;
	}

	function fileName(p: string): string {
		return p.split('/').pop() ?? p;
	}

	function fileDir(p: string): string {
		const base = fileName(p);
		if (!base || p === base) return '';
		return p.slice(0, p.length - base.length);
	}

	function togglePathExpanded(path: string) {
		const next = new Set(expandedPaths);
		if (next.has(path)) {
			next.delete(path);
		} else {
			next.add(path);
		}
		expandedPaths = next;
	}
</script>

<div class="geo-pane">
	<header class="pane-header">
		<h2>Гео-данные</h2>
		<span class="pane-meta">{files.length} файла</span>
		<Button
			variant="ghost"
			size="sm"
			disabled={routeActionsDisabled}
			loading={busy === 'sync'}
			onclick={syncFromHR}
			title="Подтянуть пути из hrneo.conf (External) и перекачать файлы AWGM (External не трогаем — обновляйте в HR Neo)"
		>
			Синхронизировать
		</Button>
	</header>

	{#if err}<div class="error-banner">{err}</div>{/if}

	{#if files.length === 0}
		<div class="empty">Файлы не загружены. Добавьте URL ниже.</div>
	{:else}
		<div class="files">
			{#each files as f (f.path)}
				{@const fp = progressFor(f.url)}
				<div class="file-row">
					<div class="file-info">
						<span class="file-type type-{f.type}">{f.type}</span>
						<button
							type="button"
							class="file-name"
							title={expandedPaths.has(f.path) ? 'Скрыть путь' : f.path}
							onclick={() => togglePathExpanded(f.path)}
						>
							{#if expandedPaths.has(f.path)}
								<span class="file-path">{fileDir(f.path)}</span><span
									class="file-basename">{fileName(f.path)}</span
								>
							{:else}
								{fileName(f.path)}
							{/if}
						</button>
						{#if f.external}
							<span
								class="file-external"
								title="Данный файл управляется HydraRoute Neo"
							>External</span>
						{/if}
						<span class="file-meta">{humanSize(f.size)} · {f.tagCount} тегов</span>
						{#if busy === f.path && fp}
							<span class="row-progress">
								{#if fp.phase === 'download'}
									{fmtPercent(fp)} {humanSize(fp.downloaded)}
								{:else if fp.phase === 'validate'}
									валидация…
								{/if}
							</span>
						{/if}
					</div>
					<div class="file-actions">
						{#if f.external}
							<Button
								variant="secondary"
								size="sm"
								disabled={busy !== null}
								onclick={() => (pendingTakeControl = f)}
							>
								Взять под управление
							</Button>
						{/if}
						{#if canUpdate(f)}
							<Button
								variant="ghost"
								size="sm"
								disabled={routeActionsDisabled}
								loading={busy === f.path}
								onclick={() => update(f.path)}
							>
								Обновить
							</Button>
						{/if}
						<!-- TODO Phase 1: ghost variant with red danger hover (was .row-danger) -->
						<Button
							variant="ghost"
							size="sm"
							disabled={busy !== null}
							onclick={() => requestRemove(f)}
						>
							Удалить
						</Button>
					</div>
				</div>
			{/each}
		</div>
	{/if}

	<div class="route-box">
		{#if routeSettingsError}
			<div class="route-status route-status-error">
				{routeSettingsError}. Откройте Настройки → Загрузки или обновите страницу.
			</div>
		{:else if !routeSettingsReady}
			<div class="route-status route-status-live">
				Загрузка настроек маршрута geo.dat…
			</div>
		{:else}
			<div class="route-status route-status-live">
				Маршрут загрузки: <strong>{outboundLabel(currentRouteTag())}</strong>.
				Изменяется в <a href="/settings#downloads" data-sveltekit-reload>Настройки → Загрузки</a>.
			</div>
		{/if}
		{#if activeDownload}
			<div class="route-status route-status-live">
				Текущая операция через <strong>{activeDownload.routeLabel}</strong>
			</div>
		{:else if lastDownload}
			<div class="route-status {lastDownload.ok ? 'route-status-ok' : 'route-status-error'}">
				{lastDownload.ok ? 'Последняя операция успешна' : 'Последняя операция завершилась ошибкой'}:
				{lastDownload.action} ({lastDownload.routeLabel}){#if lastDownload.message}
					— {lastDownload.message}
				{/if}
			</div>
		{/if}
	</div>

	<div class="add-form">
		<div class="form-label">Пресеты Ground-Zerro</div>
		<div class="preset-row">
			<Button
				variant="secondary"
				size="sm"
				disabled={routeActionsDisabled}
				onclick={() => addPreset('geoip', GROUND_ZERRO_GEOIP_URL)}
			>
				+ geoip_GA.dat
			</Button>
			<Button
				variant="secondary"
				size="sm"
				disabled={routeActionsDisabled}
				onclick={() => addPreset('geosite', GROUND_ZERRO_GEOSITE_URL)}
			>
				+ geosite_GA.dat
			</Button>
			<span class="preset-hint">Агрегат v2fly + RU-блоклистов, обновляется ежедневно.</span>
		</div>
		<div class="form-label form-label-spaced">Добавить по URL</div>
		<div class="add-row">
			<div class="add-type-select">
				<Dropdown
					bind:value={addType}
					options={[
						{ value: 'geosite' as const, label: 'geosite' },
						{ value: 'geoip' as const, label: 'geoip' },
					]}
					disabled={routeActionsDisabled}
					fullWidth
				/>
			</div>
			<input
				class="form-input"
				type="url"
				placeholder="https://.../{addType}.dat"
				bind:value={addUrl}
				disabled={routeActionsDisabled}
			/>
			<Button
				variant="primary"
				size="sm"
				onclick={add}
				disabled={!addUrl.trim() || routeActionsDisabled}
				loading={busy === 'add'}
			>
				+ Добавить
			</Button>
		</div>
		{#if busy === 'add'}
			<div class="busy-hint">
				{#if progress?.phase === 'download'}
					Скачивание через {activeDownload?.routeLabel ?? outboundLabel(currentRouteTag())}:
					{fmtPercent(progress)} —
					{humanSize(progress.downloaded)}{progress.total > 0
						? ` из ${humanSize(progress.total)}`
						: ''}
				{:else if progress?.phase === 'validate'}
					Валидация файла…
				{:else}
					Подключение через {activeDownload?.routeLabel ?? outboundLabel(currentRouteTag())}…
				{/if}
				<div class="progress-bar">
					{#if progress && progress.total > 0}
						<div
							class="progress-fill"
							style="width: {Math.min(100, (progress.downloaded / progress.total) * 100)}%"
						></div>
					{:else}
						<div class="progress-fill indeterminate"></div>
					{/if}
				</div>
			</div>
		{/if}
		<div class="form-hint">
			Тип <code>{addType}</code> должен соответствовать содержимому. Файл с 0 записей будет отклонён —
			убедитесь что выбран правильный тип для этого URL. Лимит размера: 200 МБ.
		</div>
	</div>
</div>

{#if pendingTakeControl}
	{@const pt = pendingTakeControl}
	<ConfirmModal
		open={true}
		title="Взять под управление"
		message={`Перенести «${fileName(pt.path)}» в каталог awg-manager?`}
		secondary="Файл будет перенесён из директории HydraRoute (/opt/etc/HydraRoute) и дальше управляться из AWGM. Путь в hrneo.conf обновится при синхронизации."
		confirmLabel="Перенести"
		variant="primary"
		busy={busy === pt.path}
		onConfirm={confirmTakeControl}
		onClose={() => (pendingTakeControl = null)}
	/>
{/if}

{#if pendingDelete}
	{@const pd = pendingDelete}
	{@const hrKey = pd.type === 'geosite' ? 'GeoSiteFile' : 'GeoIPFile'}
	<ConfirmModal
		open={true}
		title="Удалить гео-файл"
		message={pd.external
			? `Удалить «${fileName(pd.path)}» с диска? Файл управляется HydraRoute Neo — будет удалён из /opt/etc/HydraRoute, не только из каталога AWGM.`
			: `Удалить «${fileName(pd.path)}»?`}
		filePath={pd.path}
		secondary={`Из hrneo.conf уберётся строка ${hrKey}=${pd.path} (если установлен HydraRoute).`}
		busy={busy === pd.path}
		onConfirm={confirmRemove}
		onClose={() => (pendingDelete = null)}
	/>
{/if}

<style>
	.geo-pane {
		--geo-block-gap: 0.875rem;
		display: flex;
		flex-direction: column;
		gap: var(--geo-block-gap);
	}

	.pane-header {
		display: flex;
		align-items: baseline;
		gap: 10px;
		flex-wrap: wrap;
		padding-bottom: 10px;
		border-bottom: 1px solid var(--border);
	}
	.pane-header :global(button) {
		margin-left: auto;
	}
	.pane-header h2 {
		margin: 0;
		font-size: 1.0625rem;
		color: var(--text-primary);
	}
	.pane-meta {
		color: var(--text-muted);
		font-size: 0.8125rem;
	}

	.error-banner {
		background: rgba(247, 118, 142, 0.1);
		border-left: 3px solid var(--error);
		color: var(--error);
		padding: 8px 12px;
		border-radius: 4px;
		font-size: 0.8125rem;
	}

	.empty {
		padding: 24px;
		text-align: center;
		color: var(--text-muted);
		font-style: italic;
		background: var(--bg-secondary);
		border: 1px dashed var(--border);
		border-radius: 8px;
	}

	.files {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.file-row {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 12px;
		padding: 10px 12px;
		background: var(--bg-secondary);
		border: 1px solid var(--border);
		border-radius: 8px;
	}

	.file-info {
		display: flex;
		align-items: center;
		gap: 10px;
		min-width: 0;
		flex: 1;
	}

	.file-type {
		font-size: 0.6875rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		font-weight: 600;
		padding: 2px 8px;
		border-radius: 10px;
	}
	.type-geosite {
		background: rgba(122, 162, 247, 0.15);
		color: var(--accent);
	}
	.type-geoip {
		background: rgba(125, 207, 255, 0.15);
		color: var(--info);
	}

	.file-name {
		font-family: ui-monospace, monospace;
		color: var(--text-primary);
		font-size: 0.875rem;
		cursor: pointer;
		padding: 0;
		border: none;
		background: none;
		text-align: left;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.file-name:hover .file-basename {
		text-decoration: underline;
	}

	.file-path {
		color: var(--text-muted);
	}

	.file-basename {
		color: var(--text-primary);
	}

	.file-meta {
		color: var(--text-muted);
		font-size: 0.75rem;
	}

	.file-external {
		font-size: 0.6875rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		font-weight: 600;
		padding: 2px 8px;
		border-radius: 10px;
		background: rgba(245, 158, 11, 0.15);
		color: var(--warning, #f59e0b);
		cursor: help;
	}

	.file-actions {
		display: flex;
		gap: 4px;
		flex-shrink: 0;
	}

	.add-form {
		padding: 12px;
		background: var(--bg-secondary);
		border: 1px solid var(--border);
		border-radius: 8px;
	}

	.form-label {
		display: block;
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--text-primary);
		margin-bottom: 6px;
	}

	.form-label-spaced {
		margin-top: var(--geo-block-gap);
	}

	.preset-row {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-wrap: wrap;
	}

	.preset-hint {
		color: var(--text-muted);
		font-size: 0.75rem;
	}

	.add-row {
		display: grid;
		grid-template-columns: auto 1fr auto;
		gap: 6px;
		align-items: center;
	}

	.route-box {
		padding: 12px;
		border: 1px solid var(--border);
		border-radius: 8px;
		background: var(--bg-primary);
	}

	.route-status {
		margin-top: 0.75rem;
		padding: 8px 10px;
		border-radius: 6px;
		font-size: 0.8125rem;
	}

	.route-status-live {
		background: rgba(122, 162, 247, 0.1);
		color: var(--text-primary);
		border-left: 3px solid var(--accent);
	}
	.route-status a {
		color: var(--accent);
		text-decoration: none;
	}
	.route-status a:hover {
		text-decoration: underline;
	}

	.route-status-ok {
		background: rgba(74, 222, 128, 0.1);
		color: var(--text-primary);
		border-left: 3px solid var(--success, #4ade80);
	}

	.route-status-error {
		background: rgba(247, 118, 142, 0.1);
		color: var(--error);
		border-left: 3px solid var(--error);
	}

	.add-type-select {
		min-width: 110px;
	}

	.busy-hint {
		margin-top: 0.75rem;
		padding: 8px 10px;
		background: rgba(122, 162, 247, 0.1);
		border-left: 3px solid var(--accent);
		color: var(--text-primary);
		font-size: 0.8125rem;
		border-radius: 4px;
	}

	.form-hint {
		margin-top: 0.75rem;
		color: var(--text-muted);
		font-size: 0.75rem;
	}
	.form-hint code {
		background: var(--bg-tertiary);
		padding: 0 4px;
		border-radius: 3px;
		font-family: ui-monospace, monospace;
	}

	.progress-bar {
		margin-top: 6px;
		height: 6px;
		background: var(--bg-tertiary);
		border-radius: 3px;
		overflow: hidden;
	}
	.progress-fill {
		height: 100%;
		background: var(--accent);
		border-radius: 3px;
		transition: width 0.2s ease-out;
	}
	.progress-fill.indeterminate {
		width: 30%;
		animation: indeterminate 1.4s linear infinite;
	}
	@keyframes indeterminate {
		0% {
			margin-left: -30%;
		}
		100% {
			margin-left: 100%;
		}
	}

	.row-progress {
		color: var(--accent);
		font-size: 0.75rem;
		font-family: ui-monospace, monospace;
	}

	@media (max-width: 640px) {
		.file-row {
			flex-direction: column;
			align-items: stretch;
			gap: 8px;
		}
		.file-info {
			flex-wrap: wrap;
			row-gap: 4px;
		}
		.file-actions {
			justify-content: flex-end;
		}
		.add-row {
			grid-template-columns: 1fr;
		}
	}
</style>
