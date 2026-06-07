<script lang="ts" module>
	export type StatTileAccent = 'default' | 'warning' | 'success' | 'error';

	export interface StatTile {
		label: string;
		value: string | number;
		accent?: StatTileAccent;
		title?: string;
		onclick?: () => void;
		disabled?: boolean;
	}
</script>

<script lang="ts">
	interface Props {
		tiles: StatTile[];
		columns?: number;
	}

	let { tiles, columns = 4 }: Props = $props();
</script>

<div class="stat-row" style="--stat-row-cols: {columns};">
	{#each tiles as t (t.label)}
		{#if t.onclick}
			<button
				type="button"
				class="tile tile-button"
				class:accent-warning={t.accent === 'warning'}
				class:accent-success={t.accent === 'success'}
				class:accent-error={t.accent === 'error'}
				disabled={t.disabled}
				title={t.title ?? ''}
				onclick={t.onclick}
			>
				<div class="tile-label">{t.label}</div>
				<div class="tile-value">{t.value}</div>
			</button>
		{:else}
			<div
				class="tile"
				class:accent-warning={t.accent === 'warning'}
				class:accent-success={t.accent === 'success'}
				class:accent-error={t.accent === 'error'}
				title={t.title ?? ''}
			>
				<div class="tile-label">{t.label}</div>
				<div class="tile-value">{t.value}</div>
			</div>
		{/if}
	{/each}
</div>

<style>
	.stat-row {
		display: grid;
		grid-template-columns: repeat(var(--stat-row-cols, 4), minmax(0, 1fr));
		gap: 0.625rem;
	}

	.tile {
		padding: 0.625rem 0.875rem;
		background: var(--color-bg-secondary);
		border: 1px solid var(--color-border);
		border-radius: var(--radius);
		min-height: 4.25rem;
		display: flex;
		flex-direction: column;
		justify-content: space-between;
		min-width: 0;
	}

	.tile-button {
		font-family: inherit;
		text-align: left;
		cursor: pointer;
		transition: border-color 0.15s ease, background 0.15s ease;
	}

	.tile-button:hover:not(:disabled) {
		background: var(--color-bg-hover);
	}

	.tile-button:disabled {
		cursor: not-allowed;
		opacity: 0.7;
	}

	.accent-warning {
		border-color: var(--color-warning);
	}
	.accent-success {
		border-color: var(--color-success);
	}
	.accent-error {
		border-color: var(--color-error);
	}

	.tile-label {
		font-size: 0.6875rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--color-text-muted);
		margin-bottom: 0.25rem;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.tile-value {
		font-family: var(--font-mono, ui-monospace, monospace);
		font-size: 1.125rem;
		color: var(--color-text-primary);
		font-weight: 500;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	@media (max-width: 720px) {
		.stat-row {
			grid-template-columns: repeat(2, minmax(0, 1fr));
			align-items: stretch;
		}

		.tile {
			min-height: 4.25rem;
		}
	}
</style>
