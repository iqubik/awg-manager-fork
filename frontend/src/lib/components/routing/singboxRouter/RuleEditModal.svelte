<script lang="ts">
	import Modal from '$lib/components/ui/Modal.svelte';
	import { Button, Dropdown, ChipMultiSelect, type DropdownOption, type ChipOption } from '$lib/components/ui';
	import type { SingboxRouterRule, SingboxRouterRuleSet } from '$lib/types';
	import type { OutboundGroup } from './outboundOptions';

	interface Props {
		rule?: SingboxRouterRule;
		outboundOptions: OutboundGroup[];
		availableRuleSets: SingboxRouterRuleSet[];
		/**
		 * Pre-populated rule_set tags for the add-rule path (when `rule` is
		 * undefined). Used by the deep-link prefill flow from rule set cards.
		 * Ignored when `rule` is provided (edit mode reads rule.rule_set).
		 */
		initialRuleSetTags?: string[];
		/**
		 * Per-tag count of how many *other* router rules reference each
		 * rule_set. The currently edited rule must be excluded by the caller
		 * (use computeRuleSetUsage with excludeIndex=editIndex). Empty map
		 * is fine — all sets render as unused.
		 */
		ruleSetUsage?: Map<string, number>;
		onClose: () => void;
		onSave: (rule: SingboxRouterRule) => Promise<void> | void;
	}
	let {
		rule,
		outboundOptions,
		availableRuleSets,
		initialRuleSetTags,
		ruleSetUsage,
		onClose,
		onSave,
	}: Props = $props();

	const outboundDropdownOptions = $derived<DropdownOption[]>([
		{ value: '', label: '— выберите —' },
		...outboundOptions.flatMap((g) =>
			g.items.map((i) => ({ value: i.value, label: i.label, group: g.group })),
		),
	]);

	// svelte-ignore state_referenced_locally
	let domainSuffixStr = $state((rule?.domain_suffix ?? []).join('\n'));
	// svelte-ignore state_referenced_locally
	let ipCidrStr = $state((rule?.ip_cidr ?? []).join('\n'));
	// svelte-ignore state_referenced_locally
	let sourceIpCidrStr = $state((rule?.source_ip_cidr ?? []).join('\n'));
	// svelte-ignore state_referenced_locally
	let ruleSetTags = $state<string[]>(rule?.rule_set ?? initialRuleSetTags ?? []);
	const ruleSetOptions = $derived<ChipOption[]>(
		availableRuleSets.map((rs) => ({
			value: rs.tag,
			label: rs.tag,
			usedCount: ruleSetUsage?.get(rs.tag) ?? 0,
		})),
	);
	// svelte-ignore state_referenced_locally
	let portStr = $state((rule?.port ?? []).join(', '));

	// svelte-ignore state_referenced_locally
	let action: 'route' | 'reject' = $state((rule?.action === 'reject' ? 'reject' : 'route'));
	// svelte-ignore state_referenced_locally
	let outbound = $state(rule?.outbound ?? '');

	let busy = $state(false);
	let error = $state('');

	// Snapshot initial state for isDirty detection
	let initialDomainSuffixStr = $state('');
	let initialIpCidrStr = $state('');
	let initialSourceIpCidrStr = $state('');
	let initialRuleSetTagsSnapshot = $state<string[]>([]);
	let initialPortStr = $state('');
	let initialAction: 'route' | 'reject' = $state('route');
	let initialOutbound = $state('');

	// Initialize snapshot when modal opens
	$effect(() => {
		if (rule) {
			initialDomainSuffixStr = (rule.domain_suffix ?? []).join('\n');
			initialIpCidrStr = (rule.ip_cidr ?? []).join('\n');
			initialSourceIpCidrStr = (rule.source_ip_cidr ?? []).join('\n');
			initialRuleSetTagsSnapshot = [...(rule.rule_set ?? [])];
			initialPortStr = (rule.port ?? []).join(', ');
			initialAction = rule.action === 'reject' ? 'reject' : 'route';
			initialOutbound = rule.outbound ?? '';
		} else {
			initialDomainSuffixStr = '';
			initialIpCidrStr = '';
			initialSourceIpCidrStr = '';
			initialRuleSetTagsSnapshot = [...(initialRuleSetTags ?? [])];
			initialPortStr = '';
			initialAction = 'route';
			initialOutbound = '';
		}
	});

	const isDirty = $derived.by(() => {
		return (
			domainSuffixStr !== initialDomainSuffixStr ||
			ipCidrStr !== initialIpCidrStr ||
			sourceIpCidrStr !== initialSourceIpCidrStr ||
			[...ruleSetTags].join(',') !== [...initialRuleSetTagsSnapshot].join(',') ||
			portStr !== initialPortStr ||
			action !== initialAction ||
			outbound !== initialOutbound
		);
	});

	function parseLines(text: string): string[] {
		return text.split('\n').map((s) => s.trim()).filter(Boolean);
	}

	const domainsCount = $derived(parseLines(domainSuffixStr).length);
	const ipsCount = $derived(parseLines(ipCidrStr).length);
	const sourceIPsCount = $derived(parseLines(sourceIpCidrStr).length);

	async function save(): Promise<void> {
		busy = true;
		error = '';
		try {
			const domain_suffix = parseLines(domainSuffixStr);
			const ip_cidr = parseLines(ipCidrStr);
			const source_ip_cidr = parseLines(sourceIpCidrStr);
			const rule_set = ruleSetTags;
			const port = portStr
				.split(',')
				.map((s) => parseInt(s.trim(), 10))
				.filter((n) => !isNaN(n));

			const hasMatcher =
				domain_suffix.length > 0 ||
				ip_cidr.length > 0 ||
				source_ip_cidr.length > 0 ||
				rule_set.length > 0 ||
				port.length > 0;
			if (!hasMatcher) {
				error = 'Нужен хотя бы один matcher';
				busy = false;
				return;
			}
			if (action === 'route' && !outbound) {
				error = 'Выберите outbound для действия "Направить"';
				busy = false;
				return;
			}

			const built: SingboxRouterRule = {
				domain_suffix: domain_suffix.length ? domain_suffix : undefined,
				ip_cidr: ip_cidr.length ? ip_cidr : undefined,
				source_ip_cidr: source_ip_cidr.length ? source_ip_cidr : undefined,
				rule_set: rule_set.length ? rule_set : undefined,
				port: port.length ? port : undefined,
				action,
				outbound: action === 'route' ? outbound : undefined,
			};

			await onSave(built);
		} catch (e) {
			error = (e as Error).message;
		} finally {
			busy = false;
		}
	}
</script>

<Modal open onclose={onClose} title={rule ? 'Редактировать правило' : 'Новое правило'} hasUnsavedChanges={() => isDirty}>
	<div class="form">
		<div class="section-label">Matchers (минимум один)</div>

		<label class="field">
			<div class="field-head">
				<span class="lbl">Domain suffix</span>
				{#if domainsCount > 0}
					<span class="count-chip">
						{domainsCount}
						{domainsCount === 1 ? 'домен' : domainsCount < 5 ? 'домена' : 'доменов'}
					</span>
				{/if}
			</div>
			<textarea bind:value={domainSuffixStr} rows="6" placeholder="по одному на строке, например youtube.com"></textarea>
		</label>

		<label class="field">
			<div class="field-head">
				<span class="lbl">IP CIDR</span>
				{#if ipsCount > 0}
					<span class="count-chip">
						{ipsCount}
						{ipsCount === 1 ? 'подсеть' : ipsCount < 5 ? 'подсети' : 'подсетей'}
					</span>
				{/if}
			</div>
			<textarea bind:value={ipCidrStr} rows="6" placeholder="142.250.0.0/15"></textarea>
		</label>

		<label class="field">
			<div class="field-head">
				<span class="lbl">Source IP CIDR</span>
				{#if sourceIPsCount > 0}
					<span class="count-chip">
						{sourceIPsCount}
						{sourceIPsCount === 1 ? 'источник' : sourceIPsCount < 5 ? 'источника' : 'источников'}
					</span>
				{/if}
			</div>
			<textarea bind:value={sourceIpCidrStr} rows="6" placeholder="192.168.1.50"></textarea>
		</label>

		<div class="field">
			<div class="lbl">Rule sets</div>
			<ChipMultiSelect
				values={ruleSetTags}
				options={ruleSetOptions}
				onchange={(next) => (ruleSetTags = next)}
				placeholder="не выбрано"
				allowOrphans
			/>
			<div class="hint">
				Готовые наборы (geosite/geoip). Для своих доменов и подсетей используйте поля выше.
			</div>
		</div>

		<label class="field">
			<div class="lbl">Порты (через запятую)</div>
			<input bind:value={portStr} placeholder="443, 80" />
			<div class="hint">
				Необязательно. Дополнительно ограничивает правило конкретными портами.
			</div>
		</label>

		<div class="action-section">
			<div class="section-label">Действие</div>
			<div class="segment">
				<button class:active={action === 'route'} onclick={() => (action = 'route')} type="button">Направить</button>
				<button class:active={action === 'reject'} onclick={() => (action = 'reject')} type="button">Заблокировать</button>
			</div>

			{#if action === 'route'}
				<label class="field">
					<div class="lbl">Куда направить</div>
					<Dropdown bind:value={outbound} options={outboundDropdownOptions} fullWidth />
				</label>
			{/if}
		</div>

		{#if error}<div class="error">{error}</div>{/if}
	</div>

	{#snippet actions()}
		<Button variant="ghost" size="md" onclick={onClose} type="button">Отмена</Button>
		<Button variant="primary" size="md" onclick={save} disabled={busy} loading={busy} type="button">
			Сохранить
		</Button>
	{/snippet}
</Modal>

<style>
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
	.field-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
	}
	.count-chip {
		font-size: 0.7rem;
		color: var(--muted-text);
		padding: 0.1rem 0.45rem;
		border: 1px solid var(--border);
		border-radius: 999px;
		font-family: ui-monospace, monospace;
		white-space: nowrap;
	}
	.hint {
		font-size: 0.72rem;
		color: var(--muted-text);
		line-height: 1.4;
		margin-top: 0.15rem;
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
		.segment {
			display: grid;
			grid-template-columns: repeat(2, minmax(0, 1fr));
			width: 100%;
			border-radius: 0.375rem;
		}

		.segment button {
			min-width: 0;
			width: 100%;
			min-height: 2.375rem;
			padding: 0.5rem 0.625rem;
			text-align: center;
			white-space: nowrap;
		}

		.segment button + button {
			border-left: 1px solid var(--border);
		}
	}
	.error {
		color: var(--danger, #dc2626);
		font-size: 0.85rem;
	}
</style>
