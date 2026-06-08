<script lang="ts">
	import { protocols, getSignaturePackets, calcByteSize, type ProtocolKey } from '$lib/utils/protocols';
	import { api } from '$lib/api/client';
	import type { ASCParams, ASCParamsExtended } from '$lib/types';
	import { isExtendedASCParams } from '$lib/utils/asc-validation';
	import { AWG_PARAM_HINTS } from '$lib/utils/awgParamHints';
	import { Button, Dropdown, FieldHint, type DropdownOption } from '$lib/components/ui';

	const MAX_SIGNATURE_BYTES = 4096;

	type GenerateMode = 'protocol' | 'domain';
	type SignatureModes = 'both' | 'domain';

	type ASCErrorFields = Partial<Record<keyof ASCParamsExtended, string[]>>;

	let {
		params = $bindable(),
		extended = undefined,
		mtu = 1280,
		errors = {},
		hints = AWG_PARAM_HINTS,
		signatureModes = 'both',
		idPrefix = '',
		compact = false,
	}: {
		params: ASCParams;
		extended?: boolean;
		mtu?: number;
		errors?: ASCErrorFields;
		hints?: Record<string, string>;
		signatureModes?: SignatureModes;
		idPrefix?: string;
		compact?: boolean;
	} = $props();

	const showExtended = $derived(extended ?? isExtendedASCParams(params));

	let selectedProtocol = $state<ProtocolKey>('quic_initial');
	let generateMode = $state<GenerateMode>('protocol');
	let domainInput = $state('');
	let capturing = $state(false);
	let captureError = $state('');
	let captureSource = $state('');

	let totalBytes = $derived.by(() => {
		if (!showExtended) return 0;
		const ext = params as ASCParamsExtended;
		return (
			calcByteSize(String(ext.i1 || '')) +
			calcByteSize(String(ext.i2 || '')) +
			calcByteSize(String(ext.i3 || '')) +
			calcByteSize(String(ext.i4 || '')) +
			calcByteSize(String(ext.i5 || ''))
		);
	});

	let overLimit = $derived(totalBytes > MAX_SIGNATURE_BYTES);

	function fieldId(name: string): string {
		return `${idPrefix}${name}`;
	}

	function handleGenerate() {
		if (!showExtended) return;
		const ext = params as ASCParamsExtended;
		const packets = getSignaturePackets(selectedProtocol, mtu);
		const size =
			calcByteSize(packets.i1) +
			calcByteSize(packets.i2) +
			calcByteSize(packets.i3) +
			calcByteSize(packets.i4) +
			calcByteSize(packets.i5);
		if (size > MAX_SIGNATURE_BYTES) return;
		ext.i1 = packets.i1;
		ext.i2 = packets.i2;
		ext.i3 = packets.i3;
		ext.i4 = packets.i4;
		ext.i5 = packets.i5;
	}

	async function handleCapture() {
		if (!domainInput.trim() || !showExtended) return;
		const ext = params as ASCParamsExtended;
		capturing = true;
		captureError = '';
		captureSource = '';
		try {
			const result = await api.captureSignature(domainInput.trim());
			ext.i1 = result.packets.i1 || '';
			ext.i2 = result.packets.i2 || '';
			ext.i3 = result.packets.i3 || '';
			ext.i4 = result.packets.i4 || '';
			ext.i5 = result.packets.i5 || '';
			captureSource = result.source;
			if (result.warning) {
				captureError = result.warning;
			}
		} catch (e: unknown) {
			captureError = e instanceof Error ? e.message : 'Ошибка захвата';
		} finally {
			capturing = false;
		}
	}
</script>

{#snippet paramLabel(id: string, name: string)}
	<label class="field-label param-field-label" for={fieldId(id)}>
		{name}
		{#if hints[id]}
			<FieldHint text={hints[id]} ariaLabel={`Подсказка: ${name}`} />
		{/if}
	</label>
{/snippet}

<div class="asc-editor" class:compact>
	<section class="param-section">
		<h3 class="section-title">Junk пакеты</h3>
		<p class="group-desc">Фейковые пакеты перед handshake — ломают анализ трафика DPI</p>
		<div class="inline-row inline-row-3">
			{@render paramLabel('jc', 'Jc')}
			<input type="number" id={fieldId('jc')} class="field-input" bind:value={params.jc} />
			{@render paramLabel('jmin', 'Jmin')}
			<input type="number" id={fieldId('jmin')} class="field-input" bind:value={params.jmin} />
			{@render paramLabel('jmax', 'Jmax')}
			<input type="number" id={fieldId('jmax')} class="field-input" bind:value={params.jmax} />
		</div>
	</section>

	<section class="param-section">
		<h3 class="section-title">{showExtended ? 'Padding (S1-S4)' : 'Padding (S1-S2)'}</h3>
		<p class="group-desc">Дополнительные байты в handshake — меняют размер пакетов WireGuard</p>
		<div class="inline-row inline-row-2">
			{@render paramLabel('s1', 'S1')}
			<input type="number" id={fieldId('s1')} class="field-input" bind:value={params.s1} />
			{@render paramLabel('s2', 'S2')}
			<input type="number" id={fieldId('s2')} class="field-input" bind:value={params.s2} />
			{#if showExtended}
				{@const ext = params as ASCParamsExtended}
				{@render paramLabel('s3', 'S3')}
				<input type="number" id={fieldId('s3')} class="field-input" bind:value={ext.s3} />
				{@render paramLabel('s4', 'S4')}
				<input type="number" id={fieldId('s4')} class="field-input" bind:value={ext.s4} />
			{/if}
		</div>
	</section>

	<section class="param-section">
		<h3 class="section-title">Заголовки (H1-H4)</h3>
		<p class="group-desc">Подмена типов пакетов WireGuard на произвольные значения</p>
		<div class="inline-row inline-row-2">
			{@render paramLabel('h1', 'H1')}
			<input type="text" id={fieldId('h1')} class="field-input" bind:value={params.h1} />
			{@render paramLabel('h2', 'H2')}
			<input type="text" id={fieldId('h2')} class="field-input" bind:value={params.h2} />
			{@render paramLabel('h3', 'H3')}
			<input type="text" id={fieldId('h3')} class="field-input" bind:value={params.h3} />
			{@render paramLabel('h4', 'H4')}
			<input type="text" id={fieldId('h4')} class="field-input" bind:value={params.h4} />
		</div>
	</section>

	{#if showExtended}
		{@const ext = params as ASCParamsExtended}
		<section class="param-section">
			<h3 class="section-title">Signature пакеты (I1-I5)</h3>
			<p class="group-desc">Имитация протоколов — DPI видит знакомый трафик вместо WireGuard</p>

			{#if signatureModes === 'both'}
				<div class="mode-options">
					<label class="mode-option">
						<input type="radio" value="protocol" bind:group={generateMode} />
						<span>Протокол</span>
					</label>
					<label class="mode-option">
						<input type="radio" value="domain" bind:group={generateMode} />
						<span>По домену</span>
					</label>
				</div>
			{/if}

			{#if signatureModes === 'domain' || generateMode === 'domain'}
				<div class="generate-row">
					<input
						type="text"
						class="field-input"
						bind:value={domainInput}
						placeholder="example.com"
						disabled={capturing}
						onkeydown={(e) => {
							if (e.key === 'Enter') {
								e.preventDefault();
								handleCapture();
							}
						}}
					/>
					<Button
						variant="secondary"
						size="sm"
						onclick={handleCapture}
						disabled={capturing || !domainInput.trim()}
						loading={capturing}
					>
						{capturing ? 'Захват...' : 'Захватить'}
					</Button>
				</div>
				{#if captureError}
					<p class="capture-info" class:capture-warning={!!captureSource}>{captureError}</p>
				{/if}
				{#if captureSource && !captureError}
					<span class="capture-badge">{captureSource.toUpperCase()}</span>
				{/if}
			{:else}
				{@const protocolOpts: DropdownOption<ProtocolKey>[] = Object.entries(protocols).map(
					([key, proto]) => ({
						value: key as ProtocolKey,
						label: proto.name,
						description: proto.description,
					}),
				)}
				<div class="generate-row">
					<div class="protocol-select">
						<Dropdown bind:value={selectedProtocol} options={protocolOpts} fullWidth />
					</div>
					<Button variant="secondary" size="sm" onclick={handleGenerate}>Сгенерировать</Button>
				</div>
			{/if}

			<div class="signature-fields">
				{#each ['i1', 'i2', 'i3', 'i4', 'i5'] as field, idx}
					<div class="form-group">
						{@render paramLabel(field, field.toUpperCase())}
						<input
							type="text"
							id={fieldId(field)}
							class="field-input"
							bind:value={ext[field as keyof ASCParamsExtended]}
							placeholder={field.toUpperCase() + (idx === 0 ? ' (обязательный)' : '')}
						/>
						{#if errors[field as keyof ASCParamsExtended]}
							<p class="field-error">{errors[field as keyof ASCParamsExtended]}</p>
						{/if}
					</div>
				{/each}
			</div>

			<div class="size-indicator" class:over-limit={overLimit}>
				{totalBytes} / {MAX_SIGNATURE_BYTES} байт
				{#if overLimit}
					<span class="size-error">— превышен лимит!</span>
				{/if}
			</div>
		</section>
	{/if}
</div>

<style>
	.asc-editor {
		display: flex;
		flex-direction: column;
		gap: 20px;
	}

	.param-section {
		background: var(--color-bg-secondary);
		border: 1px solid var(--color-border);
		border-radius: var(--radius);
		padding: 16px;
		overflow: visible;
	}

	.section-title {
		font-size: 14px;
		font-weight: 600;
		padding-bottom: 10px;
		border-bottom: 1px solid var(--color-border);
		margin-bottom: 12px;
	}

	.form-group {
		display: flex;
		flex-direction: column;
		gap: 6px;
		margin-bottom: 12px;
	}

	.form-group:last-child {
		margin-bottom: 0;
	}

	.inline-row {
		display: grid;
		align-items: center;
		gap: 8px;
	}

	.inline-row-2 {
		grid-template-columns: auto 1fr auto 1fr;
	}

	.inline-row-3 {
		grid-template-columns: auto 1fr auto 1fr auto 1fr;
	}

	.param-field-label {
		display: inline-flex;
		align-items: center;
		gap: 0.15rem;
		white-space: nowrap;
	}

	.field-error {
		font-size: 11px;
		color: var(--color-error);
	}

	.group-desc {
		font-size: 11px;
		color: var(--color-text-muted);
		margin: 0 0 10px 0;
		line-height: 1.4;
	}

	.signature-fields {
		display: flex;
		flex-direction: column;
	}

	.mode-options {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem 1rem;
		margin-bottom: 12px;
	}

	.mode-option {
		display: inline-flex;
		align-items: center;
		gap: 0.375rem;
		font-size: 13px;
		color: var(--color-text-primary);
		cursor: pointer;
		white-space: nowrap;
	}

	.mode-option input[type='radio'] {
		accent-color: var(--color-accent);
	}

	.generate-row {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
		margin-bottom: 12px;
	}

	.protocol-select {
		width: 100%;
	}

	.size-indicator {
		font-size: 12px;
		color: var(--color-text-muted);
		margin-top: 4px;
	}

	.size-indicator.over-limit {
		color: var(--color-error);
		font-weight: 500;
	}

	.size-error {
		font-weight: 600;
	}

	.capture-info {
		font-size: 11px;
		color: var(--color-error);
		margin-top: 4px;
	}

	.capture-info.capture-warning {
		color: var(--color-text-muted);
	}

	.capture-badge {
		display: inline-block;
		font-size: 11px;
		font-weight: 600;
		padding: 2px 8px;
		border-radius: var(--radius-sm);
		background: var(--color-bg-tertiary);
		color: var(--color-accent);
		margin-top: 4px;
	}

	@media (max-width: 640px) {
		.inline-row-2,
		.inline-row-3 {
			grid-template-columns: auto 1fr;
		}
	}

	@media (max-width: 480px) {
		.mode-options {
			flex-direction: column;
			gap: 0.5rem;
			align-items: flex-start;
		}
	}
</style>
