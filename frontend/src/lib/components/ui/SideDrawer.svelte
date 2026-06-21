<script module lang="ts">
	let nextDrawerId = 1;
	const openDrawerIds: number[] = [];

	function pushDrawer(id: number) {
		const existing = openDrawerIds.indexOf(id);
		if (existing !== -1) openDrawerIds.splice(existing, 1);
		openDrawerIds.push(id);
	}

	function removeDrawer(id: number) {
		const index = openDrawerIds.indexOf(id);
		if (index !== -1) openDrawerIds.splice(index, 1);
	}

	function isTopDrawer(id: number) {
		return openDrawerIds[openDrawerIds.length - 1] === id;
	}
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import ConfirmModal from './ConfirmModal.svelte';
	import IconButton from './IconButton.svelte';

	interface Props {
		open: boolean;
		onClose: () => void;
		title?: string;
		children: Snippet;
		footer?: Snippet;
		width?: number;
		panelClass?: string;
		bodyClass?: string;
		closeOnBackdrop?: boolean;
		hasUnsavedChanges?: () => boolean;
		mobileCloseFallback?: boolean;
	}

	let {
		open,
		onClose,
		title = '',
		children,
		footer,
		width = 480,
		panelClass = '',
		bodyClass = '',
		closeOnBackdrop = true,
		hasUnsavedChanges,
		mobileCloseFallback = true,
	}: Props = $props();

	/** Min downward drag (px) before the mobile sheet closes on release */
	const SHEET_CLOSE_DRAG_PX = 140;
	const drawerId = nextDrawerId++;

	let confirmOpen = $state(false);
	let backdropEl: HTMLElement | null = $state(null);
	let pointerDownOnBackdrop = false;
	let sheetDragY = $state(0);
	let sheetDragging = $state(false);
	let sheetTouchStartY = 0;

	function attemptClose() {
		let dirty = false;
		try {
			dirty = hasUnsavedChanges?.() === true;
		} catch {
			dirty = false;
		}

		if (dirty) {
			confirmOpen = true;
			return;
		}

		onClose();
	}

	function handleEsc(e: KeyboardEvent) {
		if (!open || e.key !== 'Escape') return;
		if (confirmOpen) return;
		if (!isTopDrawer(drawerId)) return;
		e.preventDefault();
		e.stopImmediatePropagation();
		attemptClose();
	}

	function resetSheetDrag() {
		sheetDragging = false;
		sheetDragY = 0;
		sheetTouchStartY = 0;
	}

	function onSheetTouchStart(e: TouchEvent) {
		if (e.touches.length !== 1) return;
		sheetTouchStartY = e.touches[0].clientY;
		sheetDragging = true;
		sheetDragY = 0;
	}

	function onSheetTouchMove(e: TouchEvent) {
		if (!sheetDragging || e.touches.length !== 1) return;
		const dy = e.touches[0].clientY - sheetTouchStartY;
		sheetDragY = Math.max(0, dy);
		if (sheetDragY > 0) e.preventDefault();
	}

	function onSheetTouchEnd() {
		if (!sheetDragging) return;
		if (sheetDragY >= SHEET_CLOSE_DRAG_PX) attemptClose();
		resetSheetDrag();
	}

	/** touchmove needs { passive: false } for preventDefault while dragging the sheet */
	function sheetSwipeTarget(node: HTMLElement) {
		const opts = { passive: false } as const;
		node.addEventListener('touchstart', onSheetTouchStart, { passive: true });
		node.addEventListener('touchmove', onSheetTouchMove, opts);
		node.addEventListener('touchend', onSheetTouchEnd);
		node.addEventListener('touchcancel', onSheetTouchEnd);
		return {
			destroy() {
				node.removeEventListener('touchstart', onSheetTouchStart);
				node.removeEventListener('touchmove', onSheetTouchMove);
				node.removeEventListener('touchend', onSheetTouchEnd);
				node.removeEventListener('touchcancel', onSheetTouchEnd);
			},
		};
	}

	$effect(() => {
		if (!open) {
			removeDrawer(drawerId);
			resetSheetDrag();
			confirmOpen = false;
			pointerDownOnBackdrop = false;
			return;
		}

		pushDrawer(drawerId);

		return () => {
			removeDrawer(drawerId);
		};
	});

	function handleBackdropPointerDown(e: PointerEvent) {
		pointerDownOnBackdrop = e.target === backdropEl;
	}

	function handleBackdropClick(e: MouseEvent) {
		if (!closeOnBackdrop) return;
		if (confirmOpen) return;
		if (e.target !== backdropEl) return;
		if (!pointerDownOnBackdrop) return;
		pointerDownOnBackdrop = false;
		attemptClose();
	}

	function portal(node: HTMLElement) {
		document.body.appendChild(node);
		return {
			destroy() {
				if (node.parentNode) node.parentNode.removeChild(node);
			},
		};
	}
</script>

<svelte:window onkeydown={handleEsc} />

{#if open}
	<div class="drawer-layer" use:portal>
		<div
			bind:this={backdropEl}
			class="backdrop"
			role="presentation"
			onpointerdown={handleBackdropPointerDown}
			onclick={handleBackdropClick}
		></div>
		<div
			class={`drawer ${panelClass}`.trim()}
			class:sheet-dragging={sheetDragging}
			style="--drawer-width: {width}px; --sheet-drag-y: {sheetDragY}px;"
			role="dialog"
			aria-modal="true"
			aria-label={title}
		>
			<div class="drawer-handle" aria-hidden="true" use:sheetSwipeTarget></div>
			<header class="drawer-header" use:sheetSwipeTarget>
				<h3>{title}</h3>
				<span class="drawer-close">
					<IconButton ariaLabel="Закрыть" onclick={attemptClose}>
						<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
							<path d="M18 6L6 18M6 6l12 12" />
						</svg>
					</IconButton>
				</span>
			</header>
			<div class={`drawer-body ${bodyClass}`.trim()}>
				{@render children()}
			</div>
			{#if footer}
				<div class="drawer-footer">
					{@render footer()}
				</div>
		{:else if mobileCloseFallback}
			<div class="drawer-footer drawer-footer--fallback">
				<button type="button" class="drawer-mobile-close" onclick={attemptClose}>
					Закрыть
				</button>
			</div>
		{/if}
		</div>
		{#if hasUnsavedChanges}
			<ConfirmModal
				open={confirmOpen}
				title="Закрыть без сохранения?"
				message="Все правки будут потеряны."
				confirmLabel="Закрыть"
				cancelLabel="Остаться"
				variant="danger"
				onConfirm={() => {
					confirmOpen = false;
					onClose();
				}}
				onClose={() => {
					confirmOpen = false;
				}}
			/>
		{/if}
	</div>
{/if}

<style>
	.drawer-layer {
		position: fixed;
		inset: 0;
		z-index: var(--z-drawer);
		pointer-events: none;
	}

	.backdrop {
		position: fixed;
		inset: 0;
		background: rgba(0, 0, 0, 0.45);
		z-index: 0;
		animation: fade-in 150ms ease;
	}

	.backdrop,
	.drawer {
		pointer-events: auto;
	}

	.drawer {
		position: fixed;
		top: 0;
		right: 0;
		bottom: 0;
		width: var(--drawer-width);
		max-width: 100%;
		background: var(--color-bg-secondary);
		border-left: 1px solid var(--color-border);
		box-shadow: -2px 0 16px rgba(0, 0, 0, 0.3);
		z-index: 1;
		animation: slide-in-right 200ms ease;
		display: flex;
		flex-direction: column;
		-webkit-overflow-scrolling: touch;
	}

	.drawer-handle {
		display: none;
	}

	.drawer-header {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0.875rem 1rem;
		border-bottom: 1px solid var(--color-border);
	}

	.drawer-header h3 {
		margin: 0;
		font-size: 14px;
		font-weight: 600;
	}

	.drawer-body {
		flex: 1;
		padding: 1rem;
		overflow-y: auto;
	}

	.drawer-body :global(input),
	.drawer-body :global(textarea),
	.drawer-body :global(select) {
		max-width: 100%;
		min-width: 0;
		box-sizing: border-box;
	}

	:global(.drawer-body.drawer-body-fill) {
		padding: 0;
		display: flex;
		flex-direction: column;
		min-height: 0;
		overflow: hidden;
	}

	.drawer-footer {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		padding: 0.75rem 1rem;
		border-top: 1px solid var(--color-border);
		background: var(--color-bg-secondary);
	}

	.drawer-footer > :global(*) {
		min-width: 0;
	}

	.drawer-footer > :global(.drawer-footer-full) {
		width: 100%;
		flex: 1 1 100%;
	}

	.drawer-footer--fallback {
		display: flex;
		border-top: 1px solid var(--color-border);
	}

	.drawer-mobile-close {
		width: auto;
		min-width: 120px;
		min-height: 44px;
		border: 1px solid var(--color-border);
		border-radius: 10px;
		background: var(--color-bg-secondary);
		color: var(--color-text-primary);
		font: inherit;
		font-weight: 600;
		cursor: pointer;
	}

	@keyframes fade-in {
		from { opacity: 0; }
		to { opacity: 1; }
	}

	@keyframes slide-in-right {
		from { transform: translateX(100%); }
		to { transform: translateX(0); }
	}

	@keyframes slide-up {
		from { transform: translateY(100%); }
		to { transform: translateY(0); }
	}

	@media (max-width: 768px) {
		.drawer {
			top: auto !important;
			right: 0 !important;
			bottom: 0 !important;
			left: 0 !important;
			width: 100% !important;
			max-width: none !important;
			height: auto !important;
			max-height: 85vh;
			max-height: 85dvh;
			border-radius: 16px 16px 0 0;
			border-left: none;
			border-right: none;
			border-top: 1px solid var(--color-border);
			box-shadow: 0 -4px 24px rgba(0, 0, 0, 0.3);
			animation: slide-up 220ms ease-out;
			transform: translateY(var(--sheet-drag-y, 0));
			transition: transform 0.2s ease;
		}

		.drawer.sheet-dragging {
			transition: none;
		}

		.drawer-handle {
			display: block;
			flex-shrink: 0;
			width: 100%;
			padding: 10px 0 6px;
			touch-action: none;
			cursor: grab;
		}

		.drawer-handle::before {
			content: '';
			display: block;
			width: 40px;
			height: 4px;
			margin: 0 auto;
			border-radius: 2px;
			background: var(--color-text-muted, var(--text-muted));
			opacity: 0.45;
		}

		.drawer-header {
			touch-action: none;
			cursor: grab;
			user-select: none;
			padding-top: 0.25rem;
		}

		.drawer-close {
			display: none;
		}

		.drawer-body {
			max-height: calc(85vh - 60px);
			max-height: calc(85dvh - 60px);
			overflow-y: auto;
		}

		.drawer-footer {
			display: flex;
			align-items: stretch;
			gap: 0.5rem;
			padding: 0.75rem 1rem max(0.75rem, env(safe-area-inset-bottom));
		}

		.drawer-footer > :global(*) {
			flex: 1 1 0;
		}

		.drawer-footer > :global(.actions-grid),
		.drawer-footer > :global(.drawer-footer-full) {
			flex-basis: 100%;
		}

		.drawer-footer :global(.btn) {
			width: 100%;
			min-width: 0;
			justify-content: center;
		}

		.drawer-footer--fallback {
			display: flex;
			padding-top: 12px;
		}

		.drawer-mobile-close {
			width: 100%;
		}
	}
</style>
