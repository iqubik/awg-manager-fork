<script lang="ts">
	import { api } from '$lib/api/client';
	import { notifications } from '$lib/stores/notifications';
	import type {
		SingboxWatchdogConfig,
		SingboxWatchdogRecoveryMode,
		SingboxWatchdogStatus,
	} from '$lib/types';
	import { Button, SideDrawer } from '$lib/components/ui';
	import {
		buildSingboxWatchdogPayload,
		canSaveSingboxWatchdog,
		createSingboxWatchdogDrawerState,
		isDangerousSingboxRecovery,
	} from './singboxWatchdogLogic';

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
		const next = createSingboxWatchdogDrawerState(target);
		interval = next.interval;
		failThreshold = next.failThreshold;
		timeout = next.timeout;
		recoveryMode = next.recoveryMode;
		persistSwitch = next.persistSwitch;
		confirmDangerousRestart = next.confirmDangerousRestart;
	});

	const title = $derived(target ? `Watchdog: ${target.name}` : 'Настройки watchdog');
	const restartIsDangerous = $derived(isDangerousSingboxRecovery(target, recoveryMode));
	const canSave = $derived(canSaveSingboxWatchdog({ saving, restartIsDangerous, confirmDangerousRestart }));

	async function save(): Promise<void> {
		if (!target) return;
		const payload: SingboxWatchdogConfig = buildSingboxWatchdogPayload({
			target,
			interval,
			failThreshold,
			timeout,
			recoveryMode,
			persistSwitch,
		});
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
		<Button variant="ghost" onclick={onclose} disabled={saving}>Отмена</Button>
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
</style>
