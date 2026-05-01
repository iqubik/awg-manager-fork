<script lang="ts">
	import { Modal, Button } from '$lib/components/ui';
	import { api } from '$lib/api/client';
	import { notifications } from '$lib/stores/notifications';
	import type {
		SingboxRouterInspectResult,
		SingboxRouterInspectMatch,
	} from '$lib/types';

	interface Props {
		open: boolean;
		onClose: () => void;
	}

	let { open, onClose }: Props = $props();

	let inputValue = $state('');
	let port = $state<number | ''>('');
	let protocol = $state<'' | 'tcp' | 'udp'>('');
	let advancedOpen = $state(false);
	let testing = $state(false);
	let result = $state<SingboxRouterInspectResult | null>(null);
	let error = $state('');
	let showAllRules = $state(false);

	const examples = [
		'google.com',
		'youtube.com',
		'instagram.com',
		'8.8.8.8',
		'192.168.1.1',
	];

	async function testRoute(): Promise<void> {
		const trimmed = inputValue.trim();
		if (!trimmed) return;

		testing = true;
		error = '';
		result = null;
		showAllRules = false;

		try {
			result = await api.singboxRouterInspectRoute({
				domain: trimmed,
				port: typeof port === 'number' && port > 0 ? port : undefined,
				protocol: protocol || undefined,
			});
		} catch (e) {
			const msg = e instanceof Error ? e.message : String(e);
			error = msg;
			notifications.error(`Не удалось проверить маршрут: ${msg}`);
		} finally {
			testing = false;
		}
	}

	function quickTest(value: string): void {
		inputValue = value;
		testRoute();
	}

	function handleKeydown(e: KeyboardEvent): void {
		if (e.key === 'Enter' && !testing) {
			testRoute();
		}
	}

	function reset(): void {
		inputValue = '';
		port = '';
		protocol = '';
		result = null;
		error = '';
		showAllRules = false;
		advancedOpen = false;
	}

	function close(): void {
		reset();
		onClose();
	}

	function actionVariant(action: string): 'route' | 'reject' | 'sniff' | 'other' {
		if (action === 'route') return 'route';
		if (action === 'reject') return 'reject';
		if (action === 'sniff' || action === 'hijack-dns') return 'sniff';
		return 'other';
	}

	function actionLabel(action: string): string {
		if (action === 'route') return 'ROUTE';
		if (action === 'reject') return 'REJECT';
		if (action === 'sniff') return 'SNIFF';
		if (action === 'hijack-dns') return 'HIJACK';
		return action.toUpperCase();
	}

	const matchedRuleData = $derived.by<SingboxRouterInspectMatch | null>(() => {
		const r = result;
		if (!r || r.matchedRule < 0) return null;
		return r.matches.find((m) => m.index === r.matchedRule) ?? null;
	});

	const isReject = $derived(result?.destination === 'REJECT');
</script>

<Modal {open} title="Инспектор маршрутов" size="xl" onclose={close}>
	<div class="inspector">
		<!-- Input section -->
		<section class="card input-section">
			<label for="inspector-input" class="field-label">
				Домен или IP
			</label>
			<div class="input-row">
				<input
					id="inspector-input"
					type="text"
					bind:value={inputValue}
					onkeydown={handleKeydown}
					placeholder="например, google.com или 8.8.8.8"
					class="text-input"
					autocomplete="off"
				/>
				<Button
					variant="primary"
					onclick={testRoute}
					disabled={testing || !inputValue.trim()}
				>
					{testing ? 'Проверяем…' : 'Проверить'}
				</Button>
			</div>

			<button
				type="button"
				class="advanced-toggle"
				onclick={() => (advancedOpen = !advancedOpen)}
			>
				{advancedOpen ? 'Скрыть' : 'Показать'} дополнительные параметры
			</button>

			{#if advancedOpen}
				<div class="advanced-row">
					<label class="adv-field">
						<span class="adv-label">Порт</span>
						<input
							type="number"
							min="0"
							max="65535"
							bind:value={port}
							placeholder="опционально"
							class="text-input"
						/>
					</label>
					<label class="adv-field">
						<span class="adv-label">Протокол</span>
						<select bind:value={protocol} class="select-input">
							<option value="">не задан</option>
							<option value="tcp">tcp</option>
							<option value="udp">udp</option>
						</select>
					</label>
				</div>
			{/if}

			<div class="quick-row">
				<span class="quick-label">Быстрая проверка:</span>
				{#each examples as ex (ex)}
					<button
						type="button"
						class="chip"
						onclick={() => quickTest(ex)}
						disabled={testing}
					>
						{ex}
					</button>
				{/each}
			</div>
		</section>

		{#if error}
			<div class="error-banner">{error}</div>
		{/if}

		{#if result}
			<!-- Big result card -->
			<section class="card result-card">
				<div class="result-row">
					<div class="input-block">
						<div class="input-value">{result.input}</div>
						<div class="input-type">{result.inputType === 'domain' ? 'домен' : 'IP-адрес'}</div>
					</div>
					<div class="arrow">→</div>
					<div
						class="dest-block"
						class:dest-reject={isReject}
						class:dest-final={result.matchedRule < 0 && !isReject}
					>
						<div class="dest-value">{result.destination}</div>
						<div class="dest-meta">
							{#if result.matchedRule >= 0}
								Сработало правило #{result.matchedRule + 1}
							{:else}
								Дефолтный outbound (final: {result.final || 'direct'})
							{/if}
						</div>
					</div>
				</div>

				{#if matchedRuleData}
					<div class="match-detail">
						<div class="match-header">
							<span class="rule-num">Правило #{matchedRuleData.index + 1}</span>
							<span class="badge badge-{actionVariant(matchedRuleData.action)}">
								{actionLabel(matchedRuleData.action)}
							</span>
							{#if matchedRuleData.outbound}
								<span class="match-outbound">→ {matchedRuleData.outbound}</span>
							{/if}
						</div>
						{#if matchedRuleData.reason}
							<div class="match-reason">{matchedRuleData.reason}</div>
						{/if}
						{#if matchedRuleData.conditions && matchedRuleData.conditions.length}
							<div class="match-conditions">
								<span class="cond-label">Условия:</span>
								{matchedRuleData.conditions.join(', ')}
							</div>
						{/if}
					</div>
				{:else}
					<div class="match-detail no-match">
						Ни одно правило не сработало — трафик пойдёт через
						<strong>{result.final || 'direct'}</strong>.
					</div>
				{/if}
			</section>

			{#if result.note}
				<div class="note-banner">
					<strong>Примечание:</strong>
					{result.note}
				</div>
			{/if}

			{#if result.matches.length > 0}
				<button
					type="button"
					class="walkthrough-toggle"
					onclick={() => (showAllRules = !showAllRules)}
				>
					{showAllRules ? 'Скрыть' : 'Показать'} разбор всех правил ({result.matches.length})
				</button>
			{/if}

			{#if showAllRules}
				<section class="card walkthrough">
					<header class="walkthrough-header">Порядок проверки правил</header>
					<ul class="walkthrough-list">
						{#each result.matches as m (m.index)}
							<li
								class="walkthrough-row"
								class:row-matched={m.matched}
								class:row-non-final={m.matched &&
									(m.action === 'sniff' || m.action === 'hijack-dns')}
							>
								<div class="row-head">
									<span class="row-index">#{m.index + 1}</span>
									<span class="badge badge-{actionVariant(m.action)}">
										{actionLabel(m.action)}
									</span>
									{#if m.outbound}
										<span class="row-outbound">→ {m.outbound}</span>
									{/if}
									<span class="row-status">
										{#if m.matched}
											{#if m.action === 'sniff' || m.action === 'hijack-dns'}
												совпало (не финальное)
											{:else}
												совпало
											{/if}
										{:else}
											не совпало
										{/if}
									</span>
								</div>
								{#if m.conditions && m.conditions.length}
									<div class="row-conditions">{m.conditions.join(' · ')}</div>
								{/if}
								{#if m.reason}
									<div class="row-reason">{m.reason}</div>
								{/if}
							</li>
						{/each}
						<li class="walkthrough-row row-final">
							<div class="row-head">
								<span class="row-index">∞</span>
								<span class="badge badge-other">FINAL</span>
								<span class="row-outbound">→ {result.final || 'direct'}</span>
								<span class="row-status">используется, если ни одно правило не подходит</span>
							</div>
						</li>
					</ul>
				</section>
			{/if}
		{:else if !error}
			<div class="empty-state">
				Введите домен или IP-адрес — инспектор покажет, через какой outbound пойдёт
				трафик и какое правило сработает. Это симуляция, sing-box не вызывается.
			</div>
		{/if}
	</div>
</Modal>

<style>
	.inspector {
		display: flex;
		flex-direction: column;
		gap: 0.875rem;
	}

	.card {
		background: var(--color-bg-tertiary);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-sm);
		padding: 0.875rem 1rem;
	}

	.field-label {
		display: block;
		font-size: 12px;
		color: var(--color-text-secondary);
		margin-bottom: 0.4rem;
	}

	.input-row {
		display: flex;
		gap: 0.5rem;
		align-items: stretch;
	}

	.text-input,
	.select-input {
		flex: 1;
		min-width: 0;
		padding: 0.5rem 0.75rem;
		background: var(--color-bg-primary);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-sm);
		color: var(--color-text-primary);
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		font-size: 13px;
		box-sizing: border-box;
	}

	.text-input:focus,
	.select-input:focus {
		outline: none;
		border-color: var(--color-accent);
		box-shadow: 0 0 0 2px var(--color-accent-tint);
	}

	.advanced-toggle {
		margin-top: 0.6rem;
		padding: 0;
		background: none;
		border: none;
		color: var(--color-text-muted);
		font-size: 12px;
		cursor: pointer;
		text-align: left;
	}

	.advanced-toggle:hover {
		color: var(--color-text-secondary);
	}

	.advanced-row {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.5rem;
		margin-top: 0.5rem;
	}

	.adv-field {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.adv-label {
		font-size: 11px;
		color: var(--color-text-muted);
	}

	.quick-row {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.4rem;
		margin-top: 0.75rem;
	}

	.quick-label {
		font-size: 11px;
		color: var(--color-text-muted);
	}

	.chip {
		padding: 0.25rem 0.55rem;
		font-size: 12px;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		background: var(--color-bg-primary);
		color: var(--color-text-secondary);
		border: 1px solid var(--color-border);
		border-radius: 999px;
		cursor: pointer;
	}

	.chip:hover:not(:disabled) {
		background: var(--color-bg-hover);
		color: var(--color-text-primary);
	}

	.chip:disabled {
		opacity: 0.5;
		cursor: not-allowed;
	}

	.error-banner {
		padding: 0.6rem 0.75rem;
		background: var(--color-error-tint);
		border: 1px solid var(--color-error-border);
		border-radius: var(--radius-sm);
		color: var(--color-error);
		font-size: 13px;
	}

	.note-banner {
		padding: 0.6rem 0.75rem;
		background: var(--color-warning-tint);
		border: 1px solid var(--color-warning-border);
		border-radius: var(--radius-sm);
		color: var(--color-text-primary);
		font-size: 12px;
		line-height: 1.5;
	}

	.note-banner strong {
		color: var(--color-warning);
	}

	.result-card {
		display: flex;
		flex-direction: column;
		gap: 0.875rem;
	}

	.result-row {
		display: grid;
		grid-template-columns: 1fr auto 1fr;
		align-items: center;
		gap: 0.75rem;
	}

	.input-block,
	.dest-block {
		display: flex;
		flex-direction: column;
		gap: 0.2rem;
		min-width: 0;
	}

	.dest-block {
		text-align: right;
	}

	.input-value,
	.dest-value {
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		font-size: 18px;
		color: var(--color-text-primary);
		word-break: break-all;
	}

	.dest-value {
		color: var(--color-success);
		font-weight: 600;
	}

	.dest-block.dest-reject .dest-value {
		color: var(--color-error);
	}

	.dest-block.dest-final .dest-value {
		color: var(--color-text-secondary);
	}

	.input-type,
	.dest-meta {
		font-size: 11px;
		color: var(--color-text-muted);
	}

	.arrow {
		font-size: 22px;
		color: var(--color-text-muted);
	}

	.match-detail {
		padding-top: 0.75rem;
		border-top: 1px solid var(--color-border);
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
	}

	.match-detail.no-match {
		color: var(--color-text-secondary);
		font-size: 13px;
	}

	.match-header {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.5rem;
	}

	.rule-num {
		font-weight: 600;
		color: var(--color-text-primary);
	}

	.match-outbound,
	.row-outbound {
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		font-size: 12px;
		color: var(--color-text-secondary);
	}

	.match-reason {
		font-size: 12px;
		color: var(--color-success);
	}

	.match-conditions {
		font-size: 11px;
		color: var(--color-text-muted);
	}

	.cond-label {
		color: var(--color-text-muted);
		margin-right: 0.25rem;
	}

	.badge {
		display: inline-block;
		padding: 0.1rem 0.45rem;
		font-size: 10px;
		font-weight: 600;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		border-radius: 4px;
		border: 1px solid transparent;
	}

	.badge-route {
		background: var(--color-success-tint);
		color: var(--color-success);
		border-color: var(--color-success-border);
	}

	.badge-reject {
		background: var(--color-error-tint);
		color: var(--color-error);
		border-color: var(--color-error-border);
	}

	.badge-sniff {
		background: var(--color-info-tint);
		color: var(--color-info);
		border-color: var(--color-info-border);
	}

	.badge-other {
		background: var(--color-muted-tint);
		color: var(--color-text-secondary);
		border-color: var(--color-border);
	}

	.walkthrough-toggle {
		align-self: center;
		padding: 0.4rem 0.75rem;
		background: none;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-sm);
		color: var(--color-text-secondary);
		font-size: 12px;
		cursor: pointer;
	}

	.walkthrough-toggle:hover {
		background: var(--color-bg-hover);
		color: var(--color-text-primary);
	}

	.walkthrough {
		padding: 0;
		overflow: hidden;
	}

	.walkthrough-header {
		padding: 0.6rem 0.875rem;
		background: var(--color-bg-secondary);
		border-bottom: 1px solid var(--color-border);
		font-size: 12px;
		color: var(--color-text-secondary);
		font-weight: 600;
	}

	.walkthrough-list {
		list-style: none;
		margin: 0;
		padding: 0;
		max-height: 360px;
		overflow-y: auto;
	}

	.walkthrough-row {
		padding: 0.55rem 0.875rem;
		border-bottom: 1px solid var(--color-border);
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
	}

	.walkthrough-row:last-child {
		border-bottom: none;
	}

	.walkthrough-row.row-matched {
		background: color-mix(in srgb, var(--color-success) 6%, transparent);
	}

	.walkthrough-row.row-non-final {
		background: color-mix(in srgb, var(--color-info) 6%, transparent);
	}

	.walkthrough-row.row-final {
		background: var(--color-bg-secondary);
	}

	.row-head {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.5rem;
		font-size: 12px;
	}

	.row-index {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 1.5rem;
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		color: var(--color-text-muted);
	}

	.row-status {
		margin-left: auto;
		font-size: 11px;
		color: var(--color-text-muted);
	}

	.row-matched .row-status {
		color: var(--color-success);
	}

	.row-non-final .row-status {
		color: var(--color-info);
	}

	.row-conditions {
		font-size: 11px;
		color: var(--color-text-muted);
		font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
		padding-left: 2rem;
	}

	.row-reason {
		font-size: 11px;
		color: var(--color-text-secondary);
		padding-left: 2rem;
	}

	.empty-state {
		padding: 1rem;
		text-align: center;
		color: var(--color-text-secondary);
		font-size: 13px;
		line-height: 1.5;
		border: 1px dashed var(--color-border);
		border-radius: var(--radius-sm);
	}
</style>
