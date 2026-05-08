<script lang="ts">
	import { goto } from '$app/navigation';
	import { Modal, Button, Dropdown } from '$lib/components/ui';
	import { api } from '$lib/api/client';
	import HeadersTextarea from './HeadersTextarea.svelte';
	import { DEFAULT_PRESET, parseHeadersText } from './headersParser';

	interface Props {
		open: boolean;
	}
	let { open = $bindable(false) }: Props = $props();

	let label = $state('');
	let url = $state('');
	let headersText = $state(DEFAULT_PRESET);
	let refreshHoursStr = $state('24');
	let refreshHours = $state(24);
	let enabled = $state(true);
	let submitting = $state(false);
	let error = $state('');

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

	function reset(): void {
		label = '';
		url = '';
		headersText = DEFAULT_PRESET;
		refreshHoursStr = '24';
		refreshHours = 24;
		enabled = true;
		error = '';
	}

	function close(): void {
		if (submitting) return;
		open = false;
		reset();
	}

	async function submit(): Promise<void> {
		error = '';
		submitting = true;
		try {
			const sub = await api.createSubscription({
				label,
				url,
				headers: parseHeadersText(headersText),
				refreshHours,
				enabled,
			});
			open = false;
			reset();
			goto(`/subscriptions/${sub.id}`);
		} catch (e) {
			error = e instanceof Error ? e.message : 'Не удалось создать';
		} finally {
			submitting = false;
		}
	}
</script>

<Modal {open} title="Добавить подписку" size="lg" onclose={close}>
	<form
		class="form"
		onsubmit={(e) => {
			e.preventDefault();
			submit();
		}}
		id="sub-create-form"
	>
		<label class="row">
			<span class="lbl">Название</span>
			<input class="inp" type="text" bind:value={label} placeholder="Provider X" required />
		</label>
		<label class="row">
			<span class="lbl">URL подписки</span>
			<input
				class="inp"
				type="url"
				bind:value={url}
				placeholder="https://provider.example/sub/abc"
				required
			/>
		</label>
		<div class="row">
			<HeadersTextarea bind:value={headersText} />
		</div>
		<div class="row">
			<Dropdown
				label="Авто-обновление"
				bind:value={refreshHoursStr}
				options={refreshOptions}
				fullWidth
			/>
		</div>
		<label class="row chk">
			<input type="checkbox" bind:checked={enabled} />
			<span>Включить сразу</span>
		</label>
		{#if error}<div class="err">{error}</div>{/if}
	</form>

	{#snippet actions()}
		<Button variant="ghost" onclick={close} disabled={submitting}>Отмена</Button>
		<Button
			variant="primary"
			onclick={submit}
			disabled={submitting}
			loading={submitting}
		>
			{submitting ? 'Создаём...' : 'Создать'}
		</Button>
	{/snippet}
</Modal>

<style>
	.form { display: flex; flex-direction: column; gap: 1rem; }
	.row { display: flex; flex-direction: column; gap: 0.3rem; }
	.row.chk { flex-direction: row; align-items: center; gap: 0.5rem; }
	.lbl { font-size: 0.85rem; color: var(--color-text-muted); }
	.inp {
		padding: 0.5rem 0.7rem;
		background: var(--color-bg-primary);
		border: 1px solid var(--color-border);
		border-radius: 4px;
		color: var(--color-text-primary);
	}
	.err { color: #f85149; font-size: 0.85rem; }
</style>
