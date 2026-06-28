<script lang="ts">
	import { tick, untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';

	interface Tab {
		id: string;
		label: string;
		badge?: number | string;
		badgeTone?: 'default' | 'success' | 'warning' | 'muted';
		separatorBefore?: boolean;
	}

	interface Props {
		tabs: Tab[];
		active: string;
		onchange: (id: string) => void;
		urlParam?: string;
		defaultTab?: string;
		ariaLabel?: string;
	}

	let {
		tabs,
		active,
		onchange,
		urlParam,
		defaultTab,
		ariaLabel = 'Sections',
	}: Props = $props();

	let containerEl: HTMLDivElement | undefined = $state();
	let urlConsumed = $state(false);
	let initialRevealDone = $state(false);

	function writeUrl(id: string) {
		if (!urlParam) return;
		const url = new URL($page.url);
		const fallback = defaultTab ?? tabs[0]?.id;
		if (id === fallback) {
			url.searchParams.delete(urlParam);
		} else {
			url.searchParams.set(urlParam, id);
		}
		const nextSearch = url.searchParams.toString();
		const currentSearch = $page.url.searchParams.toString();
		if (
			url.pathname === $page.url.pathname &&
			nextSearch === currentSearch &&
			url.hash === $page.url.hash
		) return;
		const target = url.pathname + (nextSearch ? `?${nextSearch}` : '') + url.hash;
		void goto(target, { replaceState: true, keepFocus: true, noScroll: true });
	}

	$effect(() => {
		if (!urlParam) {
			urlConsumed = true;
			return;
		}
		const fromUrl = $page.url.searchParams.get(urlParam);
		if (fromUrl == null) {
			urlConsumed = true;
			return;
		}
		if (fromUrl === untrack(() => active)) {
			urlConsumed = true;
			return;
		}
		if (!tabs.find((t) => t.id === fromUrl)) return;
		urlConsumed = true;
		onchange(fromUrl);
	});

	$effect(() => {
		if (!urlParam) return;
		if (!urlConsumed) return;
		writeUrl(active);
	});

	async function revealActiveTab(behavior: ScrollBehavior) {
		await tick();
		requestAnimationFrame(() => {
			if (!containerEl) return;
			const activeButton = containerEl.querySelector<HTMLButtonElement>(
				`[data-tab-id="${active}"]`,
			);
			if (!activeButton) return;
			const padding = 12;
			const containerRect = containerEl.getBoundingClientRect();
			const buttonRect = activeButton.getBoundingClientRect();
			const isClippedLeft = buttonRect.left < containerRect.left + padding;
			const isClippedRight = buttonRect.right > containerRect.right - padding;
			if (!isClippedLeft && !isClippedRight) return;
			const centeredLeft =
				activeButton.offsetLeft - (containerEl.clientWidth - activeButton.offsetWidth) / 2;
			containerEl.scrollTo({
				left: Math.max(0, centeredLeft),
				behavior,
			});
		});
	}

	$effect(() => {
		if (!containerEl) return;
		void tabs.length;
		void revealActiveTab(initialRevealDone ? 'smooth' : 'auto');
		initialRevealDone = true;
	});
</script>

<div
	class="mobile-tab-rail"
	role="tablist"
	aria-label={ariaLabel}
	bind:this={containerEl}
>
	{#each tabs as tab (tab.id)}
		<button
			type="button"
			role="tab"
			class="mobile-tab-rail__tab"
			class:is-active={tab.id === active}
			aria-selected={tab.id === active}
			data-tab-id={tab.id}
			onclick={() => onchange(tab.id)}
		>
			<span class="mobile-tab-rail__label">{tab.label}</span>
			{#if tab.badge !== undefined}
				<span
					class="mobile-tab-rail__badge"
					class:success={tab.badgeTone === 'success'}
					class:warning={tab.badgeTone === 'warning'}
					class:muted={tab.badgeTone === 'muted'}
				>
					{tab.badge}
				</span>
			{/if}
		</button>
	{/each}
</div>

<style>
	.mobile-tab-rail {
		display: flex;
		align-items: stretch;
		gap: 0.35rem;
		margin-bottom: 1rem;
		padding: 0 0 0.25rem;
		border-bottom: 1px solid var(--border);
		overflow-x: auto;
		overflow-y: hidden;
		scrollbar-width: none;
		-webkit-overflow-scrolling: touch;
		scroll-snap-type: x proximity;
		mask-image: linear-gradient(
			to right,
			transparent 0,
			#000 12px,
			#000 calc(100% - 12px),
			transparent 100%
		);
	}

	.mobile-tab-rail::-webkit-scrollbar {
		display: none;
	}

	.mobile-tab-rail__tab {
		display: inline-flex;
		align-items: center;
		gap: 0.45rem;
		flex: 0 0 auto;
		padding: 0.7rem 0.2rem 0.8rem;
		border: 0;
		border-bottom: 2px solid transparent;
		background: transparent;
		color: var(--text-muted);
		font-size: 0.875rem;
		font-weight: 600;
		white-space: nowrap;
		scroll-snap-align: start;
		transition: color 0.16s ease, border-color 0.16s ease;
	}

	.mobile-tab-rail__tab.is-active {
		color: var(--text-primary);
		border-bottom-color: var(--accent);
	}

	.mobile-tab-rail__label {
		display: inline-flex;
		align-items: center;
		min-width: 0;
	}

	.mobile-tab-rail__badge {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 1.2rem;
		height: 1.2rem;
		padding: 0 0.36rem;
		border-radius: 999px;
		background: var(--bg-hover);
		color: var(--text-muted);
		font-size: 0.68rem;
		font-weight: 700;
		line-height: 1;
	}

	.mobile-tab-rail__tab.is-active .mobile-tab-rail__badge {
		background: var(--accent);
		color: var(--color-accent-contrast, #fff);
	}

	.mobile-tab-rail__badge.success {
		background: rgba(158, 206, 106, 0.18);
		color: var(--success);
	}

	.mobile-tab-rail__badge.warning {
		background: rgba(224, 175, 104, 0.18);
		color: var(--warning);
	}

	.mobile-tab-rail__badge.muted {
		background: var(--bg-hover);
		color: var(--text-muted);
		opacity: 0.75;
	}

	.mobile-tab-rail__tab.is-active .mobile-tab-rail__badge.success,
	.mobile-tab-rail__tab.is-active .mobile-tab-rail__badge.warning {
		color: var(--color-accent-contrast, #fff);
	}

	.mobile-tab-rail__tab.is-active .mobile-tab-rail__badge.success {
		background: var(--success);
	}

	.mobile-tab-rail__tab.is-active .mobile-tab-rail__badge.warning {
		background: var(--warning);
	}
</style>
