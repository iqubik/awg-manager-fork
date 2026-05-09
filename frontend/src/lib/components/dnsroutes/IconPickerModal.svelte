<script lang="ts">
	import { Modal, Button } from '$lib/components/ui';
	import {
		QURE_ICONS,
		QURE_CDN_BASE,
		qureIconUrl,
		qureMatchByName,
	} from '$lib/generated/qureIcons';

	interface Props {
		open: boolean;
		iconUrl?: string;
		ruleName: string;
		onclose: () => void;
		onapply: (newUrl: string | null) => void;
	}

	let { open, iconUrl = '', ruleName, onclose, onapply }: Props = $props();

	type Tab = 'catalog' | 'url';

	let tab = $state<Tab>('catalog');
	let search = $state('');
	let selectedQure = $state<string | null>(null);
	let customUrl = $state('');

	// Cached Qure match by ruleName — used by both init effect and the hint.
	let autoMatch = $derived(iconUrl ? null : qureMatchByName(ruleName));
	let trimmedUrl = $derived(customUrl.trim());

	// Initialize state when the modal opens, based on current iconUrl + ruleName.
	$effect(() => {
		if (!open) return;

		search = '';

		if (iconUrl && iconUrl.startsWith(QURE_CDN_BASE)) {
			// Stored URL is a Qure CDN URL — extract icon name
			const match = iconUrl.slice(QURE_CDN_BASE.length + 1).replace(/\.png$/, '');
			tab = 'catalog';
			selectedQure = decodeURIComponent(match);
			customUrl = '';
		} else if (iconUrl) {
			// Custom non-Qure URL
			tab = 'url';
			customUrl = iconUrl;
			selectedQure = null;
		} else {
			// No iconUrl — try auto-match by rule name
			tab = 'catalog';
			customUrl = '';
			selectedQure = autoMatch;
		}
	});

	let filteredIcons = $derived.by(() => {
		const q = search.trim().toLowerCase();
		if (!q) return QURE_ICONS;
		return QURE_ICONS.filter((n) => n.toLowerCase().includes(q));
	});

	let canApply = $derived(
		(tab === 'catalog' && selectedQure !== null) ||
		(tab === 'url' && trimmedUrl !== '')
	);

	function handleApply() {
		let url: string | null = null;
		if (tab === 'catalog' && selectedQure) {
			url = qureIconUrl(selectedQure);
		} else if (tab === 'url' && trimmedUrl) {
			url = trimmedUrl;
		}
		onapply(url);
	}

	function handleReset() {
		onapply(null);
	}

	let autoMatchHint = $derived.by(() => {
		if (iconUrl) return null;
		return autoMatch && tab === 'catalog' && selectedQure === autoMatch
			? `Авто-найдено по имени правила «${ruleName}»`
			: null;
	});
</script>

<Modal {open} {onclose} title="Выбрать иконку" size="lg">
	<div class="picker">
		<div class="tabs" role="tablist" aria-label="Источник иконки">
			<button
				class="tab"
				class:active={tab === 'catalog'}
				onclick={() => (tab = 'catalog')}
				type="button"
				role="tab"
				aria-selected={tab === 'catalog'}
			>
				Каталог Qure
			</button>
			<button
				class="tab"
				class:active={tab === 'url'}
				onclick={() => (tab = 'url')}
				type="button"
				role="tab"
				aria-selected={tab === 'url'}
			>
				Свой URL
			</button>
		</div>

		{#if tab === 'catalog'}
			<div class="search-row">
				<input
					type="text"
					class="search-input"
					placeholder="Поиск (telegram, netflix, github...)"
					aria-label="Поиск иконки"
					bind:value={search}
				/>
				<span class="count">
					{filteredIcons.length} иконок{search ? ' (отфильтровано)' : ''}
				</span>
			</div>

			{#if autoMatchHint}
				<p class="auto-hint">{autoMatchHint}</p>
			{/if}

			<div class="grid">
				{#each filteredIcons as name (name)}
					<button
						class="tile"
						class:selected={selectedQure === name}
						onclick={() => (selectedQure = name)}
						type="button"
						title={name}
					>
						<img src={qureIconUrl(name)} alt={name} loading="lazy" />
						<span class="label">{name.replace(/_/g, ' ')}</span>
					</button>
				{/each}
			</div>
		{:else}
			<div class="url-section">
				<label class="url-label" for="icon-url-input">URL картинки</label>
				<input
					id="icon-url-input"
					type="url"
					class="url-input"
					placeholder="https://example.com/icon.png"
					bind:value={customUrl}
				/>
				<p class="url-hint">
					Принимаются PNG/JPG/WebP/SVG; рекомендуем квадратные иконки 32-128px.
				</p>
				{#if trimmedUrl}
					<div class="url-preview">
						<div class="preview-img">
							<img src={trimmedUrl} alt="" />
						</div>
						<span class="preview-url">{trimmedUrl}</span>
					</div>
				{/if}
			</div>
		{/if}
	</div>

	{#snippet actions()}
		<div class="footer-left">
			{#if iconUrl}
				<Button variant="ghost" size="sm" onclick={handleReset}>&#x21BA; Сбросить (на авто)</Button>
			{/if}
		</div>
		<div class="footer-right">
			<Button variant="ghost" onclick={onclose}>Отмена</Button>
			<Button variant="primary" onclick={handleApply} disabled={!canApply}>Применить</Button>
		</div>
	{/snippet}
</Modal>

<style>
	.picker {
		display: flex;
		flex-direction: column;
		gap: 12px;
		min-height: 380px;
	}
	.tabs {
		display: flex;
		gap: 4px;
		border-bottom: 1px solid var(--border);
	}
	.tab {
		padding: 10px 14px;
		color: var(--text-muted);
		background: transparent;
		border: none;
		border-bottom: 2px solid transparent;
		margin-bottom: -1px;
		cursor: pointer;
		font-size: 0.875rem;
		font-weight: 500;
		font-family: inherit;
	}
	.tab:hover {
		color: var(--text-secondary);
	}
	.tab.active {
		color: var(--accent);
		border-bottom-color: var(--accent);
	}
	.search-row {
		display: flex;
		align-items: center;
		gap: 8px;
	}
	.search-input {
		flex: 1;
		background: var(--bg-secondary);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 8px 12px;
		color: var(--text-primary);
		font-size: 0.875rem;
		font-family: inherit;
	}
	.search-input:focus {
		outline: none;
		border-color: var(--accent);
	}
	.count {
		font-size: 0.75rem;
		color: var(--text-muted);
		flex-shrink: 0;
	}
	.auto-hint {
		font-size: 0.75rem;
		color: var(--accent);
		margin: 0;
	}
	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(72px, 1fr));
		gap: 8px;
		max-height: 360px;
		overflow-y: auto;
	}
	.tile {
		aspect-ratio: 1;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 4px;
		padding: 6px;
		border: 1px solid transparent;
		border-radius: 8px;
		background: transparent;
		cursor: pointer;
		font-family: inherit;
		transition: background 0.12s, border-color 0.12s;
	}
	.tile:hover {
		background: var(--bg-hover);
		border-color: var(--border-hover);
	}
	.tile.selected {
		background: var(--bg-hover);
		border-color: var(--accent);
	}
	.tile img {
		width: 36px;
		height: 36px;
		object-fit: contain;
	}
	.tile .label {
		font-size: 0.625rem;
		color: var(--text-muted);
		max-width: 100%;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.tile.selected .label {
		color: var(--text-primary);
	}
	.url-section {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.url-label {
		font-size: 0.8125rem;
		color: var(--text-muted);
	}
	.url-input {
		background: var(--bg-secondary);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 9px 12px;
		color: var(--text-primary);
		font-size: 0.875rem;
		font-family: inherit;
	}
	.url-input:focus {
		outline: none;
		border-color: var(--accent);
	}
	.url-hint {
		font-size: 0.75rem;
		color: var(--text-muted);
		margin: 0;
	}
	.url-preview {
		margin-top: 8px;
		padding: 12px;
		background: var(--bg-secondary);
		border: 1px solid var(--border);
		border-radius: 6px;
		display: flex;
		gap: 12px;
		align-items: center;
	}
	.preview-img {
		width: 36px;
		height: 36px;
		border-radius: 6px;
		background: var(--bg-tertiary);
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
	}
	.preview-img img {
		width: 32px;
		height: 32px;
		object-fit: contain;
	}
	.preview-url {
		font-size: 0.75rem;
		color: var(--text-muted);
		font-family: monospace;
		word-break: break-all;
	}
	.footer-left {
		flex: 1;
	}
	.footer-right {
		display: flex;
		gap: 8px;
	}
</style>
