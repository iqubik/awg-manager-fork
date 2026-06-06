<script lang="ts">
	import { SideDrawer, Button } from '$lib/components/ui';
	import { api } from '$lib/api/client';
	import { copyToClipboard } from '$lib/utils/clipboard';
	import { downloadSingboxConfigText } from '$lib/utils/singboxConfigExport';
	import { highlightJson } from '$lib/utils/shareEditorHighlight';
	import { notifications } from '$lib/stores/notifications';

	interface Props {
		open: boolean;
		onClose: () => void;
	}

	let { open, onClose }: Props = $props();

	let json = $state<string>('');
	let loading = $state(false);
	let error = $state<string | null>(null);
	let copied = $state(false);
	let lastLoadedFor = $state(false);
	let highlightedJson = $derived(highlightJson(json));

	async function load() {
		loading = true;
		error = null;
		try {
			const res = await api.singboxGetConfigPreview();
			json = res.json;
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
			json = '';
		} finally {
			loading = false;
		}
	}

	async function onCopy() {
		if (!json) return;
		const ok = await copyToClipboard(json);
		if (ok) {
			copied = true;
			notifications.success('Конфиг скопирован в буфер обмена');
			setTimeout(() => {
				copied = false;
			}, 1500);
		} else {
			notifications.error('Не удалось скопировать');
		}
	}

	function onExport() {
		if (!json) return;
		try {
			downloadSingboxConfigText(json);
			notifications.success('Конфиг скачан');
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : String(e));
		}
	}

	$effect(() => {
		if (open && !lastLoadedFor) {
			lastLoadedFor = true;
			load();
		} else if (!open && lastLoadedFor) {
			lastLoadedFor = false;
		}
	});
</script>

<SideDrawer {open} {onClose} title="Конфиг sing-box" width={720}>
	<div class="content">
		<div class="toolbar">
			<Button variant="secondary" size="sm" onclick={load} disabled={loading}>
				{loading ? 'Загрузка…' : 'Обновить'}
			</Button>
			<Button
				variant="secondary"
				size="sm"
				onclick={onCopy}
				disabled={loading || !json}
			>
				{copied ? 'Скопировано' : 'Копировать'}
			</Button>
			<Button
				variant="primary"
				size="sm"
				onclick={onExport}
				disabled={loading || !json}
			>
				<span class="export-btn-content">
					<svg
						class="export-btn-icon"
						width="14"
						height="14"
						viewBox="0 0 24 24"
						fill="none"
						stroke="currentColor"
						stroke-width="2"
						stroke-linecap="round"
						stroke-linejoin="round"
						aria-hidden="true"
					>
						<path d="M5 4h11l3 3v13H5z" />
						<path d="M8 4v6h8V4" />
						<path d="M9 18h6" />
					</svg>
					<span>Экспорт</span>
				</span>
			</Button>
		</div>

		<div class="state">
			{#if loading && !json}
				<div class="placeholder">Загрузка конфига…</div>
			{:else if error}
				<div class="error">
					<div class="error-title">Не удалось загрузить конфиг</div>
					<div class="error-message">{error}</div>
				</div>
			{:else if json}
				<pre class="json json-highlighted"><code>{@html highlightedJson}</code></pre>
			{:else}
				<div class="placeholder">Конфиг пуст</div>
			{/if}
		</div>
	</div>
</SideDrawer>

<style>
	.content {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
		height: 100%;
	}

	.toolbar {
		display: flex;
		gap: 0.5rem;
		align-items: center;
	}

	.export-btn-content {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 0.4375rem;
	}

	.export-btn-icon {
		flex-shrink: 0;
	}

	@media (max-width: 640px) {
		.toolbar {
			display: grid;
			grid-template-columns: repeat(3, minmax(0, 1fr));
			align-items: stretch;
		}

		.toolbar :global(.btn) {
			width: 100%;
			min-width: 0;
		}
	}

	.state {
		flex: 1;
		min-height: 0;
		display: flex;
		flex-direction: column;
	}

	.placeholder {
		padding: 1.5rem;
		text-align: center;
		color: var(--color-text-secondary);
		font-size: 13px;
		border: 1px dashed var(--color-border);
		border-radius: var(--radius-sm);
	}

	.error {
		padding: 0.875rem 1rem;
		border: 1px solid var(--color-error);
		background: color-mix(in srgb, var(--color-error) 12%, transparent);
		border-radius: var(--radius-sm);
		color: var(--color-text-primary);
	}

	.error-title {
		font-weight: 600;
		font-size: 13px;
		margin-bottom: 0.25rem;
	}

	.error-message {
		font-size: 12px;
		color: var(--color-text-secondary);
		word-break: break-word;
	}

	.json {
		flex: 1;
		margin: 0;
		padding: 0.875rem 1rem;
		background: var(--color-bg-primary);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-sm);
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', monospace;
		font-size: 12px;
		line-height: 1.5;
		color: var(--color-text-primary);
		overflow: auto;
		white-space: pre;
		tab-size: 2;
	}

	.json-highlighted code {
		display: block;
		min-width: fit-content;
	}

	.json-highlighted :global(.hl-json-key) {
		color: #93c5fd;
	}

	.json-highlighted :global(.hl-json-str) {
		color: #86efac;
	}

	.json-highlighted :global(.hl-json-num) {
		color: #fca5a5;
	}

	.json-highlighted :global(.hl-json-lit) {
		color: #c4b5fd;
	}

	.json-highlighted :global(.hl-json-punct) {
		color: var(--color-text-secondary);
	}
</style>
