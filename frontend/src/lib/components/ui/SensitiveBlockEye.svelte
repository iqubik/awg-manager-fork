<script lang="ts">
	import { Eye, EyeOff } from 'lucide-svelte';

	interface Props {
		hidden?: boolean;
		label: string;
	}

	let { hidden = $bindable(true), label }: Props = $props();
</script>

<button
	type="button"
	class="sensitive-block-eye"
	aria-pressed={!hidden}
	aria-label={hidden ? `Показать чувствительные данные блока ${label}` : `Скрыть чувствительные данные блока ${label}`}
	title={hidden ? `Показать чувствительные данные блока ${label}` : `Скрыть чувствительные данные блока ${label}`}
	onclick={() => {
		hidden = !hidden;
	}}
>
	{#if hidden}
		<EyeOff size={14} strokeWidth={2} aria-hidden="true" />
	{:else}
		<Eye size={14} strokeWidth={2} aria-hidden="true" />
	{/if}
</button>

<style>
	.sensitive-block-eye {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 24px;
		height: 24px;
		padding: 0;
		border: 0;
		border-radius: 6px;
		background: transparent;
		color: var(--color-text-muted, var(--text-muted));
		cursor: pointer;
		transition: color 0.15s ease, background 0.15s ease;
		flex: 0 0 auto;
	}

	.sensitive-block-eye:hover {
		color: var(--color-text-secondary, var(--text-secondary));
		background: color-mix(in srgb, var(--color-text-muted, var(--text-muted)) 10%, transparent);
	}

	.sensitive-block-eye:focus-visible {
		outline: 2px solid var(--color-accent, var(--accent));
		outline-offset: 2px;
	}
</style>
