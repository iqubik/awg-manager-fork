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
>
	<div class="drawer-card">
		<div class="drawer-card-body">
			<div class="form">
				<label class="field">
					<div class="lbl">Шаблон домена</div>
					<input class="mono" bind:value={pattern} placeholder="nas.lan · *.discord.media · finland10*.discord.media" />
					<div class="hint">
						Без <code>*</code> — точный домен. <code>*.suffix</code> — все поддомены.
						<code>prefix*.suffix</code> — wildcard внутри первой метки (нужен доменный хвост после <code>*</code>).
					</div>
				</label>
				<label class="field">
					<div class="lbl">IP-адреса (через запятую)</div>
					<input class="mono" bind:value={ipsStr} placeholder="104.25.158.178, fd00::5" />
				</label>
				{#if error}<div class="error">{error}</div>{/if}
			</div>
		</div>
		<footer class="drawer-card-footer">
			<Button variant="ghost" size="md" onclick={onClose} type="button">Отмена</Button>
			<Button variant="primary" size="md" onclick={save} disabled={busy} loading={busy} type="button">
				Сохранить
			</Button>
		</footer>
	</div>
</SideDrawer>

<style>
	.drawer-card {
		min-width: 0;
		border: 1px solid var(--border);
		border-radius: 12px;
		background:
			linear-gradient(180deg, rgba(255, 255, 255, 0.025), rgba(255, 255, 255, 0)),
			var(--bg-secondary, var(--color-bg-secondary));
		overflow: hidden;
	}
	.drawer-card-body {
		padding: 1rem;
		min-width: 0;
	}
	.drawer-card-footer {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
		padding: 0.875rem 1rem;
		border-top: 1px solid var(--border);
		background: var(--bg-secondary, var(--color-bg-secondary));
	}
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
	.lbl { font-size: 0.75rem; color: var(--muted-text); }
	.field input {
		background: var(--bg);
		border: 1px solid var(--border);
		padding: 0.4rem 0.6rem;
		border-radius: 4px;
		color: var(--text);
		font-size: 0.85rem;
		box-sizing: border-box;
		width: 100%;
		min-width: 0;
		min-height: 2.25rem;
	}
	.mono { font-family: ui-monospace, monospace; }
	.hint {
		font-size: 0.75rem;
		color: var(--muted-text);
		line-height: 1.4;
		overflow-wrap: anywhere;
	}
	.hint code { background: var(--bg); padding: 0.05rem 0.25rem; border-radius: 2px; font-family: ui-monospace, monospace; }
	.error { color: var(--danger, #dc2626); font-size: 0.85rem; }
	@media (max-width: 640px) {
		.drawer-card {
			border-radius: 12px;
		}
		.drawer-card-body {
			padding: 0.875rem;
		}
		.drawer-card-footer {
			display: grid;
			grid-template-columns: repeat(2, minmax(0, 1fr));
			gap: 0.5rem;
			padding: 0.75rem 0.875rem;
			align-items: stretch;
		}
		.drawer-card-footer :global(.btn) {
			width: 100%;
			min-width: 0;
		}
	}
</style>
