<!--
  Источник дизайна: singbox-router/project/screens/RuleDetail.jsx (PathStation)
  Primitive для path-визуализации в result hero: Source → Rule → Outbound.
  Аналог FlowStation но с kicker (uppercase microcopy) + title + mono-sub.
-->

<script lang="ts" module>
  import type { Snippet } from 'svelte';
  export type TracePathTone = 'success' | 'error' | 'muted' | 'accent' | 'info' | 'warning';
</script>

<script lang="ts">
  interface Props {
    icon: Snippet;
    tone: TracePathTone;
    /** Uppercase kicker ("Откуда" / "Правило #N" / "Куда"). */
    kicker: string;
    /** Primary text — IP / rule label / outbound name. */
    title: string;
    /** Optional mono sub (IP details / outbound type). */
    sub?: string;
  }
  let { icon, tone, kicker, title, sub }: Props = $props();
</script>

<div class="station tone-{tone}">
  <div class="icon-row">
    {@render icon()}
    <span class="kicker">{kicker}</span>
  </div>
  <div class="title">{title}</div>
  {#if sub}
    <div class="sub">{sub}</div>
  {/if}
</div>

<style>
  .station {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 12px 14px;
    background: var(--bg-primary);
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    min-width: 0;
  }
  .icon-row {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .station.tone-success .icon-row { color: var(--success); }
  .station.tone-error   .icon-row { color: var(--error); }
  .station.tone-muted   .icon-row { color: var(--text-muted); }
  .station.tone-accent  .icon-row { color: var(--accent); }
  .station.tone-info    .icon-row { color: var(--info); }
  .station.tone-warning .icon-row { color: var(--warning); }

  .kicker {
    font-size: 10px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .title {
    font-size: 14px;
    font-weight: 600;
    color: var(--text-primary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .sub {
    font-size: 11px;
    color: var(--text-muted);
    font-family: var(--font-mono);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
</style>
