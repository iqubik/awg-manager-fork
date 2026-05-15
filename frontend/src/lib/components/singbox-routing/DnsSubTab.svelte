<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { notifications } from '$lib/stores/notifications';
	import { singboxRouter } from '$lib/stores/singboxRouter';
	import {
		Button,
		IconButton,
		Badge,
		Dropdown,
		StatRow,
		ConfirmModal,
	} from '$lib/components/ui';
	import type { DropdownOption, StatTile } from '$lib/components/ui';
	import type {
		SingboxRouterDNSRule,
		SingboxRouterDNSServer,
		SingboxRouterDNSStrategy,
		SingboxRouterRuleSet,
	} from '$lib/types';
	import {
		DNSServerEditModal,
		DNSRuleEditModal,
	} from '$lib/components/routing/singboxRouter';

	const dnsServersStore = singboxRouter.dnsServers;
	const dnsRulesStore = singboxRouter.dnsRules;
	const dnsGlobalsStore = singboxRouter.dnsGlobals;
	const optionsStore = singboxRouter.options;
	const ruleSetsStore = singboxRouter.ruleSets;

	const servers = $derived($dnsServersStore);
	const rules = $derived($dnsRulesStore);
	const globals = $derived($dnsGlobalsStore);
	const outboundOptions = $derived($optionsStore);
	const ruleSets = $derived<SingboxRouterRuleSet[]>($ruleSetsStore);

	async function refresh(): Promise<void> {
		await singboxRouter.loadAll();
	}

	// Bootstrap data on mount in case this sub-tab is the first one visited
	// (e.g. direct navigation to ?sub=dns skips EngineSubTab.onMount).
	onMount(() => {
		void refresh();
	});

	// ── Globals (final + strategy) ────────────────────────────────
	const STRATEGY_OPTIONS: DropdownOption<SingboxRouterDNSStrategy>[] = [
		{ value: '', label: '— default —' },
		{ value: 'ipv4_only', label: 'ipv4_only' },
		{ value: 'ipv6_only', label: 'ipv6_only' },
		{ value: 'prefer_ipv4', label: 'prefer_ipv4' },
		{ value: 'prefer_ipv6', label: 'prefer_ipv6' },
	];

	const finalServerOptions = $derived<DropdownOption[]>([
		{ value: '', label: '— не задан —' },
		...servers.map((s) => ({ value: s.tag, label: s.tag })),
	]);

	let draftFinal = $state('');
	let draftStrategy = $state<SingboxRouterDNSStrategy>('');
	let savingGlobals = $state(false);

	$effect(() => {
		draftFinal = globals.final;
		draftStrategy = globals.strategy;
	});

	const globalsDirty = $derived(
		draftFinal !== globals.final || draftStrategy !== globals.strategy,
	);

	async function saveGlobals(): Promise<void> {
		savingGlobals = true;
		try {
			await api.singboxRouterPutDNSGlobals({
				final: draftFinal,
				strategy: draftStrategy,
			});
			await refresh();
		} catch (e) {
			notifications.error((e as Error).message);
		} finally {
			savingGlobals = false;
		}
	}

	// ── Stat tiles ─────────────────────────────────────────────────
	const statTiles = $derived<StatTile[]>([
		{ label: 'DNS серверов', value: servers.length },
		{
			label: 'DNS правил',
			value: rules.length,
			title: rules.length > 0 ? 'first-match-wins' : '',
		},
		{
			label: 'Strategy',
			value: globals.strategy || '— default —',
			title: globals.final ? `final: ${globals.final}` : 'final: —',
		},
	]);

	// ── Server table helpers ──────────────────────────────────────
	function detourLabel(s: SingboxRouterDNSServer): string {
		if (!s.detour) return 'default';
		return s.detour;
	}

	function detourVariant(
		s: SingboxRouterDNSServer,
	): 'accent' | 'muted' {
		if (!s.detour || s.detour === 'direct') return 'muted';
		return 'accent';
	}

	function typeVariant(
		s: SingboxRouterDNSServer,
	): 'success' | 'muted' {
		return s.type === 'udp' ? 'muted' : 'success';
	}

	function resolverLabel(s: SingboxRouterDNSServer): string {
		if (!s.domain_resolver) return '';
		return s.domain_resolver.server;
	}

	// ── Rule helpers ──────────────────────────────────────────────
	function ruleActionVariant(
		r: SingboxRouterDNSRule,
	): 'success' | 'error' {
		if (r.action === 'reject') return 'error';
		return 'success';
	}

	function ruleActionLabel(r: SingboxRouterDNSRule): string {
		if (r.action === 'reject') return 'REJECT';
		return 'RESOLVE';
	}

	function matcherSummary(r: SingboxRouterDNSRule): string {
		const parts: string[] = [];
		if (r.rule_set?.length) parts.push(`rule_set: ${r.rule_set.join(', ')}`);
		if (r.domain_suffix?.length) {
			const more =
				r.domain_suffix.length > 1
					? ` +${r.domain_suffix.length - 1}`
					: '';
			parts.push(`suffix: ${r.domain_suffix[0]}${more}`);
		}
		if (r.domain?.length) {
			const more = r.domain.length > 1 ? ` +${r.domain.length - 1}` : '';
			parts.push(`domain: ${r.domain[0]}${more}`);
		}
		if (r.domain_keyword?.length) {
			parts.push(`keyword: ${r.domain_keyword[0]}`);
		}
		if (r.query_type?.length) {
			parts.push(`type: ${r.query_type.join(',')}`);
		}
		return parts.join(' · ') || '—';
	}

	// ── Modal / busy state ────────────────────────────────────────
	let serverEditTag = $state<string | null>(null);
	let serverAddMode = $state(false);
	let serverDeleteTag = $state<string | null>(null);
	let serverForceDeleteTag = $state<string | null>(null);
	let serverBusy = $state(false);

	let ruleEditIndex = $state<number | null>(null);
	let ruleAddMode = $state(false);
	let ruleDeleteIndex = $state<number | null>(null);
	let ruleBusy = $state(false);

	function requestServerDelete(tag: string): void {
		serverDeleteTag = tag;
	}

	async function confirmServerDelete(): Promise<void> {
		if (serverDeleteTag === null) return;
		const tag = serverDeleteTag;
		serverBusy = true;
		try {
			await api.singboxRouterDeleteDNSServer(tag, false);
			serverDeleteTag = null;
			await refresh();
		} catch (e) {
			const msg = (e as Error).message;
			serverDeleteTag = null;
			if (msg.includes('referenced')) {
				serverForceDeleteTag = tag;
			} else {
				notifications.error(msg);
			}
		} finally {
			serverBusy = false;
		}
	}

	async function confirmServerForceDelete(): Promise<void> {
		if (serverForceDeleteTag === null) return;
		const tag = serverForceDeleteTag;
		serverBusy = true;
		try {
			await api.singboxRouterDeleteDNSServer(tag, true);
			serverForceDeleteTag = null;
			await refresh();
		} catch (e) {
			notifications.error((e as Error).message);
		} finally {
			serverBusy = false;
		}
	}

	function requestRuleDelete(index: number): void {
		ruleDeleteIndex = index;
	}

	async function confirmRuleDelete(): Promise<void> {
		if (ruleDeleteIndex === null) return;
		ruleBusy = true;
		try {
			await api.singboxRouterDeleteDNSRule(ruleDeleteIndex);
			ruleDeleteIndex = null;
			await refresh();
		} catch (e) {
			notifications.error((e as Error).message);
		} finally {
			ruleBusy = false;
		}
	}

	async function moveRule(index: number, to: number): Promise<void> {
		if (to < 0 || to >= rules.length) return;
		try {
			await api.singboxRouterMoveDNSRule(index, to);
			await refresh();
		} catch (e) {
			notifications.error((e as Error).message);
		}
	}
</script>

<div class="stat-row-wrap">
	<StatRow tiles={statTiles} columns={3} />
</div>

<!-- ── DNS Strategy + final ─────────────────────────────────── -->
<section class="card">
	<header class="card-head">
		<h3 class="card-title">Общие настройки DNS</h3>
		<p class="card-hint">
			<code>final</code> — DNS-сервер по умолчанию (для запросов, не попавших ни под одно правило).
			<code>strategy</code> — глобальная стратегия разрешения IPv4/IPv6.
		</p>
	</header>
	<div class="row-2">
		<label class="field">
			<div class="field-label">Final сервер</div>
			<Dropdown
				bind:value={draftFinal}
				options={finalServerOptions}
				disabled={servers.length === 0}
				fullWidth
			/>
		</label>
		<label class="field">
			<div class="field-label">Strategy (глобальная)</div>
			<Dropdown
				bind:value={draftStrategy}
				options={STRATEGY_OPTIONS}
				fullWidth
			/>
		</label>
	</div>
	<div class="card-actions">
		<Button
			variant="primary"
			size="sm"
			onclick={saveGlobals}
			disabled={savingGlobals || !globalsDirty}
		>
			Сохранить
		</Button>
	</div>
</section>

<!-- ── DNS серверы ─────────────────────────────────────────── -->
<section class="block">
	<div class="action-row">
		<div class="hint">{servers.length} DNS серверов</div>
		<Button
			variant="primary"
			size="sm"
			onclick={() => {
				serverAddMode = true;
				serverEditTag = null;
			}}
		>
			+ Сервер
		</Button>
	</div>

	{#if servers.length === 0}
		<div class="empty empty-warn">
			DNS серверы не настроены. Без них правило <code>hijack-dns</code>
			не будет отвечать на запросы. Добавьте как минимум один сервер
			(например <code>1.1.1.1</code> UDP) чтобы DNS заработал.
		</div>
	{:else}
		<div class="table">
			<div class="t-head t-head-srv">
				<div>Tag</div>
				<div>Type</div>
				<div>Server</div>
				<div>Detour</div>
				<div>Resolver</div>
				<div></div>
			</div>
			{#each servers as s (s.tag)}
				<div class="t-row t-row-srv">
					<div class="mono tag">{s.tag}</div>
					<div>
						<Badge variant={typeVariant(s)} size="sm" uppercase mono>
							{s.type}
						</Badge>
					</div>
					<div class="mono server">
						{s.server}{#if s.server_port}:{s.server_port}{/if}{#if s.path}{s.path}{/if}
					</div>
					<div class="mono detour">
						<Badge variant={detourVariant(s)} size="sm" mono>
							{detourLabel(s)}
						</Badge>
					</div>
					<div class="mono resolver" title={resolverLabel(s)}>
						{resolverLabel(s) || '—'}
					</div>
					<div class="col-edit">
						<IconButton
							ariaLabel="Редактировать"
							size="sm"
							onclick={() => (serverEditTag = s.tag)}
						>
							<svg
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
								aria-hidden="true"
							>
								<path
									d="M12 20h9M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4 12.5-12.5z"
								/>
							</svg>
						</IconButton>
						<IconButton
							ariaLabel="Удалить"
							size="sm"
							variant="danger"
							onclick={() => requestServerDelete(s.tag)}
						>
							<svg
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
								aria-hidden="true"
							>
								<path
									d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"
								/>
							</svg>
						</IconButton>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</section>

<!-- ── DNS правила ─────────────────────────────────────────── -->
<section class="block">
	<div class="action-row">
		<div class="hint">
			{rules.length} правил · first-match-wins · final:
			<strong>{globals.final || '—'}</strong>
		</div>
		<Button
			variant="primary"
			size="sm"
			onclick={() => {
				ruleAddMode = true;
				ruleEditIndex = null;
			}}
			disabled={servers.length === 0}
		>
			+ Правило
		</Button>
	</div>

	{#if servers.length === 0}
		<div class="empty empty-warn">
			Сначала добавьте хотя бы один DNS-сервер — правила ссылаются на tag сервера.
		</div>
	{:else if rules.length === 0}
		<div class="empty">
			Правил нет. Все запросы идут на
			<code>final: {globals.final || '—'}</code>.
		</div>
	{:else}
		<div class="table">
			<div class="t-head t-head-rule">
				<div>#</div>
				<div>Действие</div>
				<div>Matchers</div>
				<div>Server</div>
				<div class="center">Порядок</div>
				<div></div>
			</div>
			{#each rules as r, i (i)}
				<div class="t-row t-row-rule">
					<div class="mono idx">{i}</div>
					<div>
						<Badge variant={ruleActionVariant(r)} size="sm" uppercase mono>
							{ruleActionLabel(r)}
						</Badge>
					</div>
					<div class="mono matcher" title={matcherSummary(r)}>
						{matcherSummary(r)}
					</div>
					<div class="mono rule-server">
						{#if r.action !== 'reject' && r.server}
							<Badge variant="accent" size="sm" mono>{r.server}</Badge>
						{:else}
							<span class="dim">—</span>
						{/if}
					</div>
					<div class="order">
						<button
							class="arrow"
							onclick={() => moveRule(i, i - 1)}
							disabled={i === 0}
							aria-label="Выше"
							type="button">↑</button
						>
						<button
							class="arrow"
							onclick={() => moveRule(i, i + 1)}
							disabled={i === rules.length - 1}
							aria-label="Ниже"
							type="button">↓</button
						>
					</div>
					<div class="col-edit">
						<IconButton
							ariaLabel="Редактировать"
							size="sm"
							onclick={() => (ruleEditIndex = i)}
						>
							<svg
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
								aria-hidden="true"
							>
								<path
									d="M12 20h9M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4 12.5-12.5z"
								/>
							</svg>
						</IconButton>
						<IconButton
							ariaLabel="Удалить"
							size="sm"
							variant="danger"
							onclick={() => requestRuleDelete(i)}
						>
							<svg
								viewBox="0 0 24 24"
								fill="none"
								stroke="currentColor"
								stroke-width="2"
								stroke-linecap="round"
								stroke-linejoin="round"
								aria-hidden="true"
							>
								<path
									d="M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"
								/>
							</svg>
						</IconButton>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</section>

<!-- ── Modals ─────────────────────────────────────────────── -->
{#if serverAddMode}
	<DNSServerEditModal
		{servers}
		{outboundOptions}
		onClose={() => (serverAddMode = false)}
		onSave={async (server) => {
			await api.singboxRouterAddDNSServer(server);
			serverAddMode = false;
			await refresh();
		}}
	/>
{/if}

{#if serverEditTag !== null}
	{@const tag = serverEditTag}
	{@const existing = servers.find((s) => s.tag === tag)}
	{#if existing}
		<DNSServerEditModal
			server={existing}
			{servers}
			{outboundOptions}
			onClose={() => (serverEditTag = null)}
			onSave={async (server) => {
				await api.singboxRouterUpdateDNSServer(tag, server);
				serverEditTag = null;
				await refresh();
			}}
		/>
	{/if}
{/if}

{#if ruleAddMode}
	<DNSRuleEditModal
		{servers}
		availableRuleSets={ruleSets}
		onClose={() => (ruleAddMode = false)}
		onSave={async (rule) => {
			await api.singboxRouterAddDNSRule(rule);
			ruleAddMode = false;
			await refresh();
		}}
	/>
{/if}

{#if ruleEditIndex !== null}
	{@const idx = ruleEditIndex}
	<DNSRuleEditModal
		rule={rules[idx]}
		{servers}
		availableRuleSets={ruleSets}
		onClose={() => (ruleEditIndex = null)}
		onSave={async (rule) => {
			await api.singboxRouterUpdateDNSRule(idx, rule);
			ruleEditIndex = null;
			await refresh();
		}}
	/>
{/if}

<ConfirmModal
	open={serverDeleteTag !== null}
	title="Удалить DNS-сервер"
	message={serverDeleteTag !== null
		? `Удалить DNS-сервер "${serverDeleteTag}"?`
		: ''}
	busy={serverBusy}
	onConfirm={confirmServerDelete}
	onClose={() => {
		if (!serverBusy) serverDeleteTag = null;
	}}
/>

<ConfirmModal
	open={serverForceDeleteTag !== null}
	title="Удалить с потерей ссылок?"
	message="На этот DNS-сервер ссылаются правила или другие серверы."
	secondary="Удалить всё равно? Зависимые правила могут перестать работать."
	confirmLabel="Удалить принудительно"
	busy={serverBusy}
	onConfirm={confirmServerForceDelete}
	onClose={() => {
		if (!serverBusy) serverForceDeleteTag = null;
	}}
/>

<ConfirmModal
	open={ruleDeleteIndex !== null}
	title="Удалить DNS-правило"
	message={ruleDeleteIndex !== null
		? `Удалить DNS-правило #${ruleDeleteIndex}?`
		: ''}
	busy={ruleBusy}
	onConfirm={confirmRuleDelete}
	onClose={() => {
		if (!ruleBusy) ruleDeleteIndex = null;
	}}
/>

<style>
	.stat-row-wrap {
		margin-bottom: 1rem;
	}

	.card {
		background: var(--color-bg-secondary);
		border: 1px solid var(--color-border);
		border-radius: var(--radius);
		padding: 0.875rem 1rem;
		margin-bottom: 1rem;
	}
	.card-head {
		margin-bottom: 0.75rem;
	}
	.card-title {
		font-size: 0.875rem;
		font-weight: 600;
		color: var(--color-text-primary);
		margin: 0 0 0.25rem 0;
	}
	.card-hint {
		font-size: 0.75rem;
		color: var(--color-text-muted);
		margin: 0;
		line-height: 1.5;
	}
	.card-hint code {
		font-family: var(--font-mono, ui-monospace, monospace);
		background: var(--color-bg-tertiary);
		padding: 0.05rem 0.3rem;
		border-radius: 3px;
		color: var(--color-text-secondary);
		font-size: 0.7rem;
	}
	.row-2 {
		display: grid;
		grid-template-columns: 1fr 1fr;
		gap: 0.75rem;
	}
	@media (max-width: 600px) {
		.row-2 {
			grid-template-columns: 1fr;
		}
	}
	.field {
		display: grid;
		gap: 0.3rem;
	}
	.field-label {
		font-size: 0.75rem;
		color: var(--color-text-muted);
	}
	.card-actions {
		margin-top: 0.75rem;
		display: flex;
		justify-content: flex-end;
	}

	.block {
		margin-bottom: 1.25rem;
	}

	.action-row {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 0.75rem;
		margin-bottom: 0.625rem;
		flex-wrap: wrap;
	}
	.hint {
		font-size: 0.8rem;
		color: var(--color-text-muted);
	}
	.hint strong {
		color: var(--color-success);
		font-family: var(--font-mono, ui-monospace, monospace);
		font-weight: 600;
	}

	.table {
		display: flex;
		flex-direction: column;
		gap: 0.25rem;
		border: 1px solid var(--color-border);
		border-radius: var(--radius);
		overflow: hidden;
		background: var(--color-bg-secondary);
	}
	.t-head,
	.t-row {
		display: grid;
		gap: 0.625rem;
		align-items: center;
		padding: 0.5rem 0.875rem;
	}
	.t-head {
		background: var(--color-bg-tertiary);
		border-bottom: 1px solid var(--color-border);
		font-size: 0.6875rem;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--color-text-muted);
		padding: 0.45rem 0.875rem;
	}
	.t-head .center {
		text-align: center;
	}
	.t-row {
		border-bottom: 1px solid var(--color-border);
	}
	.t-row:last-child {
		border-bottom: none;
	}
	.t-head-srv,
	.t-row-srv {
		grid-template-columns: 130px 70px 1fr 130px 130px 72px;
	}
	.t-head-rule,
	.t-row-rule {
		grid-template-columns: 32px 88px 1fr 140px 64px 72px;
	}

	.mono {
		font-family: var(--font-mono, ui-monospace, monospace);
		font-size: 0.8rem;
	}
	.tag {
		color: var(--color-text-primary);
		font-weight: 600;
	}
	.idx {
		color: var(--color-text-muted);
	}
	.server,
	.matcher,
	.resolver {
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.server {
		color: var(--color-text-primary);
	}
	.matcher {
		color: var(--color-text-secondary);
	}
	.resolver {
		color: var(--color-text-muted);
	}
	.detour {
		display: flex;
	}
	.dim {
		color: var(--color-text-muted);
	}

	.col-edit {
		display: flex;
		gap: 0.25rem;
		justify-content: flex-end;
	}

	.order {
		display: flex;
		gap: 2px;
		justify-content: center;
	}
	.arrow {
		background: var(--color-bg);
		border: 1px solid var(--color-border);
		color: var(--color-text-muted);
		width: 26px;
		height: 26px;
		border-radius: 3px;
		cursor: pointer;
		padding: 0;
		font-size: 0.75rem;
	}
	.arrow:disabled {
		opacity: 0.3;
		cursor: not-allowed;
	}

	.empty {
		padding: 0.75rem 0.9rem;
		background: var(--color-bg-secondary);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-sm);
		color: var(--color-text-muted);
		font-size: 0.85rem;
		line-height: 1.5;
	}
	.empty-warn {
		background: color-mix(in srgb, var(--color-warning) 12%, transparent);
		border: 1px solid color-mix(in srgb, var(--color-warning) 40%, transparent);
		border-left-width: 3px;
	}
	.empty code {
		font-family: var(--font-mono, ui-monospace, monospace);
		background: var(--color-bg-tertiary);
		padding: 0.05rem 0.3rem;
		border-radius: 3px;
		color: var(--color-text-secondary);
	}

	@media (max-width: 720px) {
		.t-head-srv,
		.t-row-srv {
			grid-template-columns: 110px 60px 1fr 100px 72px;
		}
		.t-head-srv > :nth-child(5),
		.t-row-srv > :nth-child(5) {
			display: none;
		}
		.t-head-rule,
		.t-row-rule {
			grid-template-columns: 28px 80px 1fr 60px 72px;
		}
		.t-head-rule > :nth-child(4),
		.t-row-rule > :nth-child(4) {
			display: none;
		}
	}
</style>
