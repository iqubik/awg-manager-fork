<script lang="ts">
	import Modal from '$lib/components/ui/Modal.svelte';
	import { Dropdown, type DropdownOption } from '$lib/components/ui';
	import type { SingboxRouterRuleSet } from '$lib/types';
	import type { OutboundGroup } from './outboundOptions';
	import { parseInlineRuleList } from '$lib/utils/singboxInlineRules';

	interface Props {
		ruleSet?: SingboxRouterRuleSet;
		outboundOptions: OutboundGroup[];
		onClose: () => void;
		onSave: (rs: SingboxRouterRuleSet) => Promise<void> | void;
	}
	let { ruleSet, outboundOptions, onClose, onSave }: Props = $props();

	// ── constants ───────────────────────────────────────────────
	const UPDATE_INTERVAL_OPTIONS: DropdownOption[] = [
		{ value: '6h', label: '6h' },
		{ value: '12h', label: '12h' },
		{ value: '24h', label: '24h (рекомендуется)' },
		{ value: '168h', label: '168h (неделя)' },
	];

	const DEFAULT_RULES_LIST = `# Домены
openai.com
chatgpt.com
*.perplexity.ai
https://gemini.google.com/app

# IP/CIDR
1.1.1.1
8.8.8.0/24

# Дополнительно
keyword:youtube`;

	// ── derived ────────────────────────────────────────────────
	const downloadDetourOptions = $derived<DropdownOption[]>([
		{ value: '', label: 'автоматически (direct)' },
		...outboundOptions.flatMap((g) =>
			g.items.map((i) => ({ value: i.value, label: i.label, group: g.group })),
		),
	]);

	const isEditing = $derived(Boolean(ruleSet));

	// ── inline preview derived ──────────────────────────────────
	const listParsePreview = $derived.by(() => {
		if (type !== 'inline' || inlineMode !== 'list') {
			return { rules: [] as Record<string, unknown>[], warnings: [] as string[], errors: [] as string[] };
		}
		return parseInlineRuleList(rulesList);
	});

	// ── line numbers for rules-list textarea ─────────────────────
	const rulesListLineNumbers = $derived.by(() => {
		const count = Math.max(1, rulesList.split(/\r?\n/).length);
		return Array.from({ length: count }, (_, i) => String(i + 1)).join('\n');
	});

	let rulesListTextarea = $state<HTMLTextAreaElement | null>(null);
	let rulesListLineNumberGutter = $state<HTMLPreElement | null>(null);

	function syncRulesListLineNumbersScroll(): void {
		if (!rulesListTextarea || !rulesListLineNumberGutter) return;
		rulesListLineNumberGutter.scrollTop = rulesListTextarea.scrollTop;
	}

	// ── form state ──────────────────────────────────────────────
	// For inline rule sets: 'list' = smart-line-by-line, 'json' = raw JSON array
	// svelte-ignore state_referenced_locally
	let inlineMode: 'list' | 'json' = $state(ruleSet?.rules?.length ? 'json' : 'list');

	let rulesList = $state(DEFAULT_RULES_LIST);

	// svelte-ignore state_referenced_locally
	let type: 'remote' | 'local' | 'inline' = $state(ruleSet?.type ?? 'remote');
	// svelte-ignore state_referenced_locally
	let format: 'binary' | 'source' = $state(ruleSet?.format ?? 'binary');
	// svelte-ignore state_referenced_locally
	let tag = $state(ruleSet?.tag ?? '');
	// svelte-ignore state_referenced_locally
	let url = $state(ruleSet?.url ?? '');
	// svelte-ignore state_referenced_locally
	let updateInterval = $state(ruleSet?.update_interval ?? '24h');
	// svelte-ignore state_referenced_locally
	let downloadDetour = $state(ruleSet?.download_detour ?? '');
	// svelte-ignore state_referenced_locally
	let path = $state(ruleSet?.path ?? '');
	// svelte-ignore state_referenced_locally
	let rulesJson = $state(
		ruleSet?.rules?.length
			? JSON.stringify(ruleSet.rules, null, 2)
			: `[
  {
    "domain_suffix": [
      ".example.com"
    ]
  }
]`,
	);

	let busy = $state(false);
	let error = $state('');

	// Default rulesJson template for new rule sets (must match $state initializer above)
	const DEFAULT_RULES_JSON = `[
  {
    "domain_suffix": [
      ".example.com"
    ]
  }
]`;

	// Snapshot initial state for isDirty detection
	let initialInlineMode: 'list' | 'json' = $state('list');
	let initialType: 'remote' | 'local' | 'inline' = $state('remote');
	let initialFormat: 'binary' | 'source' = $state('binary');
	let initialTag = $state('');
	let initialUrl = $state('');
	let initialUpdateInterval = $state('24h');
	let initialDownloadDetour = $state('');
	let initialPath = $state('');
	let initialRulesList = $state('');
	let initialRulesJson = $state('');

	// Initialize snapshot when modal opens
	$effect(() => {
		if (ruleSet) {
			initialType = ruleSet.type;
			initialFormat = ruleSet.format ?? 'binary';
			initialTag = ruleSet.tag;
			initialUrl = ruleSet.url ?? '';
			initialUpdateInterval = ruleSet.update_interval ?? '24h';
			initialDownloadDetour = ruleSet.download_detour ?? '';
			initialPath = ruleSet.path ?? '';
			initialRulesJson = ruleSet.rules?.length ? JSON.stringify(ruleSet.rules, null, 2) : '';
			initialInlineMode = 'json';
			initialRulesList = DEFAULT_RULES_LIST;
		} else {
			initialType = 'remote';
			initialFormat = 'binary';
			initialTag = '';
			initialUrl = '';
			initialUpdateInterval = '24h';
			initialDownloadDetour = '';
			initialPath = '';
			initialRulesJson = DEFAULT_RULES_JSON;
			initialInlineMode = 'list';
			initialRulesList = DEFAULT_RULES_LIST;
		}
	});

	const isDirty = $derived.by(() => {
		return (
			type !== initialType ||
			format !== initialFormat ||
			tag !== initialTag ||
			url !== initialUrl ||
			updateInterval !== initialUpdateInterval ||
			downloadDetour !== initialDownloadDetour ||
			path !== initialPath ||
			inlineMode !== initialInlineMode ||
			rulesList !== initialRulesList ||
			rulesJson !== initialRulesJson
		);
	});

	async function save(): Promise<void> {
		busy = true;
		error = '';
		try {
			const cleanTag = isEditing ? (ruleSet?.tag ?? '') : tag.trim();
			if (!cleanTag) {
				error = 'Tag обязателен';
				busy = false;
				return;
			}
			if (type === 'remote' && !url.trim()) {
				error = 'URL обязателен для type=remote';
				busy = false;
				return;
			}
			if (type === 'local' && !path.trim()) {
				error = 'Path обязателен для type=local';
				busy = false;
				return;
			}

			let parsedRules: Record<string, unknown>[] | undefined;
			if (type === 'inline') {
				if (inlineMode === 'json') {
					try {
						const parsed = JSON.parse(rulesJson);
						if (!Array.isArray(parsed) || parsed.length === 0) {
							error = 'Для inline rule set нужен непустой JSON-массив правил';
							busy = false;
							return;
						}
						parsedRules = parsed as Record<string, unknown>[];
					} catch (e) {
						error = `Некорректный JSON: ${(e as Error).message}`;
						busy = false;
						return;
					}
				} else {
					const parsed = parseInlineRuleList(rulesList);
					if (parsed.errors.length > 0) {
						error = parsed.errors.join('\n');
						busy = false;
						return;
					}
					if (parsed.rules.length === 0) {
						error = 'Нет валидных строк для inline rule set';
						busy = false;
						return;
					}
					parsedRules = parsed.rules;
				}
			}

			const built: SingboxRouterRuleSet = {
				tag: cleanTag,
				type,
				format: type === 'inline' ? undefined : format,
				url: type === 'remote' ? url.trim() : undefined,
				update_interval: type === 'remote' ? updateInterval : undefined,
				download_detour: type === 'remote' && downloadDetour ? downloadDetour : undefined,
				path: type === 'local' ? path.trim() : undefined,
				rules: type === 'inline' ? parsedRules : undefined,
			};
			await onSave(built);
		} catch (e) {
			error = (e as Error).message;
		} finally {
			busy = false;
		}
	}
</script>

<Modal open onclose={onClose} title={ruleSet ? 'Редактировать rule set' : 'Новый rule set'} hasUnsavedChanges={() => isDirty}>
	<div class="form">
		<div class="section-label">Тип</div>
		<div class="segment">
			<button class:active={type === 'remote'} onclick={() => (type = 'remote')} type="button">Remote</button>
			<button class:active={type === 'local'} onclick={() => (type = 'local')} type="button">Local</button>
			<button class:active={type === 'inline'} onclick={() => (type = 'inline')} type="button">Inline</button>
		</div>

		<label class="field">
			<div class="lbl">Tag (имя)</div>
			<input bind:value={tag} placeholder="geosite-example" disabled={isEditing} />
			{#if isEditing}<div class="hint">Tag нельзя менять у существующего набора.</div>{/if}
		</label>

		{#if type !== 'inline'}
			<label class="field">
				<div class="lbl">Формат</div>
				<div class="segment">
					<button class:active={format === 'binary'} onclick={() => (format = 'binary')} type="button">Binary (.srs)</button>
					<button class:active={format === 'source'} onclick={() => (format = 'source')} type="button">Source (JSON)</button>
				</div>
			</label>
		{/if}

		{#if type === 'remote'}
			<label class="field">
				<div class="lbl">URL к файлу</div>
				<input bind:value={url} placeholder="https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-example.srs" />
			</label>

			<label class="field">
				<div class="lbl">Интервал обновления</div>
				<Dropdown bind:value={updateInterval} options={UPDATE_INTERVAL_OPTIONS} fullWidth />
			</label>

			<div class="field highlight">
				<div class="lbl">Скачивать через (download detour)</div>
				<Dropdown bind:value={downloadDetour} options={downloadDetourOptions} fullWidth />
				<div class="hint">
					Через какой outbound скачивать этот файл. Полезно если URL заблокирован у провайдера — используйте VPN-туннель.
				</div>
			</div>
		{:else if type === 'local'}
			<label class="field">
				<div class="lbl">Путь к файлу</div>
				<input bind:value={path} placeholder="/opt/etc/awg-manager/singbox/rulesets/my-custom.srs" />
				<div class="hint">Абсолютный путь. Файл должен существовать на роутере.</div>
			</label>
		{:else}
			<div class="field">
				<div class="lbl">Формат ввода</div>
				<div class="segment">
					<button class:active={inlineMode === 'list'} onclick={() => (inlineMode = 'list')} type="button">
						Список
					</button>
					<button class:active={inlineMode === 'json'} onclick={() => (inlineMode = 'json')} type="button">
						JSON
					</button>
				</div>
			</div>

			{#if inlineMode === 'list'}
				<div class="field">
					<div class="lbl">Список правил</div>
					<div class="rules-editor">
						<pre class="line-numbers" aria-hidden="true" bind:this={rulesListLineNumberGutter}>{rulesListLineNumbers}</pre>
						<textarea
							class="rules-json rules-list-textarea"
							bind:this={rulesListTextarea}
							bind:value={rulesList}
							rows="12"
							spellcheck="false"
							wrap="off"
							onscroll={syncRulesListLineNumbersScroll}
						></textarea>
					</div>
				<details class="inline-help">
					<summary>Подсказка по формату списка</summary>

					<div class="inline-help-body">
						<div>
							<span class="help-label">Поддерживается:</span>
							домены, URL, wildcard <code>*.example.com</code>, IP/CIDR,
							<code>keyword:</code>, <code>regex:</code>.
						</div>
						<div>
							<span class="help-label">Расширенные matchers:</span>
							<code>port:</code>, <code>process:</code>, <code>package:</code>,
							<code>network:</code>.
						</div>
						<div>
							<span class="help-label">Осторожно:</span>
							<code>port:443</code> создаёт отдельное правило для любого HTTPS-трафика,
							а <code>process:</code> на Keenetic/Entware относится к локальным процессам,
							не к LAN-клиентам.
						</div>
						<div>
							<span class="help-label">Пока не поддерживается:</span>
							исключения <code>@@</code>, <code>port_range:</code>, логические правила
							<code>and/or</code> и произвольные JSON-поля sing-box. Для них используйте
							режим <code>JSON</code>.
						</div>
					</div>
				</details>
				</div>

				{#if listParsePreview.errors.length > 0}
					<div class="parse-messages parse-messages-error">
						<div class="parse-messages-title">Ошибки разбора</div>
						<ul>
							{#each listParsePreview.errors as msg}
								<li>{msg}</li>
							{/each}
						</ul>
					</div>
				{/if}
				{#if listParsePreview.warnings.length > 0}
					<div class="parse-messages parse-messages-warning">
						<div class="parse-messages-title">Предупреждения</div>
						<ul>
							{#each listParsePreview.warnings as msg}
								<li>{msg}</li>
							{/each}
						</ul>
					</div>
				{/if}
				{#if listParsePreview.rules.length > 0}
					<div class="info">
						Будет создано групп правил: {listParsePreview.rules.length}
					</div>
				{/if}
				{#if listParsePreview.rules.length > 0}
					<details class="json-preview">
						<summary>Предпросмотр JSON</summary>
						<pre>{JSON.stringify(listParsePreview.rules, null, 2)}</pre>
					</details>
				{/if}
			{:else}
				<label class="field">
					<div class="lbl">Правила (JSON-массив)</div>
					<textarea class="rules-json" bind:value={rulesJson} rows="10" spellcheck="false"></textarea>
					<div class="hint">
						Advanced mode: массив объектов с матчерами sing-box.
					</div>
				</label>
			{/if}
		{/if}

		{#if error}<div class="error">{error}</div>{/if}

		<div class="actions">
			<button class="btn btn-secondary" onclick={onClose} type="button">Отмена</button>
			<button class="btn btn-primary" onclick={save} disabled={busy} type="button">Сохранить</button>
		</div>
	</div>
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
	}
	.field {
		display: grid;
		gap: 0.25rem;
	}
	.field.highlight {
		padding: 0.6rem;
		background: var(--bg);
		border-left: 2px solid var(--accent, #3b82f6);
		border-radius: 4px;
	}
	.lbl {
		font-size: 0.75rem;
		color: var(--muted-text);
	}
	.hint {
		font-size: 0.75rem;
		color: var(--muted-text);
		line-height: 1.4;
		margin-top: 0.25rem;
	}
	.inline-help {
		margin-top: 0.35rem;
		padding: 0.5rem 0.65rem;
		border: 1px solid var(--border);
		border-radius: 0.45rem;
		background: var(--surface-1, rgba(255, 255, 255, 0.035));
		color: var(--muted-text);
		font-size: 0.8rem;
		line-height: 1.45;
	}

	.inline-help summary {
		cursor: pointer;
		color: var(--text);
		font-weight: 700;
		outline: none;
	}

	.inline-help-body {
		margin-top: 0.45rem;
		display: grid;
		gap: 0.28rem;
	}

	.inline-help code {
		background: var(--surface-2, rgba(255, 255, 255, 0.06));
		border-radius: 0.25rem;
		padding: 0.05rem 0.25rem;
		font-size: 0.78rem;
	}

	.help-label {
		color: var(--text);
		font-weight: 600;
	}
	.field input {
		background: var(--bg);
		border: 1px solid var(--border);
		padding: 0.4rem 0.6rem;
		border-radius: 4px;
		color: var(--text);
		font-family: ui-monospace, monospace;
		font-size: 0.85rem;
		width: 100%;
		box-sizing: border-box;
	}
	.field input:disabled {
		opacity: 0.7;
		cursor: not-allowed;
	}
	.rules-json {
		background: var(--bg);
		border: 1px solid var(--border);
		padding: 0.5rem 0.6rem;
		border-radius: 4px;
		color: var(--text);
		font-family: ui-monospace, monospace;
		font-size: 0.8rem;
		width: 100%;
		box-sizing: border-box;
		resize: vertical;
		line-height: 1.45;
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
	.error {
		color: var(--danger, #dc2626);
		font-size: 0.85rem;
	}
	.parse-messages {
		padding: 0.55rem 0.65rem;
		border: 1px solid var(--border);
		border-radius: 0.45rem;
		background: var(--surface-1, rgba(255, 255, 255, 0.035));
		font-size: 0.82rem;
		line-height: 1.4;
	}

	.parse-messages-title {
		font-weight: 700;
		margin-bottom: 0.35rem;
	}

	.parse-messages ul {
		margin: 0;
		padding-left: 1.1rem;
		display: grid;
		gap: 0.22rem;
	}

	.parse-messages li {
		margin: 0;
	}

	.parse-messages-error {
		border-color: var(--danger, #dc2626);
		color: var(--danger, #dc2626);
		background: rgba(220, 38, 38, 0.08);
	}

	.parse-messages-warning {
		border-color: var(--color-warning, #d97706);
		color: var(--color-warning, #d97706);
		background: rgba(217, 119, 6, 0.08);
	}
	.info {
		color: #10b981;
		font-size: 0.85rem;
	}
	.json-preview {
		margin: 0;
	}
	.json-preview summary {
		cursor: pointer;
		font-size: 0.85rem;
		color: var(--accent, #3b82f6);
		outline: none;
	}
	.json-preview pre {
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 4px;
		padding: 0.5rem 0.6rem;
		font-size: 0.75rem;
		overflow-x: auto;
		margin-top: 0.25rem;
	}
	.actions {
		display: flex;
		justify-content: flex-end;
		gap: 0.5rem;
	}
	.rules-editor {
		display: grid;
		grid-template-columns: auto 1fr;
		height: 16rem;
		background: var(--bg);
		border: 1px solid var(--border);
		border-radius: 4px;
		overflow: hidden;
		min-width: 0;
		align-items: stretch;
	}
	.line-numbers {
		margin: 0;
		padding: 0.5rem 0.45rem 0.5rem 0.55rem;
		border-right: 1px solid var(--border);
		background: var(--bg);
		color: var(--muted-text);
		font-family: ui-monospace, monospace;
		font-size: 0.8rem;
		line-height: 1.45;
		text-align: right;
		user-select: none;
		overflow: hidden;
		min-width: 2.25rem;
		white-space: pre;
		height: 100%;
		box-sizing: border-box;
	}
	.rules-list-textarea {
		border: 0;
		border-radius: 0;
		background: transparent;
		min-width: 0;
		height: 100%;
		overflow: auto;
		resize: none;
		white-space: pre;
		box-sizing: border-box;
	}

</style>
