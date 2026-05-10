<script lang="ts">
	import { untrack } from 'svelte';
	import { api } from '$lib/api/client';
	import type { AWGTagInfo } from '$lib/types';
	import { singboxWizard } from '$lib/stores/singboxWizard';
	import { singboxRouter } from '$lib/stores/singboxRouter';

	interface Props {
		onAdvance: () => void;
	}
	let { onAdvance }: Props = $props();

	const wizardState = singboxWizard.state;
	const optionsStore = singboxRouter.options;

	// Filter out "Специальные" group: 'direct' makes no sense for "through
	// which tunnel?". The wizard only offers actual outbounds.
	const groups = $derived(
		$optionsStore.filter((g) => g.group !== 'Специальные'),
	);

	const totalCount = $derived(
		groups.reduce((sum, g) => sum + g.items.length, 0),
	);

	const selected = $derived($wizardState.tunnelTag);

	// Empty-state import flow — still uses api.getAWGTags() to detect the
	// post-import tunnel and advance automatically.
	let importContent = $state('');
	let importName = $state('');
	let importing = $state(false);
	let importError = $state('');

	// Auto-pick when exactly 1 outbound exists across all groups.
	// Guards on selected via untrack so this effect doesn't re-fire after
	// it sets the tag (which would loop with onAdvance call). Effect re-runs
	// only when groups/totalCount change.
	$effect(() => {
		if (totalCount !== 1) return;
		if (untrack(() => selected)) return;
		const only = groups.flatMap((g) => g.items)[0];
		if (!only) return;
		singboxWizard.setTunnelTag(only.value);
		setTimeout(onAdvance, 500);
	});

	function pick(value: string): void {
		singboxWizard.setTunnelTag(value);
	}

	function primaryLabel(label: string): string {
		// Strip ` · <tag>` and ` (<tag>)` to extract human-friendly part.
		const subBreak = label.indexOf(' · ');
		if (subBreak > 0) return label.slice(0, subBreak);
		const parenBreak = label.indexOf(' (');
		if (parenBreak > 0) return label.slice(0, parenBreak);
		return label;
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
			const tags = await api.getAWGTags();
			const newTag = tags.find((t: AWGTagInfo) => t.tag === tunnel.id || t.tag.includes(tunnel.id))?.tag;
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
</script>

<div class="title">Через какой туннель пускать трафик?</div>

{#if totalCount === 1}
	{@const only = groups.flatMap((g) => g.items)[0]}
	<div class="toast">Используем <b>{primaryLabel(only.label)}</b>. Шаг проскакивается автоматически.</div>
{:else if totalCount > 1}
	<div class="hint">Выберите outbound, через который пойдут выбранные пресеты.</div>
	<div class="groups">
		{#each groups as g (g.group)}
			<div class="group-head">{g.group}</div>
			<div class="radio-list">
				{#each g.items as item (item.value)}
					{@const checked = selected === item.value}
					{@const human = primaryLabel(item.label)}
					<label class="option" class:checked>
						<input
							type="radio"
							name="wizard-tunnel-tag"
							value={item.value}
							{checked}
							onchange={() => pick(item.value)}
						/>
						<span class="option-content">
							<span class="option-name">{human}</span>
							{#if item.value !== human}
								<span class="option-meta">{item.value}</span>
							{/if}
						</span>
						<span class="option-check" aria-hidden="true">
							{#if checked}
								<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
									<polyline points="20 6 9 17 4 12"/>
								</svg>
							{/if}
						</span>
					</label>
				{/each}
			</div>
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

	.radio-list {
		display: flex;
		flex-direction: column;
		gap: 0.375rem;
	}

	.option {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.625rem 0.875rem;
		background: var(--color-bg-secondary);
		border: 1px solid var(--color-border);
		border-radius: 6px;
		cursor: pointer;
		transition: background 0.15s ease, border-color 0.15s ease;
		min-width: 0;
	}
	.option:hover:not(.checked) {
		border-color: var(--color-border-hover);
		background: var(--color-bg-hover);
	}
	.option.checked {
		border-color: var(--color-accent);
		background: rgba(122, 162, 247, 0.08);
	}
	.option input[type='radio'] {
		position: absolute;
		opacity: 0;
		pointer-events: none;
		width: 0;
		height: 0;
	}
	.option-content {
		display: flex;
		flex-direction: column;
		gap: 0.125rem;
		flex: 1;
		min-width: 0;
	}
	.option-name {
		font-size: 0.875rem;
		color: var(--color-text-primary);
		font-weight: 500;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.option-meta {
		font-family: var(--font-mono);
		font-size: 0.6875rem;
		color: var(--color-text-muted);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.option-check {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 18px;
		height: 18px;
		flex-shrink: 0;
		color: var(--color-accent);
	}

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

	.groups {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}
	.group-head {
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		color: var(--color-text-muted);
		padding: 0.25rem 0;
	}
</style>
