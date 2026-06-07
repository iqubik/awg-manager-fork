<script lang="ts">
	interface Props {
		children?: import('svelte').Snippet;
	}

	let { children }: Props = $props();
</script>

<span class="tunnel-field-label">
	{#if children}
		{@render children()}
	{/if}
</span>

<style>
	.tunnel-field-label {
		font-size: var(--tunnel-card-label-size, var(--sbx-card-label, 10px));
		color: var(--color-text-muted);
		text-transform: uppercase;
		letter-spacing: 0.04em;
	}
</style>
