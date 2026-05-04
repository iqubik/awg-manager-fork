<script lang="ts">
	import { HAPP_PRESET } from './headersParser';

	interface Props {
		value: string;
	}
	let { value = $bindable('') }: Props = $props();

	function applyPreset(preset: string): void {
		if (value.trim() && !confirm('Заменить текущие заголовки пресетом?')) return;
		value = preset;
	}
</script>

<div class="head">
	<label class="lbl" for="hdr">Заголовки запроса (по одному на строку, формат «Key: Value»)</label>
	<select
		class="preset-picker"
		onchange={(e) => {
			const v = (e.currentTarget as HTMLSelectElement).value;
			if (v === 'happ') applyPreset(HAPP_PRESET);
			(e.currentTarget as HTMLSelectElement).value = '';
		}}
	>
		<option value="">Подставить пресет</option>
		<option value="happ">Happ iOS</option>
	</select>
</div>
<textarea
	id="hdr"
	class="textarea"
	bind:value
	placeholder={'# Пример:\nUser-Agent: Happ/4.6.0/ios/2603181556604\nX-Device-OS: iOS'}
	rows="8"
></textarea>

<style>
	.head {
		display: flex;
		justify-content: space-between;
		align-items: center;
		margin-bottom: 0.4rem;
	}
	.lbl {
		color: var(--color-text-muted);
		font-size: 0.85rem;
	}
	.preset-picker {
		padding: 0.3rem 0.5rem;
		border: 1px solid var(--color-border);
		border-radius: 4px;
		background: var(--color-bg-secondary);
		color: var(--color-text-primary);
		font-size: 0.8rem;
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
