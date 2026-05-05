<script lang="ts">
	import type { Subscription } from '$lib/types';
	import { api } from '$lib/api/client';
	import { goto } from '$app/navigation';
	import HeadersTextarea from './HeadersTextarea.svelte';
	import { parseHeadersText, serializeHeaders } from './headersParser';
	import { Button, Dropdown } from '$lib/components/ui';
	import { untrack } from 'svelte';

	interface Props {
		subscription: Subscription;
		onUpdated: () => void;
	}
	let { subscription, onUpdated }: Props = $props();

	let label = $state(untrack(() => subscription.label));
	let url = $state(untrack(() => subscription.url));
	let headersText = $state(untrack(() => serializeHeaders(subscription.headers)));
	let refreshHoursStr = $state(untrack(() => String(subscription.refreshHours)));
	let refreshHours = $state(untrack(() => subscription.refreshHours));
	let enabled = $state(untrack(() => subscription.enabled));
	let saving = $state(false);
	let confirmDelete = $state(false);
	let deleting = $state(false);

	// Re-sync form state when subscription prop changes after parent reload.
	$effect(() => {
		label = subscription.label;
		url = subscription.url;
		headersText = serializeHeaders(subscription.headers);
		refreshHoursStr = String(subscription.refreshHours);
		refreshHours = subscription.refreshHours;
		enabled = subscription.enabled;
	});

	$effect(() => {
		refreshHours = parseInt(refreshHoursStr, 10) || 0;
	});

	const refreshOptions = [
		{ value: '0', label: 'Только вручную' },
		{ value: '1', label: 'Каждый час' },
		{ value: '6', label: 'Каждые 6 часов' },
		{ value: '12', label: 'Каждые 12 часов' },
		{ value: '24', label: 'Раз в сутки' },
		{ value: '168', label: 'Раз в неделю' },
	];

	async function save(): Promise<void> {
		saving = true;
		try {
			await api.updateSubscription(subscription.id, {
				label,
				url,
				headers: parseHeadersText(headersText),
				refreshHours,
				enabled,
			});
			onUpdated();
		} finally {
			saving = false;
		}
	}

	async function doDelete(): Promise<void> {
		deleting = true;
		try {
			await api.deleteSubscription(subscription.id);
			goto('/');
		} finally {
			deleting = false;
		}
	}
</script>

<form
	class="form"
	onsubmit={(e) => {
		e.preventDefault();
		save();
	}}
>
	<label><span>Название</span><input bind:value={label} /></label>
	<label><span>URL</span><input bind:value={url} /></label>
	<HeadersTextarea bind:value={headersText} />
	<Dropdown
		label="Авто-обновление"
		bind:value={refreshHoursStr}
		options={refreshOptions}
	/>
	<label class="chk"><input type="checkbox" bind:checked={enabled} /> Включена</label>
	<div class="actions">
		<Button type="submit" variant="primary" disabled={saving} loading={saving}>
			{saving ? 'Сохраняем...' : 'Сохранить'}
		</Button>
	</div>
</form>

<div class="danger-zone">
	{#if !confirmDelete}
		<Button variant="danger" onclick={() => (confirmDelete = true)}>Удалить подписку</Button>
	{:else}
		<div>Удалить подписку и все её ресурсы (sing-box outbound'ы, NDMS Proxy)?</div>
		<div class="confirm-actions">
			<Button variant="danger" disabled={deleting} loading={deleting} onclick={doDelete}>
				Удалить
			</Button>
			<Button variant="ghost" onclick={() => (confirmDelete = false)}>Отмена</Button>
		</div>
	{/if}
</div>

<style>
	.form { display: flex; flex-direction: column; gap: 0.7rem; max-width: 640px; }
	.form label { display: flex; flex-direction: column; gap: 0.3rem; }
	.form label.chk { flex-direction: row; align-items: center; gap: 0.5rem; }
	input {
		padding: 0.45rem 0.6rem;
		border: 1px solid var(--color-border);
		border-radius: 4px;
		background: var(--color-bg-primary);
		color: var(--color-text-primary);
	}
	.actions { display: flex; justify-content: flex-end; }
	.danger-zone {
		margin-top: 1.5rem;
		padding-top: 1rem;
		border-top: 1px solid var(--color-border);
	}
	.confirm-actions { display: flex; gap: 0.5rem; flex-wrap: wrap; margin-top: 0.5rem; }
</style>
