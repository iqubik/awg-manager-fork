<script lang="ts">
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
		<BackLink href="/" />
		<div class="title-row">
			<h1 class="page-title">{tunnelName}</h1>
			<span class="badge" class:badge-success={tunnelState === 'running'} class:badge-warning={tunnelState === 'starting' || tunnelState === 'broken' || tunnelState === 'needs_start' || tunnelState === 'needs_stop'} class:badge-muted={tunnelState === 'disabled'} class:badge-error={tunnelState === 'stopped' || tunnelState === 'not_created'}>
				<span class="w-1.5 h-1.5 rounded-full bg-current"></span>
				{tunnelState === 'running' ? 'Работает'
				 : tunnelState === 'starting' ? 'Запускается'
				 : tunnelState === 'needs_start' ? 'Ожидает запуска'
				 : tunnelState === 'needs_stop' ? 'Ожидает остановки'
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
						<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<polyline points="1 4 1 10 7 10"/>
							<polyline points="23 20 23 14 17 14"/>
							<path d="M20.49 9A9 9 0 005.64 5.64L1 10m22 4l-4.64 4.36A9 9 0 013.51 15"/>
						</svg>
					{/snippet}
					<span class="btn-label">Заменить</span>
				</Button>
			</span>
		{/if}
		{#if onExport}
			<span class="icon-action">
				<Button variant="secondary" onclick={onExport}>
					{#snippet iconBefore()}
						<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
							<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
							<polyline points="7 10 12 15 17 10"/>
							<line x1="12" y1="15" x2="12" y2="3"/>
						</svg>
					{/snippet}
					<span class="btn-label">Скачать</span>
				</Button>
			</span>
		{/if}
		{#if onSaveOnly}
			<span class="save-only-action">
				<Button variant="secondary" disabled={saving} iconBefore={saveIcon} onclick={onSaveOnly}>
					Сохранить
				</Button>
			</span>
		{/if}
		{#snippet successIcon()}
			<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				<polyline points="20 6 9 17 4 12"/>
			</svg>
		{/snippet}
		{#snippet errorIcon()}
			<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				<line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/>
			</svg>
		{/snippet}
		{#snippet saveIcon()}
			<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
				<path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z"/>
				<polyline points="17 21 17 13 7 13 7 21"/>
				<polyline points="7 3 7 8 15 8"/>
			</svg>
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
				iconBefore={actionStatus === 'success' ? successIcon : actionStatus === 'error' ? errorIcon : actionStatus === 'loading' ? undefined : playIcon}
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

	/* Badge */
	.badge {
		display: flex;
		align-items: center;
		gap: 6px;
		padding: 4px 10px;
		font-size: 12px;
		font-weight: 500;
		border-radius: 12px;
	}

	.badge-success {
		background: rgba(16, 185, 129, 0.15);
		color: var(--success);
	}

	.badge-error {
		background: rgba(239, 68, 68, 0.15);
		color: var(--error);
	}

	.badge-warning {
		background: rgba(245, 158, 11, 0.15);
		color: var(--warning, #f59e0b);
	}

	.badge-muted {
		background: var(--bg-tertiary);
		color: var(--text-muted);
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
