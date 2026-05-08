<script lang="ts">
	import { DEFAULT_PRESET, HAPP_PRESET } from './headersParser';
	import { Dropdown } from '$lib/components/ui';

	interface Props {
		value: string;
	}
	let { value = $bindable('') }: Props = $props();

	let showHelp = $state(false);

	function applyPreset(preset: string): void {
		if (value.trim() && !confirm('Заменить текущие заголовки пресетом?')) return;
		value = preset;
	}
</script>

<div class="head">
	<label class="lbl" for="hdr">Заголовки запроса</label>
	<div class="head-actions">
		<button type="button" class="help-toggle" onclick={() => (showHelp = !showHelp)}>
			{showHelp ? 'Скрыть подсказку' : 'Что писать?'}
		</button>
		<Dropdown
			placeholder="Подставить пресет"
			options={[
				{ value: 'default', label: 'По умолчанию (Clash / mihomo)' },
				{ value: 'happ', label: 'Happ iOS (если требует провайдер)' },
			]}
			onchange={(v) => {
				if (v === 'happ') applyPreset(HAPP_PRESET);
				else if (v === 'default') applyPreset(DEFAULT_PRESET);
			}}
		/>
	</div>
</div>

{#if showHelp}
	<div class="help">
		<div class="help-row">
			Один заголовок на строку, формат <code>Имя: Значение</code>.
			Пустые строки и строки с <code>#</code> игнорируются.
		</div>
		<div class="help-row">
			<span class="help-lbl">Поддерживаемые заголовки</span> — любые,
			кроме служебных: <code>Connection</code>, <code>Host</code>,
			<code>Content-Length</code>, <code>Transfer-Encoding</code>,
			<code>Upgrade</code>. Они управляются Go-клиентом и тихо игнорируются.
		</div>
		<div class="help-row">
			<span class="help-lbl">Часто требуются провайдерами</span>:
			<code>User-Agent</code>, <code>X-HWID</code>,
			<code>X-Device-OS</code>, <code>X-Device-Locale</code>,
			<code>X-Device-Model</code>, <code>X-Ver-OS</code>,
			<code>X-App-Version</code>, <code>Accept-Encoding</code>,
			<code>X-Real-IP</code>, <code>X-Forwarded-For</code>.
		</div>
	</div>
{/if}

<textarea
	id="hdr"
	class="textarea"
	bind:value
	placeholder={'# Пример:\nUser-Agent: mihomo/v1.19.20'}
	rows="8"
></textarea>

<style>
	.head {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 0.4rem;
		gap: 0.5rem;
	}
	.head-actions {
		display: flex;
		align-items: center;
		gap: 0.5rem;
	}
	.lbl {
		color: var(--color-text-muted);
		font-size: 0.85rem;
	}
	.help-toggle {
		background: transparent;
		border: 1px solid var(--color-border);
		color: var(--color-text-muted);
		font-size: 0.78rem;
		padding: 0.25rem 0.55rem;
		border-radius: 4px;
		cursor: pointer;
	}
	.help-toggle:hover {
		color: var(--color-text-primary);
		border-color: var(--color-text-muted);
	}
	.help {
		margin-bottom: 0.6rem;
		padding: 0.7rem 0.8rem;
		background: var(--color-bg-secondary, var(--color-bg-primary));
		border: 1px solid var(--color-border);
		border-radius: 4px;
		font-size: 0.78rem;
		color: var(--color-text-muted);
		line-height: 1.5;
	}
	.help-row {
		margin-bottom: 0.4rem;
	}
	.help-row:last-child {
		margin-bottom: 0;
	}
	.help-lbl {
		color: var(--color-text-primary);
		font-weight: 500;
	}
	.help code {
		background: var(--color-bg-primary);
		padding: 0.05rem 0.3rem;
		border-radius: 3px;
		font-family: var(--font-mono, ui-monospace, monospace);
		font-size: 0.74rem;
		color: var(--color-text-primary);
	}
	.textarea {
		width: 100%;
		min-height: 180px;
		font-family: var(--font-mono, ui-monospace, monospace);
		font-size: 0.82rem;
		padding: 0.7rem;
		background: var(--color-bg-primary);
		border: 1px solid var(--color-border);
		border-radius: 4px;
		color: var(--color-text-primary);
		resize: vertical;
	}
</style>
