<script lang="ts">
	export type PreflightStatus = 'ok' | 'no-policy' | 'no-policy-in-ndms' | 'no-devices';

	interface Props {
		status: PreflightStatus;
		policyName: string | null;
		onResetPolicyName?: () => void;
	}
	let { status, policyName, onResetPolicyName }: Props = $props();

	const POLICIES_HREF = '/routing?tab=policy';
</script>

{#if status === 'no-policy'}
	<div class="banner warning">
		<div class="text">
			Не выбрана политика доступа. Трафик устройств не попадёт в sing-box.
		</div>
		<div class="actions">
			<a class="link" href={POLICIES_HREF}>Открыть политики доступа</a>
		</div>
	</div>
{:else if status === 'no-policy-in-ndms'}
	<div class="banner danger">
		<div class="text">
			Политика "{policyName}" указана в настройках, но отсутствует в роутере.
			Она была удалена снаружи.
		</div>
		<div class="actions">
			{#if onResetPolicyName}
				<button type="button" class="btn-reset" onclick={onResetPolicyName}>Сбросить</button>
			{/if}
			<a class="link" href={POLICIES_HREF}>Открыть политики доступа</a>
		</div>
	</div>
{:else if status === 'no-devices'}
	<div class="banner warning">
		<div class="text">
			Политика "{policyName}" пуста — привяжите устройства.
		</div>
		<div class="actions">
			<a class="link" href={POLICIES_HREF}>Открыть политики доступа</a>
		</div>
	</div>
{/if}

<style>
	.banner {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		padding: 0.6rem 0.85rem;
		margin-bottom: 0.75rem;
		border-radius: 4px;
		font-size: 0.85rem;
		line-height: 1.4;
	}
	.warning {
		background: rgba(245, 197, 24, 0.12);
		border-left: 3px solid var(--warning, #f5c518);
		color: var(--color-text-primary);
	}
	.danger {
		background: rgba(248, 81, 73, 0.10);
		border-left: 3px solid var(--danger, #f85149);
		color: var(--color-text-primary);
	}
	.text {
		flex: 1;
	}
	.actions {
		display: flex;
		gap: 0.5rem;
		align-items: center;
	}
	.link {
		color: var(--color-accent, #6cb6ff);
		text-decoration: none;
		font-size: 0.82rem;
	}
	.link:hover {
		text-decoration: underline;
	}
	.btn-reset {
		background: transparent;
		color: var(--danger, #f85149);
		border: 1px solid currentColor;
		padding: 0.2rem 0.55rem;
		border-radius: 4px;
		font-size: 0.78rem;
		cursor: pointer;
	}
	.btn-reset:hover {
		background: rgba(248, 81, 73, 0.08);
	}
</style>
