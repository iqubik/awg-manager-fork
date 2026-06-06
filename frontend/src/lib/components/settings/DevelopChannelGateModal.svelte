<script lang="ts">
	import { Modal, Button } from '$lib/components/ui';
	import { markDevelopChannelQuizPassed } from '$lib/utils/developChannelGate';

	interface Props {
		open: boolean;
		busy?: boolean;
		onclose: () => void;
		onpassed: () => void | Promise<void>;
	}

	let { open, busy = false, onclose, onpassed }: Props = $props();

	let accepted = $state(false);

	const rules = [
		'develop-ветка может содержать нестабильные сборки, визуальные дефекты и незавершённые функции',
		'после перехода на develop возможны ошибки, которые придётся описывать через нормальный баг-репорт',
		'перед обновлением желательно понимать, как откатиться на стабильную версию или переустановить AWG Manager',
		'возврат на стабильный канал может быть невозможен до следующего патча или без ручного восстановления',
		'develop-канал не даёт дополнительных привилегий и предназначен для осознанного тестирования',
	];

	$effect(() => {
		if (!open) {
			accepted = false;
		}
	});

	async function confirmAgreement() {
		if (!accepted || busy) return;
		markDevelopChannelQuizPassed();
		await onpassed();
	}
</script>

<Modal
	{open}
	title="Переход на develop-канал"
	size="md"
	onclose={onclose}
>
	<div class="gate-body">
		<p class="gate-lead">
			Ветка <b>develop</b> — это канал разработки. Здесь могут быть свежие, но нестабильные сборки.
			Квиз больше не требуется, но перед переключением нужно подтвердить согласие с правилами.
		</p>

		<div class="gate-warning">
			Переключайтесь на develop только если готовы самостоятельно проверить проблему, собрать логи,
			описать шаги воспроизведения и при необходимости откатиться.
		</div>

		<ul class="rules-list">
			{#each rules as rule}
				<li>{rule}</li>
			{/each}
		</ul>

		<label class="agreement-row">
			<input
				type="checkbox"
				bind:checked={accepted}
				disabled={busy}
			/>
			<span>
				Я понимаю риски develop-канала и согласен с правилами тестирования.
			</span>
		</label>
	</div>

	{#snippet actions()}
		<Button variant="secondary" size="md" onclick={onclose} disabled={busy}>
			Отмена
		</Button>
		<Button
			variant="primary"
			size="md"
			onclick={confirmAgreement}
			disabled={busy || !accepted}
			loading={busy}
		>
			Перейти на develop
		</Button>
	{/snippet}
</Modal>

<style>
	.gate-body {
		display: flex;
		flex-direction: column;
		gap: 1rem;
	}

	.gate-lead {
		margin: 0;
		line-height: 1.5;
		color: var(--text-primary);
	}

	.gate-warning {
		padding: 0.75rem 0.875rem;
		border: 1px solid color-mix(in srgb, var(--warning, #e0af68) 45%, var(--border));
		border-radius: 0.75rem;
		background: color-mix(in srgb, var(--warning, #e0af68) 10%, transparent);
		color: var(--text-primary);
		font-size: 0.875rem;
		line-height: 1.45;
	}

	.rules-list {
		margin: 0;
		padding-left: 1.25rem;
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		color: var(--text-secondary);
		font-size: 0.875rem;
		line-height: 1.45;
	}

	.agreement-row {
		display: flex;
		align-items: flex-start;
		gap: 0.625rem;
		padding: 0.75rem;
		border: 1px solid var(--border);
		border-radius: 0.75rem;
		background: var(--surface-elevated, var(--bg-secondary));
		color: var(--text-primary);
		font-size: 0.875rem;
		line-height: 1.45;
		cursor: pointer;
	}

	.agreement-row input {
		margin-top: 0.15rem;
		flex: 0 0 auto;
		accent-color: var(--accent);
	}

	.agreement-row:has(input:disabled) {
		opacity: 0.65;
		cursor: not-allowed;
	}
</style>
