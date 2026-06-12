<script lang="ts">
	import { handleJsonEditorKeydown, type CodeTextareaKeyContext } from '$lib/utils/codeTextareaKeys';
	import { tick } from 'svelte';

	interface Props {
		value?: string;
		placeholder?: string;
		disabled?: boolean;
		wrap?: 'pre' | 'pre-wrap';
		highlight: (raw: string) => string;
		/** Tab / Shift+Tab / smart Enter indent for JSON. */
		indentMode?: 'json' | null;
		class?: string;
		textareaRef?: HTMLTextAreaElement | null;
		onscroll?: () => void;
	}

	let {
		value = $bindable(''),
		placeholder = '',
		disabled = false,
		wrap = 'pre-wrap',
		highlight,
		indentMode = null,
		class: className = '',
		textareaRef = $bindable(null),
		onscroll,
	}: Props = $props();

	let back = $state<HTMLDivElement | null>(null);

	/** Cursor restore after programmatic value edits (bind:value resets selection). */
	let restoreSelection: { start: number; end: number } | null = null;
	let composing = $state(false);

	function keyContext(): CodeTextareaKeyContext | null {
		if (!textareaRef) return null;
		const ta = textareaRef;
		return {
			getValue: () => value,
			setValue: (v) => {
				value = v;
			},
			getSelection: () => ({ start: ta.selectionStart, end: ta.selectionEnd }),
			setSelection: (start, end) => {
				restoreSelection = { start, end };
			},
		};
	}

	function handlePlainEnter(e: KeyboardEvent): void {
		if (e.key !== 'Enter' || e.shiftKey || e.ctrlKey || e.metaKey || e.altKey || e.isComposing) {
			return;
		}
		if (!textareaRef) return;

		const start = textareaRef.selectionStart;
		const end = textareaRef.selectionEnd;
		e.preventDefault();
		const pos = start + 1;
		restoreSelection = { start: pos, end: pos };
		value = value.slice(0, start) + '\n' + value.slice(end);
	}

	function onKeydown(e: KeyboardEvent): void {
		if (disabled || composing) return;
		if (indentMode === 'json') {
			const ctx = keyContext();
			if (!ctx) return;
			handleJsonEditorKeydown(e, ctx);
			return;
		}
		handlePlainEnter(e);
	}

	let highlightHtml = $derived(highlight(value));

	function syncGutter(): void {
		if (!textareaRef || !back) return;
		const gw = textareaRef.offsetWidth - textareaRef.clientWidth;
		const gh = textareaRef.offsetHeight - textareaRef.clientHeight;
		back.style.paddingRight = gw > 0 ? `${gw}px` : '';
		back.style.paddingBottom = gh > 0 ? `${gh}px` : '';
	}

	function syncScrollPositions(): void {
		if (!textareaRef || !back) return;
		back.scrollTop = textareaRef.scrollTop;
		back.scrollLeft = textareaRef.scrollLeft;
	}

	function onTextareaScroll(): void {
		// Only mirror scroll offsets — never reflow the highlight layer here (pre-wrap
		// would reflow and make the caret appear to lag behind while scrolling).
		syncScrollPositions();
		onscroll?.();
	}

	function reapplySelectionAndScroll(): void {
		if (!textareaRef || composing || restoreSelection === null) return;
		const { start, end } = restoreSelection;
		restoreSelection = null;
		// Re-apply selection after bind:value so the browser scrolls the caret into view.
		textareaRef.setSelectionRange(start, end);
		syncScrollPositions();
	}

	// Update highlight and restore scroll in the same turn (avoids one-frame desync on input).
	$effect(() => {
		const html = highlightHtml;
		const ta = textareaRef;
		const el = back;
		if (!el || !ta) return;
		const scrollTop = ta.scrollTop;
		const scrollLeft = ta.scrollLeft;
		el.innerHTML = html;
		el.scrollTop = scrollTop;
		el.scrollLeft = scrollLeft;
	});

	$effect(() => {
		value;
		if (restoreSelection === null) return;
		void tick().then(reapplySelectionAndScroll);
	});

	$effect(() => {
		if (!textareaRef || typeof ResizeObserver === 'undefined') return;
		const ro = new ResizeObserver(() => syncGutter());
		syncGutter();
		ro.observe(textareaRef);
		return () => ro.disconnect();
	});
</script>

<div class="shl-stack" class:shl-wrap-pre={wrap === 'pre'} class:shl-wrap-pre-wrap={wrap === 'pre-wrap'}>
	<div class="shl-back" aria-hidden="true" bind:this={back}></div>
	<textarea
		class="shl-ta {className}"
		bind:this={textareaRef}
		bind:value
		rows={1}
		{placeholder}
		{disabled}
		spellcheck="false"
		autocomplete="off"
		autocapitalize="off"
		onkeydown={onKeydown}
		oncompositionstart={() => (composing = true)}
		oncompositionend={() => {
			composing = false;
			syncScrollPositions();
		}}
		onscroll={onTextareaScroll}
		oninput={() => {
			syncGutter();
			syncScrollPositions();
		}}
	></textarea>
</div>

<style>
	.shl-stack {
		position: relative;
		display: grid;
		grid-template: 1fr / 1fr;
		align-items: stretch;
		min-height: 0;
		width: 100%;
		height: 100%;
		font-family: inherit;
		font-size: inherit;
		font-weight: 400;
		font-style: normal;
		line-height: inherit;
		letter-spacing: inherit;
		tab-size: 4;
		-moz-tab-size: 4;
		font-synthesis: none;
	}

	.shl-stack > * {
		grid-area: 1 / 1;
		min-height: 0;
		width: 100%;
		box-sizing: border-box;
	}

	.shl-back {
		margin: 0;
		padding: 0;
		border: none;
		overflow-anchor: none;
		font-family: inherit;
		font-size: inherit;
		font-weight: 400;
		font-style: normal;
		line-height: inherit;
		letter-spacing: inherit;
		tab-size: inherit;
		-moz-tab-size: inherit;
		font-synthesis: none;
		overflow: hidden;
		height: 100%;
		z-index: 0;
		pointer-events: none;
		color: var(--text, var(--color-text-primary));
		background: transparent;
		scrollbar-width: none;
	}

	.shl-back::-webkit-scrollbar {
		display: none;
	}

	.shl-wrap-pre .shl-back,
	.shl-wrap-pre .shl-ta {
		white-space: pre;
		word-break: normal;
	}

	.shl-wrap-pre-wrap .shl-back,
	.shl-wrap-pre-wrap .shl-ta {
		white-space: pre-wrap;
		overflow-wrap: anywhere;
		word-break: normal;
	}

	/* Only color may differ from textarea — weight/style change glyph widths in monospace. */
	.shl-back :global(span) {
		font-family: inherit;
		font-size: inherit;
		font-weight: 400;
		font-style: normal;
		line-height: inherit;
		letter-spacing: inherit;
	}

	.shl-back :global(.hl-json-key) {
		color: var(--hl-json-key, #0284c7);
	}
	.shl-back :global(.hl-json-str) {
		color: var(--hl-json-str, var(--text));
	}
	.shl-back :global(.hl-json-num) {
		color: var(--hl-json-num, #ea580c);
	}
	.shl-back :global(.hl-json-lit) {
		color: var(--hl-json-lit, #9333ea);
	}
	.shl-back :global(.hl-json-punct) {
		color: var(--hl-json-punct, #9333ea);
	}

	.shl-back :global(.hl-rule-prefix) {
		color: var(--hl-rule-prefix, #0284c7);
	}
	.shl-back :global(.hl-rule-comment) {
		color: var(--hl-rule-comment, var(--muted-text, var(--color-text-muted)));
	}
	.shl-back :global(.hl-rule-ip) {
		color: var(--hl-rule-ip, #9333ea);
	}
	.shl-back :global(.hl-rule-url) {
		color: var(--hl-rule-url, #2563eb);
	}
	.shl-back :global(.hl-rule-dot) {
		color: var(--hl-rule-dot, #dc2626);
	}
	.shl-back :global(.hl-rule-wild) {
		color: var(--hl-rule-wild, #9333ea);
	}
	.shl-ta {
		margin: 0;
		padding: 0;
		overflow-anchor: none;
		position: relative;
		z-index: 1;
		height: 100%;
		resize: none;
		overflow: auto;
		scrollbar-gutter: stable;
		-webkit-appearance: none;
		appearance: none;
		font-family: inherit;
		font-size: inherit;
		font-weight: 400;
		font-style: normal;
		line-height: inherit;
		letter-spacing: inherit;
		tab-size: inherit;
		-moz-tab-size: inherit;
		font-synthesis: none;
		border: none;
		border-radius: inherit;
		outline: none;
		background: transparent;
		color: transparent;
		-webkit-text-fill-color: transparent;
		caret-color: var(--text, var(--color-text-primary));
	}

	.shl-ta::placeholder {
		opacity: 1;
		color: var(--muted-text, var(--color-text-muted));
		-webkit-text-fill-color: var(--muted-text, var(--color-text-muted));
	}

	.shl-ta::selection {
		background: color-mix(in srgb, var(--accent, #3b82f6) 38%, transparent);
	}
</style>
