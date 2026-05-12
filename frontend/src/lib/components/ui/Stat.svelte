<script lang="ts">
	interface Props {
		value: string;
		label: string;
		sub?: string;
	}

	let { value, label, sub }: Props = $props();
</script>

<div class="stat">
	<span class="value">{value}</span>
	<span class="label">{label}</span>
	{#if sub}
		<span class="sub">{sub}</span>
	{/if}
</div>

<style>
	.stat {
		display: flex;
		flex-direction: column;
		gap: 0.1875rem;
		padding: 0.75rem 0.875rem;
		min-width: 0;
	}

	.value {
		font: 700 1.125rem/1 var(--font-sans);
		letter-spacing: -0.02em;
		color: var(--color-accent);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.label {
		font: 600 0.6875rem/1.2 var(--font-sans);
		letter-spacing: 0.05em;
		text-transform: uppercase;
		color: var(--color-text-secondary);
	}

	.sub {
		font: 400 0.6875rem/1.3 var(--font-mono);
		color: var(--color-text-muted);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}
</style>
