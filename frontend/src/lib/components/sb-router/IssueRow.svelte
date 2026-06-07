<!--
  Источник дизайна: singbox-router/project/screens/StatusDrawerView.jsx (IssueRow)
  В F3 cta — серый текст-хинт без onclick (F5/F6 заменит на real button).
-->

<script lang="ts">
  import type { IssueTone } from './drawerData';

  interface Props {
    tone: IssueTone;
    text: string;
    /** Серый текст-хинт справа (опц.) — в F3 не кликабелен. */
    ctaHint?: string;
  }
  let { tone, text, ctaHint }: Props = $props();
</script>

<div class="issue-row" class:tone-warning={tone === 'warning'} class:tone-error={tone === 'error'} class:tone-info={tone === 'info'}>
  <svg class="icon" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
    <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
    <line x1="12" y1="9" x2="12" y2="13" />
    <line x1="12" y1="17" x2="12.01" y2="17" />
  </svg>
  <div class="text">{text}</div>
  {#if ctaHint}<span class="cta-hint">{ctaHint}</span>{/if}
</div>

<style>
  .issue-row {
    display: flex;
    align-items: flex-start;
    gap: 10px;
    padding: 10px 12px;
    border-radius: var(--radius-sm);
    border-left: 3px solid transparent;
  }
  .icon {
    margin-top: 2px;
    flex-shrink: 0;
  }
  .text {
    flex: 1;
    font-size: 12px;
    color: var(--text-secondary);
    line-height: 1.5;
    font-family: var(--font-sans);
  }
  .cta-hint {
    font-size: 11.5px;
    color: var(--text-muted);
    font-family: var(--font-sans);
    white-space: nowrap;
    flex-shrink: 0;
  }

  .tone-warning {
    background: color-mix(in srgb, var(--warning) 10%, var(--bg-tertiary));
    border-left-color: var(--warning);
  }
  .tone-warning .icon { color: var(--warning); }

  .tone-error {
    background: color-mix(in srgb, var(--error) 10%, var(--bg-tertiary));
    border-left-color: var(--error);
  }
  .tone-error .icon { color: var(--error); }

  .tone-info {
    background: color-mix(in srgb, var(--info) 10%, var(--bg-tertiary));
    border-left-color: var(--info);
  }
  .tone-info .icon { color: var(--info); }
</style>
