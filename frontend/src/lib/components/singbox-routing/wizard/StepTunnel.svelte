<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import type { AWGTagInfo } from '$lib/types';
	import { singboxWizard } from '$lib/stores/singboxWizard';

	interface Props {
		onAdvance: () => void;
	}
	let { onAdvance }: Props = $props();

	const wizardState = singboxWizard.state;

	let tags = $state<AWGTagInfo[]>([]);
	let loading = $state(true);
	let importContent = $state('');
	let importName = $state('');
	let importing = $state(false);
	let importError = $state('');

	onMount(async () => {
		try {
			tags = await api.getAWGTags();
		} catch {
			tags = [];
		}
		loading = false;
		if (tags.length === 1) {
			singboxWizard.setTunnelTag(tags[0].tag);
			setTimeout(onAdvance, 500);
		}
	});

	function pick(tag: string): void {
		singboxWizard.setTunnelTag(tag);
	}

	async function importTunnel(): Promise<void> {
		const content = importContent.trim();
		if (!content) {
			importError = 'Вставьте wg-quick конфиг';
			return;
		}
		importing = true;
		importError = '';
		try {
			const tunnel = await api.importConfig(content, importName || undefined, 'kernel');
			tags = await api.getAWGTags();
			const newTag = tags.find((t) => t.tag === tunnel.id || t.tag.includes(tunnel.id))?.tag;
			if (newTag) {
				singboxWizard.setTunnelTag(newTag);
				onAdvance();
			} else {
				importError = 'Туннель импортирован, но не найден в списке. Откройте /tunnels.';
			}
		} catch (e) {
			importError = e instanceof Error ? e.message : 'Ошибка импорта';
		} finally {
			importing = false;
		}
	}

	const selected = $derived($wizardState.tunnelTag);
</script>

<div class="title">Через какой туннель пускать трафик?</div>

{#if loading}
	<div class="hint">Загрузка...</div>
{:else if tags.length === 1}
	<div class="toast">Используем туннель <b>{tags[0].tag}</b>. Шаг проскакивается автоматически.</div>
{:else if tags.length > 1}
	<div class="hint">Выберите AWG-туннель, через который пойдут выбранные пресеты.</div>
	<div class="radio-list">
		{#each tags as t (t.tag)}
			<button
				type="button"
				class="radio"
				class:checked={selected === t.tag}
				onclick={() => pick(t.tag)}
			>
				<span class="mark"></span>
				<div class="rad-body">
					<div class="rad-name">{t.tag}</div>
					{#if t.label}<div class="rad-meta mono">{t.label}</div>{/if}
				</div>
			</button>
		{/each}
	</div>
{:else}
	<div class="hint">Туннелей пока нет. Вставьте wg-quick конфиг — мастер импортирует и продолжит.</div>
	<input
		class="input"
		placeholder="Имя туннеля (опционально)"
		bind:value={importName}
		disabled={importing}
	/>
	<textarea
		class="paste"
		bind:value={importContent}
		placeholder={'[Interface]\nPrivateKey = ...\nAddress = 10.0.0.2/24\nDNS = 1.1.1.1\n\n[Peer]\nPublicKey = ...\nEndpoint = 1.2.3.4:51820\nAllowedIPs = 0.0.0.0/0'}
		disabled={importing}
	></textarea>
	{#if importError}
		<div class="err">{importError}</div>
	{/if}
	<button class="primary" type="button" onclick={importTunnel} disabled={importing}>
		{importing ? 'Импортируем...' : 'Импортировать и продолжить'}
	</button>
{/if}

<style>
	.title { font-size: 1.05rem; color: var(--color-text-primary); font-weight: 600; margin-bottom: 0.6rem; }
	.hint { color: var(--color-text-muted); font-size: 0.85rem; margin-bottom: 1rem; }
	.toast {
		background: rgba(63,185,80,0.1);
		border-left: 3px solid #3fb950;
		padding: 0.7rem 1rem;
		border-radius: 4px;
		color: var(--color-text-primary);
		font-size: 0.85rem;
	}
	.radio-list { display: flex; flex-direction: column; gap: 0.5rem; }
	.radio {
		padding: 0.6rem 0.85rem;
		border: 1px solid var(--color-border);
		border-radius: 6px;
		background: var(--color-bg-secondary);
		display: flex;
		align-items: flex-start;
		gap: 0.6rem;
		cursor: pointer;
		font: inherit;
		text-align: left;
		color: var(--color-text-primary);
	}
	.radio.checked { border-color: var(--color-accent); background: rgba(88,166,255,0.06); }
	.mark {
		width: 14px; height: 14px;
		border-radius: 50%;
		border: 2px solid var(--color-border);
		flex-shrink: 0;
		margin-top: 2px;
	}
	.radio.checked .mark {
		border-color: var(--color-accent);
		background: var(--color-accent);
		box-shadow: inset 0 0 0 3px var(--color-bg-secondary);
	}
	.rad-body {
		display: flex;
		flex-direction: column;
		gap: 0.15rem;
		min-width: 0;
		flex: 1;
	}
	.rad-name {
		color: var(--color-text-primary);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.rad-meta {
		font-size: 0.75rem;
		color: var(--color-text-muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.mono { font-family: var(--font-mono, ui-monospace, monospace); }
	.input {
		display: block;
		width: 100%;
		padding: 0.5rem 0.7rem;
		margin-bottom: 0.5rem;
		background: var(--color-bg-primary);
		border: 1px solid var(--color-border);
		border-radius: 4px;
		color: var(--color-text-primary);
	}
	.paste {
		width: 100%;
		min-height: 160px;
		font-family: var(--font-mono, ui-monospace, monospace);
		font-size: 0.78rem;
		padding: 0.7rem;
		background: var(--color-bg-primary);
		border: 1px solid var(--color-border);
		border-radius: 4px;
		color: var(--color-text-primary);
		resize: vertical;
	}
	.err {
		color: #f85149;
		font-size: 0.85rem;
		margin-top: 0.4rem;
	}
	.primary {
		margin-top: 0.7rem;
		padding: 0.5rem 1rem;
		background: #238636;
		color: white;
		border: 1px solid #2ea043;
		border-radius: 6px;
		font: inherit;
		cursor: pointer;
	}
	.primary:disabled { opacity: 0.6; cursor: wait; }
</style>
