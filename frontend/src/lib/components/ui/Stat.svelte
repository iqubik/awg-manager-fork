<script lang="ts">
	interface Props {
		value: string;
		label: string;
		sub?: string;
		mobileInline?: boolean;
	}

	let { value, label, sub, mobileInline = false }: Props = $props();
</script>

<div class="stat" class:mobile-inline={mobileInline}>
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
		overflow-wrap: anywhere;
	}

	@media (max-width: 760px) {
		.stat {
			padding: 0.5rem 0.625rem;
			gap: 0.125rem;
		}

		.value {
			font-size: 0.9375rem;
		}

		.label {
			font-size: 0.625rem;
			letter-spacing: 0.04em;
		}

		.sub {
			font-size: 0.625rem;
			line-height: 1.25;
		}

		.stat.mobile-inline {
			display: grid;
			grid-template-columns: auto minmax(0, 1fr);
			align-items: center;
			column-gap: 0.25rem;
			row-gap: 0;
			padding: 0.5rem;
			min-width: 0;
			overflow: hidden;
		}

		.stat.mobile-inline .label,
		.stat.mobile-inline .value {
			grid-row: 1;
			align-self: center;
			line-height: 1;
			white-space: nowrap;
		}

		.stat.mobile-inline .label {
			grid-column: 1;
			min-width: 0;
			font-size: 0.5rem;
			letter-spacing: 0.02em;
			text-transform: none;
			color: var(--color-text-secondary);
			overflow: hidden;
			text-overflow: ellipsis;
		}

		.stat.mobile-inline .label::after {
			content: ':';
			margin-left: 0.125rem;
		}

		.stat.mobile-inline .value {
			grid-column: 2;
			min-width: 0;
			font-size: 0.6875rem;
			text-align: right;
			overflow: hidden;
			text-overflow: ellipsis;
		}

		.stat.mobile-inline .sub {
			display: none;
		}
	}
</style>
