<script lang="ts">
	import { Check, Download, RefreshCw, Save, SaveAll, X } from 'lucide-svelte';
	import { Button, BackLink, type ButtonVariant } from '$lib/components/ui';

	type ActionStatus = 'loading' | 'success' | 'error';

	interface Props {
		tunnelName: string;
		tunnelState: string;
		saving: boolean;
		actionStatus: ActionStatus | null;
		onReplace?: () => void;
		onExport?: () => void;
		onSaveOnly?: () => void;
		onSaveAndStart: () => void;
	}

	let {
		tunnelName,
		tunnelState,
		saving,
		actionStatus,
		onReplace,
		onExport,
		onSaveOnly,
		onSaveAndStart
	}: Props = $props();

	const primaryVariant = $derived<ButtonVariant>(
		actionStatus === 'success' ? 'success' :
		actionStatus === 'error' ? 'danger' :
		'primary'
	);
</script>

<div class="sticky-header">
	<div class="header-left">
		<BackLink href="/" variant="accent" />
		<div class="title-row">
			<h1 class="page-title">{tunnelName}</h1>
			<span
				class="tunnel-state-badge"
				class:state-running={tunnelState === 'running'}
				class:state-starting={tunnelState === 'starting'}
				class:state-warning={tunnelState === 'needs_start' || tunnelState === 'needs_stop' || tunnelState === 'stopping'}
				class:state-disabled={tunnelState === 'disabled'}
				class:state-broken={tunnelState === 'broken'}
				class:state-stopped={tunnelState === 'stopped' || tunnelState === 'not_created'}
			>
				<span class="state-dot" aria-hidden="true"></span>
				{tunnelState === 'running' ? 'Работает'
				 : tunnelState === 'starting' ? 'Запускается'
				 : tunnelState === 'needs_start' ? 'Ожидает запуска'
				 : tunnelState === 'needs_stop' ? 'Ожидает остановки'
				 : tunnelState === 'stopping' ? 'Останавливается'
				 : tunnelState === 'disabled' ? 'Отключён'
				 : tunnelState === 'broken' ? 'Сломан'
				 : 'Остановлен'}
			</span>
		</div>
	</div>

	<div class="header-actions">
		{#if onReplace}
			<span class="icon-action">
				<!-- TODO Phase 1: secondary variant with accent-tinted border (was .btn-replace) -->
				<Button variant="secondary" onclick={onReplace}>
					{#snippet iconBefore()}
						<RefreshCw size={16} strokeWidth={2} aria-hidden="true" />
					{/snippet}
					<span class="btn-label">Заменить</span>
				</Button>
			</span>
		{/if}
		{#if onExport}
			<span class="icon-action">
				<Button variant="secondary" onclick={onExport}>
					{#snippet iconBefore()}
						<Download size={16} strokeWidth={2} aria-hidden="true" />
					{/snippet}
					<span class="btn-label">Скачать</span>
				</Button>
			</span>
		{/if}
		{#if onSaveOnly}
			<span class="save-only-action">
				<Button variant="secondary" disabled={saving} onclick={onSaveOnly}>
					{#snippet iconBefore()}
						<Save size={16} strokeWidth={2} aria-hidden="true" />
					{/snippet}
					<span class="btn-label">Сохранить</span>
				</Button>
			</span>
		{/if}
		{#snippet successIcon()}
			<Check size={16} strokeWidth={2} aria-hidden="true" />
		{/snippet}
		{#snippet errorIcon()}
			<X size={16} strokeWidth={2} aria-hidden="true" />
		{/snippet}
		{#snippet saveIcon()}
			<SaveAll size={16} strokeWidth={2} aria-hidden="true" />
		{/snippet}
		{#snippet playIcon()}
			<svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
				<path d="M8 5.14v13.72a1 1 0 0 0 1.5.86l10.5-6.86a1 1 0 0 0 0-1.72L9.5 4.28A1 1 0 0 0 8 5.14z" />
			</svg>
		{/snippet}
		<span class="primary-action">
			<Button
				variant={primaryVariant}
				disabled={saving}
				loading={actionStatus === 'loading'}
				iconBefore={actionStatus === 'success'
					? successIcon
					: actionStatus === 'error'
						? errorIcon
						: actionStatus === 'loading'
							? undefined
							: tunnelState === 'running'
								? saveIcon
								: playIcon}
				onclick={onSaveAndStart}
			>
				{#if actionStatus === 'loading'}
					Сохранение...
				{:else if actionStatus === 'success'}
					Сохранено
				{:else if actionStatus === 'error'}
					Ошибка
				{:else}
					{tunnelState === 'running' ? 'Сохранить и перезапустить' : 'Сохранить и запустить'}
				{/if}
			</Button>
		</span>
	</div>
</div>

<style>
	/* Sticky header */
	.sticky-header {
		position: sticky;
		top: 56px;
		z-index: var(--z-sticky-secondary);
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 16px;
		padding: 12px 16px;
		margin: -16px -16px 20px -16px;
		background: var(--bg-primary);
		border-bottom: 1px solid var(--border);
		flex-wrap: wrap;
	}

	.tunnel-state-badge {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		min-height: 24px;
		padding: 3px 9px;
		border-radius: 999px;
		font-size: 12px;
		font-weight: 600;
		line-height: 1;
		border: 1px solid transparent;
		white-space: nowrap;
	}

	.state-dot {
		width: 6px;
		height: 6px;
		border-radius: 999px;
		background: currentColor;
		box-shadow: 0 0 8px currentColor;
	}

	.tunnel-state-badge.state-running {
		color: var(--color-success, var(--success));
		background: var(--color-success-tint, rgba(16, 185, 129, 0.15));
		border-color: var(--color-success-border, rgba(16, 185, 129, 0.3));
	}

	.tunnel-state-badge.state-starting {
		color: var(--color-info, #38bdf8);
		background: var(--color-info-tint, rgba(56, 189, 248, 0.14));
		border-color: var(--color-info-border, rgba(56, 189, 248, 0.28));
	}

	.tunnel-state-badge.state-starting .state-dot {
		animation: tunnel-state-pulse 1.2s ease-in-out infinite;
	}

	.tunnel-state-badge.state-warning {
		color: var(--color-warning, var(--warning, #f59e0b));
		background: var(--color-warning-tint, rgba(245, 158, 11, 0.15));
		border-color: var(--color-warning-border, rgba(245, 158, 11, 0.3));
	}

	.tunnel-state-badge.state-disabled {
		color: var(--color-text-muted, var(--text-muted));
		background: var(--color-muted-tint, var(--bg-tertiary));
		border-color: var(--color-muted-border, var(--border));
	}

	.tunnel-state-badge.state-broken {
		color: var(--color-broken, var(--color-error, var(--error)));
		background: var(--color-broken-tint, var(--color-error-tint, rgba(239, 68, 68, 0.15)));
		border-color: var(--color-broken-border, var(--color-error-border, rgba(239, 68, 68, 0.3)));
	}

	.tunnel-state-badge.state-stopped {
		color: var(--color-error, var(--error));
		background: var(--color-error-tint, rgba(239, 68, 68, 0.15));
		border-color: var(--color-error-border, rgba(239, 68, 68, 0.3));
	}

	@keyframes tunnel-state-pulse {
		0%, 100% {
			opacity: 0.55;
		}

		50% {
			opacity: 1;
		}
	}

	.header-left {
		display: flex;
		align-items: center;
		gap: 16px;
		min-width: 0;
	}

	.title-row {
		display: flex;
		align-items: center;
		gap: 10px;
		min-width: 0;
	}

	.header-actions {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
	}

	.page-title {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-size: 18px;
		font-weight: 600;
	}
	@media (max-width: 600px) {
		.sticky-header {
			padding: 10px 12px;
			margin: -12px -12px 16px -12px;
			align-items: stretch;
			overflow-x: clip;
		}

		.header-left {
			flex-wrap: nowrap;
			gap: 8px;
			width: 100%;
		}

		.title-row {
			min-width: 0;
			flex: 1;
		}

		.page-title {
			font-size: 16px;
		}

		.header-actions {
			width: 100%;
			display: grid;
			grid-template-columns: repeat(2, minmax(0, 1fr));
			gap: 8px;
			align-items: stretch;
		}

		.header-actions :global(.btn) {
			width: 100%;
			min-width: 0;
		}

		.icon-action,
		.save-only-action,
		.primary-action {
			min-width: 0;
		}

		.icon-action :global(.btn) {
			display: inline-flex;
			align-items: center;
			justify-content: center;
			gap: 0.375rem;
			padding-inline: 0.5rem;
			font-size: 12px;
		}

		.save-only-action :global(.btn),
		.primary-action :global(.btn) {
			width: 100%;
		}

		.primary-action :global(.btn) {
			font-size: 12px;
			white-space: nowrap;
			overflow: hidden;
			text-overflow: ellipsis;
		}

	}
</style>
