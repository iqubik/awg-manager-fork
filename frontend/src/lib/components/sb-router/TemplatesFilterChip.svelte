<!--
  Источник дизайна: singbox-router/project/parts/RuleSetModal.jsx (FilterChip)
  При правках сверять с JSX напрямую.
-->

<script lang="ts">
  import type { FilterKey } from './templatesData';

  interface Props {
    label: string;
    value: FilterKey;
    active: boolean;
    count: number;
    onclick: () => void;
  }

  let { label, value, active, count, onclick }: Props = $props();
</script>

<button type="button" class="chip" class:active aria-pressed={active} {onclick} data-value={value}>
  <span class="label">{label}</span>
  <span class="count">{count}</span>
</button>

<style>
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    padding: 4px 10px;
    border-radius: 999px;
    background: transparent;
    border: 1px solid var(--border);
    color: var(--text-secondary);
    font-size: 11.5px;
    font-weight: 500;
    cursor: pointer;
    font-family: inherit;
  }
  .chip.active {
    background: var(--accent-soft);
    border-color: var(--accent-line);
    color: var(--accent);
    font-weight: 600;
  }
  .count {
    color: var(--text-muted);
    font-family: var(--font-mono);
    font-size: 10px;
  }
  .chip.active .count {
    color: var(--accent);
  }
</style>
