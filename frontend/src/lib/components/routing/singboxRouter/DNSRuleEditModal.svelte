<script lang="ts">
	import { Button, Dropdown, ChipMultiSelect, SideDrawer, type DropdownOption, type ChipOption } from '$lib/components/ui';
	import type { SingboxRouterDNSRule, SingboxRouterDNSServer, SingboxRouterRuleSet } from '$lib/types';

	interface Props {
		rule?: SingboxRouterDNSRule;
		servers: SingboxRouterDNSServer[];
		availableRuleSets: SingboxRouterRuleSet[];
		/**
		 * Per-tag count of how many *other* DNS rules reference each rule_set.
		 * The currently edited rule must be excluded by the caller (use
		 * computeRuleSetUsage with excludeIndex=editIndex). Empty map is fine
		 * — all sets render as unused.
		 */
		ruleSetUsage?: Map<string, number>;
		onClose: () => void;
		onSave: (rule: SingboxRouterDNSRule) => Promise<void> | void;
	}
	let { rule, servers, availableRuleSets, ruleSetUsage, onClose, onSave }: Props = $props();

	function normalizeTags(tags: string[]): string[] {
		return [...new Set(tags.map((s) => s.trim()).filter(Boolean))];
	}

	const serverOptions = $derived<DropdownOption[]>([
		{ value: '', label: '— выберите —' },
		...servers.map((s) => ({
			value: s.tag,
			label: s.tag,
			description: `${s.type} · ${s.server}`,
		})),
	]);

	// svelte-ignore state_referenced_locally
	let ruleSetTags = $state<string[]>(rule?.rule_set ?? []);
	const ruleSetOptions = $derived<ChipOption[]>(
		availableRuleSets.map((rs) => ({
			value: rs.tag,
			label: rs.tag,
			usedCount: ruleSetUsage?.get(rs.tag) ?? 0,
		})),
	);
	// svelte-ignore state_referenced_locally
	let domainSuffixStr = $state((rule?.domain_suffix ?? []).join('\n'));
	// svelte-ignore state_referenced_locally
	let domainStr = $state((rule?.domain ?? []).join('\n'));
	// svelte-ignore state_referenced_locally
	let domainKeywordStr = $state((rule?.domain_keyword ?? []).join(', '));
	// svelte-ignore state_referenced_locally
	let domainRegexStr = $state((rule?.domain_regex ?? []).join('\n'));
	// svelte-ignore state_referenced_locally
	let queryTypeStr = $state((rule?.query_type ?? []).join(', '));

	function initAction(r?: SingboxRouterDNSRule): 'route' | 'block' {
		if (r?.action === 'reject' || r?.action === 'predefined') return 'block';
		return 'route';
	}
	function initBlockMethod(r?: SingboxRouterDNSRule): 'nxdomain' | 'refused' | 'drop' {
		if (r?.action === 'predefined') return 'nxdomain';
		if (r?.action === 'reject' && r?.method === 'drop') return 'drop';
		return 'refused';
	}
	// svelte-ignore state_referenced_locally
	let action = $state<'route' | 'block'>(initAction(rule));
	// svelte-ignore state_referenced_locally
	let blockMethod = $state<'nxdomain' | 'refused' | 'drop'>(initBlockMethod(rule));
	// svelte-ignore state_referenced_locally
	let server = $state(rule?.server ?? '');

	const blockMethodOptions = [
		{ value: 'nxdomain', label: 'NXDOMAIN (нет такого домена)' },
		{ value: 'refused', label: 'REFUSED' },
		{ value: 'drop', label: 'Drop (без ответа)' },
	];

	let busy = $state(false);
	let error = $state('');

	// Snapshot initial state for isDirty detection
	let initialRuleSetTagsSnapshot = $state<string[]>([]);
	let initialDomainSuffixStr = $state('');
	let initialDomainStr = $state('');
	let initialDomainKeywordStr = $state('');
	let initialDomainRegexStr = $state('');
	let initialQueryTypeStr = $state('');
	let initialAction: 'route' | 'block' = $state('route');
	let initialBlockMethod: 'nxdomain' | 'refused' | 'drop' = $state('refused');
	let initialServer = $state('');

	// Initialize snapshot when modal opens
	$effect(() => {
		if (rule) {
			initialRuleSetTagsSnapshot = [...(rule.rule_set ?? [])];
			initialDomainSuffixStr = (rule.domain_suffix ?? []).join('\n');
			initialDomainStr = (rule.domain ?? []).join('\n');
			initialDomainKeywordStr = (rule.domain_keyword ?? []).join(', ');
			initialDomainRegexStr = (rule.domain_regex ?? []).join('\n');
			initialQueryTypeStr = (rule.query_type ?? []).join(', ');
			initialAction = initAction(rule);
			initialBlockMethod = initBlockMethod(rule);
			initialServer = rule.server ?? '';
		} else {
			initialRuleSetTagsSnapshot = [];
			initialDomainSuffixStr = '';
			initialDomainStr = '';
			initialDomainKeywordStr = '';
			initialDomainRegexStr = '';
			initialQueryTypeStr = '';
			initialAction = 'route';
			initialBlockMethod = 'refused';
			initialServer = '';
		}
	});

	const isDirty = $derived.by(() => {
		return (
			normalizeTags(ruleSetTags).join(',') !== normalizeTags(initialRuleSetTagsSnapshot).join(',') ||
			domainSuffixStr !== initialDomainSuffixStr ||
			domainStr !== initialDomainStr ||
			domainKeywordStr !== initialDomainKeywordStr ||
			domainRegexStr !== initialDomainRegexStr ||
			queryTypeStr !== initialQueryTypeStr ||
			action !== initialAction ||
			blockMethod !== initialBlockMethod ||
			server !== initialServer
		);
	});

	async function save(): Promise<void> {
		busy = true;
		error = '';
		try {
			const rule_set = normalizeTags(ruleSetTags);
			const domain_suffix = domainSuffixStr.split('\n').map((s) => s.trim()).filter(Boolean);
			const domain = domainStr.split('\n').map((s) => s.trim()).filter(Boolean);
			const domain_keyword = domainKeywordStr.split(',').map((s) => s.trim()).filter(Boolean);
			const domain_regex = domainRegexStr.split('\n').map((s) => s.trim()).filter(Boolean);
			const query_type = queryTypeStr.split(',').map((s) => s.trim().toUpperCase()).filter(Boolean);

			const hasMatcher =
				rule_set.length > 0 ||
				domain_suffix.length > 0 ||
				domain.length > 0 ||
				domain_keyword.length > 0 ||
				domain_regex.length > 0 ||
				query_type.length > 0;
			if (!hasMatcher) {
				error = 'Нужен хотя бы один matcher';
				busy = false;
				return;
			}

			const built: SingboxRouterDNSRule = {
				rule_set: rule_set.length ? rule_set : undefined,
				domain_suffix: domain_suffix.length ? domain_suffix : undefined,
				domain: domain.length ? domain : undefined,
				domain_keyword: domain_keyword.length ? domain_keyword : undefined,
				domain_regex: domain_regex.length ? domain_regex : undefined,
				query_type: query_type.length ? query_type : undefined,
			};

			if (action === 'route') {
				if (!server) { error = 'Выберите DNS сервер'; busy = false; return; }
				built.action = 'route';
				built.server = server;
			} else if (blockMethod === 'nxdomain') {
				built.action = 'predefined';
				built.rcode = 'NXDOMAIN';
			} else if (blockMethod === 'drop') {
				built.action = 'reject';
				built.method = 'drop';
			} else {
				built.action = 'reject';
				built.method = 'default';
			}

			await onSave(built);
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
	title={rule ? 'Редактировать DNS правило' : 'Новое DNS правило'}
	width={620}
>
	<div class="drawer-card">
		<div class="drawer-card-body">
			<div class="form">
		<div class="section-label">Matchers (минимум один)</div>

		<label class="field">
			<div class="lbl">Rule sets</div>
			<ChipMultiSelect
				values={ruleSetTags}
				options={ruleSetOptions}
				onchange={(next) => (ruleSetTags = next)}
				placeholder="не выбрано"
				allowOrphans
			/>
		</label>

		<label class="field">
			<div class="lbl">Domain suffix</div>
			<textarea bind:value={domainSuffixStr} rows="3" placeholder="по одному на строке: .youtube.com"></textarea>
		</label>

		<label class="field">
			<div class="lbl">Domain (точное совпадение)</div>
			<textarea bind:value={domainStr} rows="2" placeholder="example.com"></textarea>
		</label>

		<label class="field">
			<div class="lbl">Domain keyword (через запятую)</div>
			<input bind:value={domainKeywordStr} placeholder="tracker, analytics" />
		</label>

		<label class="field">
			<div class="lbl">Domain regex (по строке)</div>
			<textarea bind:value={domainRegexStr} rows="2" placeholder={"^ads?\\d+\\."}></textarea>
		</label>

		<label class="field">
			<div class="lbl">Query type (через запятую)</div>
			<input bind:value={queryTypeStr} placeholder="A, AAAA, HTTPS" />
		</label>

		<div class="action-section">
			<div class="section-label">Действие</div>
			<div class="segment">
				<button class:active={action === 'route'} onclick={() => (action = 'route')} type="button">Резолвить</button>
				<button class:active={action === 'block'} onclick={() => (action = 'block')} type="button">Заблокировать</button>
			</div>

			{#if action === 'route'}
				<label class="field">
					<div class="lbl">DNS сервер</div>
					<Dropdown bind:value={server} options={serverOptions} fullWidth />
				</label>
			{:else}
				<label class="field">
					<div class="lbl">Метод блокировки</div>
					<Dropdown bind:value={blockMethod} options={blockMethodOptions} fullWidth />
				</label>
			{/if}
		</div>

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
		gap: 0.6rem;
		min-width: 0;
	}
	.section-label {
		font-size: 0.7rem;
		text-transform: uppercase;
		letter-spacing: 0.5px;
		color: var(--muted-text);
		margin-bottom: 0.25rem;
	}
	.field {
		display: grid;
		gap: 0.25rem;
	}
	.lbl {
		font-size: 0.75rem;
		color: var(--muted-text);
	}
	.field textarea,
	.field input {
		background: var(--bg);
		border: 1px solid var(--border);
		padding: 0.4rem 0.6rem;
		border-radius: 4px;
		color: var(--text);
		font-family: ui-monospace, monospace;
		font-size: 0.85rem;
		box-sizing: border-box;
		width: 100%;
		resize: vertical;
	}
	.action-section {
		border-top: 1px solid var(--border);
		padding-top: 0.75rem;
		margin-top: 0.25rem;
		display: grid;
		gap: 0.5rem;
	}
	.segment {
		display: inline-flex;
		border: 1px solid var(--border);
		border-radius: 4px;
		overflow: hidden;
		width: fit-content;
	}
	.segment button {
		background: transparent;
		border: none;
		padding: 0.4rem 0.9rem;
		font-size: 0.85rem;
		cursor: pointer;
		color: var(--muted-text);
	}
	.segment button + button {
		border-left: 1px solid var(--border);
	}
	.segment button.active {
		background: var(--accent, #3b82f6);
		color: var(--color-accent-contrast, #ffffff);
		font-weight: 600;
	}
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
		.action-section .segment {
			display: grid;
			grid-template-columns: repeat(2, minmax(0, 1fr));
			width: 100%;
			border-radius: 0.375rem;
			overflow: hidden;
		}

		.action-section .segment button {
			width: 100%;
			min-width: 0;
			min-height: 2.25rem;
			padding: 0.5rem 0.625rem;
			text-align: center;
			white-space: nowrap;
		}

		.action-section .segment button + button {
			border-left: 1px solid var(--border);
		}

		.drawer-card-footer :global(.btn) {
			width: 100%;
			min-width: 0;
		}
	}
	.error {
		color: var(--danger, #dc2626);
		font-size: 0.85rem;
	}
</style>
