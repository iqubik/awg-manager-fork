<script lang="ts">
	import AmneziaConfEditor from './AmneziaConfEditor.svelte';
	import VpnLinkPasteImport from './VpnLinkPasteImport.svelte';
	import { notifications } from '$lib/stores/notifications';
	import {
		getVpnPastePresentation,
		PREMIUM_VPN_KEY_STORAGE
	} from '$lib/utils/amneziaPremiumVpnPaste';

	export type TunnelImportTab = 'file' | 'paste' | 'vpn';

	interface CountryConfigMeta {
		suggestedName?: string;
		countryCode: string;
		countryLabel: string;
	}

	interface Props {
		variant?: 'page' | 'modal';
		importContent?: string;
		activeTab?: TunnelImportTab;
		vpnPasteInput?: string;
		linkPreview?: string;
		storageKey?: string;
		loadStoredKeyOnMount?: boolean;
		pastePlaceholder?: string;
		oncountryconfig?: (config: string, meta: CountryConfigMeta) => void | Promise<void>;
		onregularconfig?: (meta: { suggestedName?: string }) => void;
		/** Вызывается после успешного чтения файла (например, подсказка имени). */
		onfileloaded?: (file: File, content: string) => void;
	}

	let {
		variant = 'page',
		importContent = $bindable(''),
		activeTab = $bindable<TunnelImportTab>('file'),
		vpnPasteInput = $bindable(''),
		linkPreview = $bindable(''),
		storageKey = PREMIUM_VPN_KEY_STORAGE,
		loadStoredKeyOnMount = true,
		pastePlaceholder = '[Interface]\nPrivateKey = ...\nAddress = 10.0.0.2/32\n\n[Peer]\nPublicKey = ...\nEndpoint = vpn.example.com:51820\nAllowedIPs = 0.0.0.0/0',
		oncountryconfig,
		onregularconfig,
		onfileloaded
	}: Props = $props();

	let fileInput = $state<HTMLInputElement>();
	let dragOver = $state(false);
	let vpnPasteImport = $state<VpnLinkPasteImport>();

	let vpnPastePresentation = $derived(getVpnPastePresentation(vpnPasteInput));

	function handleFileSelect(event: Event) {
		const input = event.target as HTMLInputElement;
		if (input.files?.[0]) {
			readFile(input.files[0]);
		}
	}

	function handleDrop(event: DragEvent) {
		event.preventDefault();
		dragOver = false;
		if (event.dataTransfer?.files?.[0]) {
			readFile(event.dataTransfer.files[0]);
		}
	}

	function handleDragOver(event: DragEvent) {
		event.preventDefault();
		dragOver = true;
	}

	function handleDragLeave() {
		dragOver = false;
	}

	function readFile(file: File) {
		const reader = new FileReader();
		reader.onload = (e) => {
			const content = e.target?.result as string;
			if (content && content.trim()) {
				importContent = content;
				onfileloaded?.(file, content);
				notifications.success(`Файл "${file.name}" загружен`);
			} else {
				notifications.error(`Файл "${file.name}" пуст`);
			}
		};
		reader.onerror = () => {
			notifications.error('Не удалось прочитать файл');
		};
		reader.readAsText(file);
	}

	export function activateVpnTab() {
		activeTab = 'vpn';
		void vpnPasteImport?.analyzeNow();
	}
</script>

<div class="import-panel" class:import-panel--modal={variant === 'modal'}>
	<div class="tabs">
		<button
			type="button"
			class="tab"
			class:tab-active={activeTab === 'file'}
			onclick={() => (activeTab = 'file')}
		>
			<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
				<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
				<polyline points="17 8 12 3 7 8"/>
				<line x1="12" y1="3" x2="12" y2="15"/>
			</svg>
			{variant === 'page' ? 'Загрузить файл' : 'Файл'}
		</button>
		<button
			type="button"
			class="tab"
			class:tab-active={activeTab === 'paste'}
			onclick={() => (activeTab = 'paste')}
		>
			<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16">
				<path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/>
				<rect x="8" y="2" width="8" height="4" rx="1" ry="1"/>
			</svg>
			Вставить текст
		</button>
		<button type="button" class="tab" class:tab-active={activeTab === 'vpn'} onclick={activateVpnTab}>
			{#if vpnPastePresentation.kind === 'premium'}
				<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" width="16" height="16" aria-hidden="true">
					<path d="M11.562 3.266a.5.5 0 0 1 .876 0L15.39 8.87a1 1 0 0 0 1.516.294L21.183 5.5a.5.5 0 0 1 .798.519l-2.834 10.246a1 1 0 0 1-.956.734H5.81a1 1 0 0 1-.957-.734L2.078 6.02a.5.5 0 0 1 .798-.519l4.276 3.664a1 1 0 0 0 1.516-.294z" />
				</svg>
			{:else}
				<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" width="16" height="16" aria-hidden="true">
					<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/>
					<path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/>
				</svg>
			{/if}
			{vpnPastePresentation.label}
		</button>
	</div>

	<div class="tab-content">
		{#if activeTab === 'file'}
			<div
				class="file-drop-zone"
				class:drag-over={dragOver}
				class:has-content={!!importContent.trim()}
				ondrop={handleDrop}
				ondragover={handleDragOver}
				ondragleave={handleDragLeave}
				role="button"
				tabindex="0"
				onclick={() => fileInput?.click()}
				onkeydown={(e) => e.key === 'Enter' && fileInput?.click()}
			>
				<input
					type="file"
					accept=".conf,text/plain,application/octet-stream"
					bind:this={fileInput}
					onchange={handleFileSelect}
					style="display: none"
				>
				{#if importContent.trim()}
					<div class="drop-content">
						<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width={variant === 'page' ? 48 : 36} height={variant === 'page' ? 48 : 36}>
							<polyline points="20 6 9 17 4 12"/>
						</svg>
						<div class="drop-text">
							<p class="drop-title">Файл загружен</p>
							<p class="drop-hint">Нажмите чтобы заменить</p>
						</div>
					</div>
				{:else}
					<div class="drop-content">
						<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" width={variant === 'page' ? 48 : 36} height={variant === 'page' ? 48 : 36}>
							<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
							<polyline points="17 8 12 3 7 8"/>
							<line x1="12" y1="3" x2="12" y2="15"/>
						</svg>
						<div class="drop-text">
							<p class="drop-title">Перетащите .conf файл сюда</p>
							<p class="drop-hint">или нажмите для выбора</p>
						</div>
					</div>
				{/if}
			</div>
		{:else if activeTab === 'paste'}
			<AmneziaConfEditor
				bind:value={importContent}
				variant={variant === 'modal' ? 'modal' : 'page'}
				placeholder={pastePlaceholder}
			/>
		{:else if activeTab === 'vpn'}
			<VpnLinkPasteImport
				bind:this={vpnPasteImport}
				bind:value={vpnPasteInput}
				bind:configContent={importContent}
				bind:linkPreview
				{storageKey}
				{variant}
				{loadStoredKeyOnMount}
				{oncountryconfig}
				{onregularconfig}
			/>
		{/if}
	</div>
</div>

<style>
	.import-panel--modal {
		--import-border: var(--border);
		--import-text-muted: var(--text-muted);
		--import-text-secondary: var(--text-secondary);
		--import-accent: var(--accent);
		--import-bg-tertiary: var(--bg-tertiary);
		--import-success: var(--success);
	}

	.import-panel:not(.import-panel--modal) {
		--import-border: var(--color-border);
		--import-text-muted: var(--color-text-muted);
		--import-text-secondary: var(--color-text-secondary);
		--import-accent: var(--color-accent);
		--import-bg-tertiary: var(--color-bg-tertiary);
		--import-success: var(--color-success);
	}

	.tabs {
		display: flex;
		border-bottom: 1px solid var(--import-border);
		gap: 0;
	}

	.tab {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: var(--import-tab-pad, 10px 16px);
		font-size: var(--import-tab-font, 13px);
		font-weight: 500;
		color: var(--import-text-muted);
		background: none;
		border: none;
		border-bottom: 2px solid transparent;
		cursor: pointer;
		transition: all 0.15s;
		margin-bottom: -1px;
	}

	.import-panel--modal .tab {
		--import-tab-pad: 8px 14px;
		--import-tab-font: 0.8125rem;
	}

	.tab:hover {
		color: var(--import-text-secondary);
	}

	.tab-active {
		color: var(--import-accent);
		border-bottom-color: var(--import-accent);
	}

	.tab-content {
		margin-top: 16px;
		padding: 0;
	}

	.file-drop-zone {
		min-height: var(--import-drop-min-h, 220px);
		border: 2px dashed var(--import-border);
		border-radius: 8px;
		display: flex;
		align-items: center;
		justify-content: center;
		cursor: pointer;
		transition: all 0.15s ease;
		background: transparent;
	}

	.import-panel--modal .file-drop-zone {
		--import-drop-min-h: 140px;
	}

	.file-drop-zone:hover {
		border-color: var(--import-accent);
		background: var(--import-bg-tertiary);
	}

	.file-drop-zone.drag-over {
		border-color: var(--import-accent);
		background: rgba(122, 162, 247, 0.1);
	}

	.file-drop-zone.has-content {
		border-color: var(--import-success);
		border-style: solid;
	}

	.file-drop-zone.has-content svg {
		color: var(--import-success);
	}

	.drop-content {
		display: flex;
		align-items: center;
		gap: var(--import-drop-gap, 16px);
	}

	.import-panel--modal .drop-content {
		--import-drop-gap: 12px;
	}

	.drop-content svg {
		color: var(--import-text-muted);
		flex-shrink: 0;
	}

	.drop-title {
		font-size: var(--import-drop-title, 17px);
		font-weight: 500;
		color: var(--color-text-primary, var(--text-primary));
		margin-bottom: 4px;
	}

	.import-panel--modal .drop-title {
		--import-drop-title: 0.875rem;
		margin-bottom: 2px;
	}

	.drop-hint {
		font-size: var(--import-drop-hint, 14px);
		color: var(--import-text-muted);
	}

	.import-panel--modal .drop-hint {
		--import-drop-hint: 0.75rem;
	}

	@media (max-width: 640px) {
		.import-panel:not(.import-panel--modal) .tabs {
			flex-direction: column;
			align-items: stretch;
			gap: 6px;
			border-bottom: none;
			margin-bottom: 4px;
		}

		.import-panel:not(.import-panel--modal) .tab {
			width: 100%;
			justify-content: flex-start;
			margin-bottom: 0;
			border: 1px solid var(--import-border);
			border-radius: var(--radius-sm);
		}

		.import-panel:not(.import-panel--modal) .tab-active {
			background: var(--color-accent-tint);
			border-color: var(--import-accent);
			border-bottom-color: var(--import-accent);
		}
	}
</style>
