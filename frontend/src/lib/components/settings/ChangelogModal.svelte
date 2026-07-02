<script lang="ts">
	import { api } from '$lib/api/client';
	import { SideDrawer, Button } from '$lib/components/ui';
	import { LoadingSpinner } from '$lib/components/layout';
	import ChangelogRender from './ChangelogRender.svelte';
	import type { ChangelogEntry } from '$lib/types';

	interface Props {
		open: boolean;
		fromVersion: string;
		toVersion: string;
		/** true — диапазон до pending-релиза; false — уже установленная ветка (minor line). */
		pendingUpdate?: boolean;
		oncheckUpdates?: () => void;
		onclose: () => void;
	}

	let {
		open,
		fromVersion,
		toVersion,
		pendingUpdate = false,
		oncheckUpdates,
		onclose
	}: Props = $props();

	let loading = $state(false);
	let error = $state('');
	let entries = $state<ChangelogEntry[]>([]);

	function changelogErrorToText(error: unknown): string {
		const raw = error instanceof Error ? error.message : String(error ?? '');

		if (
			raw.includes('502') ||
			raw.includes('Bad Gateway') ||
			raw.includes('CHANGELOG_FETCH_FAILED') ||
			raw.includes('Unexpected token') ||
			raw.includes('<!DOCTYPE') ||
			raw.includes('status 502')
		) {
			return 'Список изменений временно недоступен. Проверьте подключение к репозиторию обновлений или повторите попытку позже.';
		}

		if (raw.includes('changelog not published yet')) {
			return 'Для этой версии список изменений ещё не опубликован.';
		}

		return 'Не удалось загрузить список изменений. Повторите попытку позже.';
	}

	$effect(() => {
		if (!open) return;
		loading = true;
		error = '';
		entries = [];
		api.getUpdateChangelog(fromVersion, toVersion)
			.then((resp) => {
				entries = resp.entries ?? [];
			})
			.catch((e: unknown) => {
				error = changelogErrorToText(e);
			})
			.finally(() => {
				loading = false;
			});
	});
</script>

<SideDrawer {open} title="Что нового" width={640} bodyClass="drawer-body-fill" onClose={onclose}>
	<div class="modal-body">
		{#if loading}
			<LoadingSpinner />
		{:else if error}
			<p class="state-msg state-error">{error}</p>
		{:else if entries.length === 0}
			<p class="state-msg">В CHANGELOG нет записей для этой ветки версий.</p>
		{:else}
			<ChangelogRender {entries} />
		{/if}
	</div>
	{#snippet footer()}
		<Button variant="primary" size="md" onclick={onclose}>Закрыть</Button>
	{/snippet}
</SideDrawer>

<style>
	.modal-body {
		flex: 1;
		min-height: 0;
		height: 100%;
		padding: 1rem;
		overflow-y: auto;
		overflow-x: hidden;
	}
	.state-msg {
		margin: 0;
		padding: 12px 0;
		color: var(--text-muted);
	}
	.state-error {
		color: var(--error);
	}
</style>
