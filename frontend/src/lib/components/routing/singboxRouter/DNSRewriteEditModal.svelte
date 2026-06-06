<script lang="ts">
	import { Button, SideDrawer } from '$lib/components/ui';
	import type { SingboxRouterDNSRewrite } from '$lib/types';

	interface Props {
		rewrite?: SingboxRouterDNSRewrite;
		onClose: () => void;
		onSave: (rewrite: SingboxRouterDNSRewrite) => Promise<void> | void;
	}
	let { rewrite, onClose, onSave }: Props = $props();

	// svelte-ignore state_referenced_locally
	let pattern = $state(rewrite?.pattern ?? '');
	// svelte-ignore state_referenced_locally
	let ipsStr = $state((rewrite?.ips ?? []).join(', '));
	let busy = $state(false);
	let error = $state('');

	async function save(): Promise<void> {
		busy = true;
		error = '';
		try {
			const p = pattern.trim();
			if (!p) { error = 'Шаблон обязателен'; busy = false; return; }
			const ips = ipsStr.split(',').map((s) => s.trim()).filter(Boolean);
			if (ips.length === 0) { error = 'Укажите хотя бы один IP'; busy = false; return; }
			await onSave({ pattern: p, ips });
		} catch (e) {
			error = (e as Error).message;
		} finally {
			busy = false;
		}
	}
</script>

<SideDrawer
	open
	onClose={onClose}
	title={rewrite ? 'Редактировать перезапись' : 'Новая перезапись'}
	width={520}
	footer={drawerFooter}
>
	<div class="form">
		<label class="field">
			<div class="lbl">Шаблон домена</div>
			<input class="mono" bind:value={pattern} placeholder="nas.lan · *.discord.media · finland10*.discord.media" />
			<div class="hint hint-block">
				Без <code>*</code> — точный домен. <code>*.suffix</code> — все поддомены. <code>prefix*.suffix</code> — wildcard внутри первой метки (нужен доменный хвост после <code>*</code>).
			</div>
		</label>
		<label class="field">
			<div class="lbl">IP-адреса (через запятую)</div>
			<input class="mono" bind:value={ipsStr} placeholder="104.25.158.178, fd00::5" />
		</label>
		{#if error}<div class="error">{error}</div>{/if}
	</div>
</SideDrawer>

{#snippet drawerFooter()}
	<Button variant="ghost" size="md" onclick={onClose} type="button">Отмена</Button>
	<Button variant="primary" size="md" onclick={save} disabled={busy} loading={busy} type="button">
		Сохранить
	</Button>
{/snippet}

<style>
	.form {
		display: grid;
		gap: 0.875rem;
		min-width: 0;
	}
	.field {
		display: grid;
		gap: 0.35rem;
		min-width: 0;
	}
	.lbl {
		font-size: 0.75rem;
		color: var(--muted-text);
	}
	.mono {
		font-family: ui-monospace, monospace;
	}
	.field input {
		background: var(--bg);
		border: 1px solid var(--border);
		padding: 0.4rem 0.6rem;
		border-radius: 4px;
		color: var(--text);
		font-size: 0.85rem;
		box-sizing: border-box;
		width: 100%;
	}
	.hint {
		font-size: 0.75rem;
		color: var(--text-muted);
		line-height: 1.35;
	}
	.hint-block {
		padding: 0.45rem 0.55rem;
		border: 1px solid color-mix(in srgb, var(--border) 75%, transparent);
		border-radius: 6px;
		background: color-mix(in srgb, var(--bg-secondary) 45%, transparent);
	}
	.hint-block code {
		color: var(--text-secondary);
		background: transparent;
		padding: 0;
	}
	.error {
		color: var(--danger, #dc2626);
		font-size: 0.85rem;
	}
</style>
