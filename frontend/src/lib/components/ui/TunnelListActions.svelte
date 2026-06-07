<script lang="ts">
	import type { Snippet } from 'svelte';
	import TunnelTestIcon from '$lib/components/tunnels/TunnelTestIcon.svelte';

	interface Props {
		variant?: 'list' | 'labeled';
		editHref?: string;
		editLabel?: string;
		onEdit?: () => void;
		onTest?: () => void;
		onDelete?: () => void;
		testDisabled?: boolean;
		deleteDisabled?: boolean;
		deleting?: boolean;
		testTitle?: string;
		deleteTitle?: string;
		editTitle?: string;
		extra?: Snippet;
	}

	let {
		variant = 'list',
		editHref,
		editLabel = 'Изменить',
		onEdit,
		onTest,
		onDelete,
		testDisabled = false,
		deleteDisabled = false,
		deleting = false,
		testTitle = 'Тест',
		deleteTitle = 'Удалить',
		editTitle = 'Изменить',
		extra,
	}: Props = $props();

	const isLabeled = $derived(variant === 'labeled');
</script>

<div class="tunnel-list-actions" class:tunnel-list-actions--labeled={isLabeled}>
	{#if editHref}
		<a class="tunnel-list-actions__btn" href={editHref} title={editTitle} aria-label={editTitle}>
			<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
				<path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
				<path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
			</svg>
			{#if isLabeled}{editLabel}{/if}
		</a>
	{:else if onEdit}
		<button type="button" class="tunnel-list-actions__btn" title={editTitle} aria-label={editTitle} onclick={onEdit}>
			<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
				<path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
				<path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
			</svg>
			{#if isLabeled}{editLabel}{/if}
		</button>
	{/if}

	{#if onTest}
		<button
			type="button"
			class="tunnel-list-actions__btn tunnel-list-actions__btn--test"
			disabled={testDisabled}
			title={testTitle}
			aria-label={testTitle}
			onclick={onTest}
		>
			<TunnelTestIcon />
			{#if isLabeled}Тест{/if}
		</button>
	{/if}

	{#if extra}
		{@render extra()}
	{/if}

	{#if onDelete}
		<button
			type="button"
			class="tunnel-list-actions__btn tunnel-list-actions__btn--danger"
			disabled={deleteDisabled || deleting}
			title={deleteTitle}
			aria-label={deleteTitle}
			onclick={onDelete}
		>
			{#if deleting}
				<span class="tunnel-list-actions__spinner"></span>
			{:else}
				<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
					<polyline points="3,6 5,6 21,6"/>
					<path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
				</svg>
			{/if}
			{#if isLabeled}Удалить{/if}
		</button>
	{/if}
</div>
