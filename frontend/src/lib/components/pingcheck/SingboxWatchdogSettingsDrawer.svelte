<script lang="ts">
	import { api } from '$lib/api/client';
	import { notifications } from '$lib/stores/notifications';
	import type {
		SingboxWatchdogConfig,
		SingboxWatchdogRecoveryMode,
		SingboxWatchdogStatus,
	} from '$lib/types';
	import { Button, SideDrawer } from '$lib/components/ui';

	interface Props {
		open: boolean;
		target: SingboxWatchdogStatus | null;
		onclose: () => void;
		onSaved: () => void;
	}

	let { open, target, onclose, onSaved }: Props = $props();

	let interval = $state(30);
	let failThreshold = $state(3);
	let timeout = $state(5);
	let recoveryMode = $state<SingboxWatchdogRecoveryMode>('off');
	let persistSwitch = $state(false);
	let confirmDangerousRestart = $state(false);
	let saving = $state(false);

	$effect(() => {
		if (!target) return;
		interval = Math.min(3600, Math.max(5, target.interval || 30));
		failThreshold = Math.min(20, Math.max(1, target.failThreshold || 3));
		timeout = Math.min(30, Math.max(1, target.timeout || 5));
		recoveryMode =
			target.recoveryMode ||
			(target.kind === 'subscription' ? 'switch-member' : 'off');
		persistSwitch = target.kind === 'subscription' && target.persistSwitch === true;
		confirmDangerousRestart = false;
	});

	const title = $derived(target ? `Watchdog: ${target.name}` : 'Настройки watchdog');
	const restartIsDangerous = $derived(target?.kind === 'tunnel' && recoveryMode === 'restart-singbox');
	const canSave = $derived(!saving && (!restartIsDangerous || confirmDangerousRestart));

	async function save(): Promise<void> {
		if (!target) return;
		const payload: SingboxWatchdogConfig = {
			id: target.id,
			kind: target.kind,
			ref: target.ref,
			enabled: true,
			interval: Math.min(3600, Math.max(5, Number(interval) || 30)),
			failThreshold: Math.min(20, Math.max(1, Number(failThreshold) || 3)),
			timeout: Math.min(30, Math.max(1, Number(timeout) || 5)),
			recoveryMode,
			persistSwitch: target.kind === 'subscription' ? persistSwitch : false,
		};
		saving = true;
		try {
			await api.singboxWatchdogConfigure(payload);
			notifications.success('Watchdog сохранён');
			onSaved();
		} catch (error) {
			notifications.error(error instanceof Error ? error.message : 'Не удалось сохранить watchdog');
		} finally {
			saving = false;
		}
	}
</script>

<SideDrawer {open} title={title} width={460} onClose={onclose}>
	<div class="drawer-body">
		<div class="field">
			<label for="wd-interval">Интервал проверки, секунд</label>
			<input id="wd-interval" type="number" min="5" max="3600" bind:value={interval} />
		</div>

		<div class="field">
			<label for="wd-threshold">Порог сбоев</label>
			<input id="wd-threshold" type="number" min="1" max="20" bind:value={failThreshold} />
		</div>

		<div class="field">
			<label for="wd-timeout">Таймаут, секунд</label>
			<input id="wd-timeout" type="number" min="1" max="30" bind:value={timeout} />
		</div>

		<div class="field">
			<label for="wd-recovery">Режим восстановления</label>
			<select id="wd-recovery" bind:value={recoveryMode}>
				<option value="off">Выключено</option>
				{#if target?.kind === 'subscription'}
					<option value="switch-member">Сменить участника</option>
				{:else}
					<option value="restart-singbox">Перезапуск всего sing-box</option>
				{/if}
			</select>
		</div>

		{#if target?.kind === 'tunnel'}
			<div class="hint {restartIsDangerous ? 'hint--danger' : ''}">
				{#if restartIsDangerous}
					Перезапускает весь sing-box, может кратковременно оборвать все Sing-box туннели и подписки.
				{:else}
					Для raw Sing-box tunnel безопасный режим по умолчанию: восстановление выключено.
				{/if}
			</div>
		{/if}

		{#if restartIsDangerous}
			<label class="check check--danger">
				<input type="checkbox" bind:checked={confirmDangerousRestart} />
				<span>Я понимаю, что это перезапускает весь sing-box и может оборвать все текущие Sing-box соединения.</span>
			</label>
		{/if}

		{#if target?.kind === 'subscription'}
			<label class="check">
				<input type="checkbox" bind:checked={persistSwitch} />
				<span>Сохранять выбранного участника в конфиге подписки после успешного переключения.</span>
			</label>
			<div class="hint">Если опция включена, удачный recovery закрепит нового участника и после перезапуска sing-box.</div>
		{/if}
	</div>

	<div class="drawer-actions">
		<Button variant="secondary" onclick={onclose} disabled={saving}>Отмена</Button>
		<Button variant="primary" onclick={() => void save()} loading={saving} disabled={!canSave}>Сохранить</Button>
	</div>
</SideDrawer>

<style>
	.drawer-body {
		display: flex;
		flex-direction: column;
		gap: 14px;
		padding: 4px 2px 16px;
	}

	.field {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}

	.field label,
	.check {
		color: var(--color-text-primary);
		font-size: 14px;
	}

	.field input,
	.field select {
		width: 100%;
		padding: 10px 12px;
		border-radius: 10px;
		border: 1px solid var(--color-border);
		background: var(--color-bg-secondary);
		color: var(--color-text-primary);
	}

	.check {
		display: flex;
		align-items: center;
		gap: 8px;
		color: var(--color-text-muted);
	}

	.check--danger {
		align-items: flex-start;
		color: var(--color-error);
	}

	.hint {
		font-size: 13px;
		color: var(--color-text-muted);
	}

	.hint--danger {
		color: var(--color-warning);
	}

	.drawer-actions {
		display: flex;
		justify-content: flex-end;
		gap: 8px;
		padding-top: 12px;
		border-top: 1px solid var(--color-border);
	}

	@media (max-width: 768px) {
		.drawer-actions {
			align-items: stretch;
		}

		.drawer-actions :global(.btn) {
			flex: 1 1 0;
			width: 100%;
			min-width: 0;
			justify-content: center;
		}
	}
</style>
