<!--
  Источник дизайна: singbox-router/project/parts/RuleCard.jsx (MatcherChip)
  При правках сверять с JSX напрямую — не угадывать spacing/typography/layout.
-->

<script lang="ts" module>
  import type { MatcherKind } from './types';

  /** Локализованная подпись для каждой категории матчера. */
  const LABELS: Record<MatcherKind, string> = {
    domain:   'домен',
    ip:       'IP',
    port:     'порт',
    src:      'источник',
    ruleset:  'набор',
    protocol: 'proto',
    private:  'тип',
  };
</script>

<script lang="ts">
  interface Props {
    kind: MatcherKind;
    label: string;
    /** Если true — значение mono шрифтом (для IP/port/cidr). Подпись слева всегда sans. */
    mono?: boolean;
    /** Клик по чипу — открыть связанный редактор */
    onclick?: () => void;
    /** Подсказка для кликабельного чипа */
    title?: string;
  }
  let { kind, label, mono = false, onclick, title }: Props = $props();

  const isClickable = $derived(typeof onclick === 'function');
</script>

{#if isClickable}
  <button type="button" class="chip is-clickable" {title} aria-label={title} {onclick}>
    <span class="chip-key">{LABELS[kind]}:</span>
    <span class="chip-val" class:is-mono={mono}>{label}</span>
  </button>
{:else}
  <span class="chip">
    <span class="chip-key">{LABELS[kind]}:</span>
    <span class="chip-val" class:is-mono={mono}>{label}</span>
  </span>
{/if}

<style>
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-size: 11px;
    padding: 2px 7px;
    border-radius: 4px;
    background: var(--bg-tertiary);
    border: 1px solid var(--border);
    color: var(--text-secondary);
    white-space: nowrap;
    line-height: 1.4;
    max-width: 100%;
    min-width: 0;
    overflow: hidden;
  }
  button.chip {
    margin: 0;
    cursor: pointer;
    transition:
      border-color var(--t-fast),
      background var(--t-fast),
      color var(--t-fast);
  }
  button.chip:hover {
    border-color: var(--border-hover);
    background: color-mix(in srgb, var(--accent) 8%, var(--bg-tertiary));
    color: var(--text-primary);
  }
  button.chip:focus-visible {
    outline: 2px solid var(--accent);
    outline-offset: 1px;
  }
  .chip-key {
    font-size: 10px;
    color: var(--text-muted);
    font-family: var(--font-sans);
    flex-shrink: 0;
  }
  .chip-val {
    color: var(--text-primary);
    font-family: var(--font-sans);
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .chip-val.is-mono {
    font-family: var(--font-mono);
  }
</style>
