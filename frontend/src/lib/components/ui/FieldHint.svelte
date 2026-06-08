<script lang="ts">
	import { tick } from 'svelte';

	interface Props {
		text: string;
		ariaLabel?: string;
	}

	let { text, ariaLabel }: Props = $props();

	let open = $state(false);
	let rootEl = $state<HTMLElement | null>(null);
	let triggerEl = $state<HTMLButtonElement | null>(null);
	let popupEl = $state<HTMLDivElement | null>(null);

	let popupTop = $state(0);
	let popupLeft = $state(0);

	const VIEWPORT_PADDING = 12;
	const POPUP_GAP = 6;

	function portal(node: HTMLElement, target: HTMLElement = document.body) {
		target.appendChild(node);
		return {
			destroy() {
				if (node.parentNode === target) {
					target.removeChild(node);
				}
			},
		};
	}

	function toggle() {
		open = !open;
	}

	function closeOnOutside(e: MouseEvent) {
		if (!open || !rootEl) return;
		const target = e.target as Node;
		if (rootEl.contains(target) || popupEl?.contains(target)) return;
		open = false;
	}

	function closeOnEscape(e: KeyboardEvent) {
		if (e.key === 'Escape') open = false;
	}

	function reposition() {
		if (!open || !triggerEl) return;

		const rect = triggerEl.getBoundingClientRect();
		const popupWidth = popupEl?.offsetWidth ?? Math.min(288, window.innerWidth - VIEWPORT_PADDING * 2);
		const popupHeight = popupEl?.offsetHeight ?? 0;

		let left = rect.left;
		left = Math.max(
			VIEWPORT_PADDING,
			Math.min(left, window.innerWidth - popupWidth - VIEWPORT_PADDING),
		);

		let top = rect.bottom + POPUP_GAP;
		if (popupHeight > 0 && top + popupHeight > window.innerHeight - VIEWPORT_PADDING) {
			top = Math.max(VIEWPORT_PADDING, rect.top - popupHeight - POPUP_GAP);
		}

		popupLeft = left;
		popupTop = top;
	}

	$effect(() => {
		if (!open) return;

		if (triggerEl) {
			const rect = triggerEl.getBoundingClientRect();
			popupLeft = Math.max(VIEWPORT_PADDING, rect.left);
			popupTop = rect.bottom + POPUP_GAP;
		}

		let ready = false;
		const run = async () => {
			await tick();
			reposition();
			ready = true;
		};
		void run();

		const handleChange = () => {
			if (ready) reposition();
		};

		window.addEventListener('resize', handleChange);
		window.addEventListener('scroll', handleChange, true);
		return () => {
			window.removeEventListener('resize', handleChange);
			window.removeEventListener('scroll', handleChange, true);
		};
	});
</script>

<svelte:window onclick={closeOnOutside} onkeydown={closeOnEscape} />

<span class="param-help" bind:this={rootEl}>
	<button
		type="button"
		class="param-help-trigger"
		aria-label={ariaLabel ?? text}
		aria-expanded={open}
		bind:this={triggerEl}
		onclick={toggle}
	>
		<svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true">
			<circle cx="8" cy="8" r="7" fill="none" stroke="currentColor" stroke-width="1.4" />
			<path
				d="M6.15 6.05c0-1.05 0.85-1.95 1.95-1.95 1.05 0 1.85 0.65 1.85 1.75 0 0.85-0.45 1.35-1.25 1.85-0.85 0.55-1.05 0.95-1.05 1.75v0.25"
				fill="none"
				stroke="currentColor"
				stroke-width="1.25"
				stroke-linecap="round"
			/>
			<circle cx="8" cy="12.15" r="0.8" fill="currentColor" />
		</svg>
	</button>
</span>

{#if open}
	<div
		class="param-help-popup"
		role="tooltip"
		bind:this={popupEl}
		use:portal
		style:top="{popupTop}px"
		style:left="{popupLeft}px"
	>
		{text}
	</div>
{/if}

<style>
	.param-help {
		display: inline-flex;
		align-items: center;
		align-self: center;
		flex-shrink: 0;
		vertical-align: -0.12em;
		margin-left: 0.2rem;
	}

	.param-help-trigger {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 1.05rem;
		height: 1.05rem;
		padding: 0;
		border: none;
		border-radius: 50%;
		background: transparent;
		color: var(--color-text-muted);
		cursor: pointer;
		transition: color 0.12s ease;
	}

	.param-help-trigger:hover,
	.param-help-trigger[aria-expanded='true'] {
		color: var(--color-accent);
	}

	.param-help-trigger:focus-visible {
		outline: 2px solid var(--color-accent);
		outline-offset: 2px;
		border-radius: 50%;
	}

	:global(.param-help-popup) {
		position: fixed;
		z-index: var(--z-floating);
		width: max-content;
		max-width: min(18rem, calc(100vw - 1.5rem));
		padding: 0.55rem 0.7rem;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-sm);
		background: var(--color-bg-secondary);
		box-shadow: 0 8px 24px rgba(0, 0, 0, 0.25);
		font-size: 0.75rem;
		line-height: 1.4;
		font-weight: 400;
		color: var(--color-text-primary);
		white-space: normal;
		text-align: left;
		cursor: default;
	}
</style>
