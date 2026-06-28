<script lang="ts">
	import { Modal, Button } from '$lib/components/ui';
	import AwgConfigAnalyzer from '$lib/components/diagnostics/AwgConfigAnalyzer.svelte';
	import { api } from '$lib/api/client';
	import { notifications } from '$lib/stores/notifications';
	import { copyToClipboard } from '$lib/utils/clipboard';
	import QRCode from 'qrcode';

	interface Props {
		open: boolean;
		serverId: string;
		pubkey: string;
		peerName: string;
		kind?: 'managed' | 'system';
		onclose: () => void;
	}

	let { open = $bindable(false), serverId, pubkey, peerName, kind = 'managed', onclose }: Props = $props();

	let conf = $state('');
	let loading = $state(false);
	let viewMode = $state<'conf' | 'qr' | 'analysis'>('conf');
	let qrDataUrl = $state('');
	let qrGenerating = $state(false);
	let loadedForKey = $state('');

	function confLoadKey(): string {
		return `${serverId}\0${pubkey}\0${kind}`;
	}

	// Load once per open cycle / peer — polling updates on the servers page
	// re-run parent effects and would otherwise flash "Загрузка..." and reset QR.
	$effect(() => {
		if (!open || !pubkey) {
			if (!open) loadedForKey = '';
			return;
		}
		const key = confLoadKey();
		if (loadedForKey === key) return;
		loadedForKey = key;
		viewMode = 'conf';
		qrDataUrl = '';
		void loadConf();
	});

	async function loadConf() {
		loading = true;
		try {
			conf = kind === 'system'
				? await api.getSystemServerPeerConf(serverId, pubkey)
				: await api.getManagedPeerConf(serverId, pubkey);
		} catch (e) {
			notifications.error(e instanceof Error ? e.message : 'Ошибка загрузки');
			conf = '';
		} finally {
			loading = false;
		}
	}

	async function toggleQR() {
		if (viewMode === 'qr') {
			viewMode = 'conf';
			return;
		}
		if (!qrDataUrl) {
			qrGenerating = true;
			try {
				qrDataUrl = await QRCode.toDataURL(conf, {
					width: 512,
					margin: 2,
					errorCorrectionLevel: 'L',
					color: { dark: '#000000', light: '#ffffff' }
				});
			} catch (e) {
				const size = new Blob([conf]).size;
				if (size > 2900) {
					notifications.error(`Конфигурация слишком большая для QR-кода (${size} байт). Используйте .conf файл.`, 8000);
				} else {
					notifications.error('Ошибка генерации QR-кода');
				}
				return;
			} finally {
				qrGenerating = false;
			}
		}
		viewMode = 'qr';
	}

	function toggleAnalysis() {
		viewMode = viewMode === 'analysis' ? 'conf' : 'analysis';
	}

	function downloadConf() {
		const name = peerName || 'peer';
		const safeName = name.replace(/[^a-zA-Z0-9а-яА-Я_-]/g, '_');
		const blob = new Blob([conf], { type: 'text/plain' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `${safeName}.conf`;
		a.click();
		URL.revokeObjectURL(url);
	}

	async function copyConf() {
		if (await copyToClipboard(conf)) {
			notifications.success('Скопировано');
		} else {
			notifications.error('Не удалось скопировать');
		}
	}
</script>

<Modal {open} title="Конфигурация клиента" size="md" {onclose}>
	{#if loading}
		<div class="loading">Загрузка...</div>
	{:else if conf}
		{#if viewMode === 'qr' && qrDataUrl}
			<div class="qr-container">
				<img src={qrDataUrl} alt="QR-код конфигурации" class="qr-image" />
				<span class="qr-hint">Отсканируйте в AmneziaWG / WireGuard</span>
			</div>
		{:else if viewMode === 'analysis'}
			<div class="analysis-container">
				<AwgConfigAnalyzer
					layoutMode="embedded"
					embedded
					forceSingleColumn
					initialRaw={conf}
					autoAnalyze
					readonlySource
					allowTunnelSave={false}
					sourceLabel={`Клиент ${peerName}`}
				/>
			</div>
		{:else}
			<pre class="conf-preview">{conf}</pre>
		{/if}
	{:else}
		<div class="loading">Нет данных</div>
	{/if}

	{#snippet actions()}
		<div class="actions-grid">
			<Button variant="secondary" size="md" onclick={toggleQR} disabled={!conf} loading={qrGenerating}>
				{viewMode === 'qr' ? 'Конфиг' : 'QR-код'}
			</Button>
			<Button variant="secondary" size="md" onclick={toggleAnalysis} disabled={!conf}>
				{viewMode === 'analysis' ? 'Конфиг' : 'Проверить'}
			</Button>
			<Button variant="secondary" size="md" onclick={copyConf} disabled={!conf}>
				Копировать
			</Button>
			<Button variant="primary" size="md" onclick={downloadConf} disabled={!conf}>
				Скачать .conf
			</Button>
		</div>
	{/snippet}
</Modal>

<style>
	.actions-grid {
		display: grid;
		grid-template-columns: repeat(4, minmax(0, 1fr));
		gap: 0.5rem;
		width: 100%;
	}

	.actions-grid :global(.btn),
	.actions-grid :global(button),
	.actions-grid :global(a) {
		width: 100%;
		min-width: 0;
	}

	.conf-preview {
		background: var(--bg-primary);
		border: 1px solid var(--border);
		border-radius: 6px;
		padding: 1rem;
		font-size: 0.75rem;
		font-family: var(--font-mono, monospace);
		white-space: pre-wrap;
		word-break: break-all;
		max-height: 400px;
		overflow-y: auto;
		color: var(--text-primary);
		margin: 0;
	}

	.loading {
		padding: 2rem;
		text-align: center;
		color: var(--text-muted);
	}

	.analysis-container {
		max-height: min(78vh, 960px);
		overflow: auto;
		padding-right: 2px;
	}

	.qr-container {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 0.75rem;
		padding: 1.5rem;
	}

	.qr-image {
		width: min(360px, 100%);
		aspect-ratio: 1 / 1;
		height: auto;
		object-fit: contain;
		border-radius: 8px;
		image-rendering: pixelated;
	}

	@media (max-width: 640px) {
		.actions-grid {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}

		.qr-container {
			padding: 1rem;
		}

		.qr-image {
			width: min(320px, 100%);
		}
	}

	.qr-hint {
		font-size: 0.75rem;
		color: var(--text-muted);
	}
</style>
