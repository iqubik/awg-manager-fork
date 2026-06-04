<!--
  Источник дизайна: singbox-router/project/screens/MainExpert.jsx (RoutingTable)
  Reuse adapters.ts из F2 для action + matchers расшифровки.
-->

<script lang="ts">
  import type { SingboxRouterRule, SingboxRouterOutbound } from '$lib/types';
  import type { OutboundGroup } from '$lib/components/routing/singboxRouter/outboundOptions';
  import { Badge } from '$lib/components/ui';
  import { ChevronUp, ChevronDown, Edit3, Trash2 } from 'lucide-svelte';
  import { isSystemRule, systemRuleTooltip } from './adapters';
  import { resolveMemberLabel } from '$lib/utils/memberLabel';

  const AWG_OPTION_GROUPS = new Set(['AWG туннели', 'Системные WireGuard']);

  interface Props {
    rules: SingboxRouterRule[];
    outbounds: SingboxRouterOutbound[];
    outboundOptions?: OutboundGroup[];
    onEdit: (idx: number) => void;
    onDelete: (idx: number) => void;
    onMove: (idx: number, dir: 'up' | 'down') => void;
    bare?: boolean;
  }

  let { rules, outboundOptions = [], onEdit, onDelete, onMove, bare = false }: Props = $props();

  type ActionLabel = 'SNIFF' | 'HIJACK' | 'BYPASS' | 'REJECT' | 'ROUTE';
  type ActionVariant = 'default' | 'accent' | 'success' | 'error' | 'warning' | 'info' | 'muted';

  interface RowData {
    idx: number;
    sys: boolean;
    actionLabel: ActionLabel;
    actionVariant: ActionVariant;
    matchers: string;
    outbound: string;
    outboundLabel: string;
    outboundVariant: 'accent' | 'purple';
    outboundKind: 'route' | 'direct' | 'reject' | 'none';
    tooltip?: string;
  }

  function compileMatchers(r: SingboxRouterRule): string {
    const parts: string[] = [];
    if (r.protocol) parts.push(`protocol=${r.protocol}`);
    if (r.domain_suffix?.length) {
      const head = r.domain_suffix[0];
      const rest = r.domain_suffix.length > 1 ? ` +${r.domain_suffix.length - 1}` : '';
      parts.push(`domain: ${head}${rest}`);
    }
    if (r.ip_cidr?.length) {
      const head = r.ip_cidr[0];
      const rest = r.ip_cidr.length > 1 ? ` +${r.ip_cidr.length - 1}` : '';
      parts.push(`ip: ${head}${rest}`);
    }
    if (r.source_ip_cidr?.length) {
      parts.push(`src: ${r.source_ip_cidr[0]}${r.source_ip_cidr.length > 1 ? ` +${r.source_ip_cidr.length - 1}` : ''}`);
    }
    if (r.port?.length) {
      parts.push(`port: ${r.port.join(',')}`);
    }
    if (r.rule_set?.length) {
      parts.push(`set: ${r.rule_set.join(', ')}`);
    }
    if (r.ip_is_private) {
      parts.push('ip_is_private');
    }
    return parts.length > 0 ? parts.join(' · ') : '—';
  }

  function actionDisplay(r: SingboxRouterRule): { label: ActionLabel; variant: ActionVariant } {
    if (r.action === 'sniff') return { label: 'SNIFF', variant: 'default' };
    if (r.action === 'hijack-dns') return { label: 'HIJACK', variant: 'default' };
    if (r.ip_is_private && r.action === 'route' && (!r.outbound || r.outbound === 'direct')) {
      return { label: 'BYPASS', variant: 'default' };
    }
    if (r.action === 'reject') return { label: 'REJECT', variant: 'error' };
    return { label: 'ROUTE', variant: 'success' };
  }

  const rowData = $derived<RowData[]>(
    rules.map((r, idx) => {
      const sys = isSystemRule(r);
      const a = actionDisplay(r);
      const outbound = r.outbound ?? (r.action === 'reject' ? 'reject' : '—');
      const outboundKind: RowData['outboundKind'] = outbound === '—' ? 'none'
        : outbound === 'direct' ? 'direct'
        : outbound === 'reject' ? 'reject' : 'route';
      const outboundLabel = outboundKind === 'route'
        ? resolveMemberLabel(outbound, null, outboundOptions)
        : outbound;
      const outboundVariant = outboundOptions.some((g) =>
        AWG_OPTION_GROUPS.has(g.group) && g.items.some((i) => i.value === outbound)
      ) ? 'purple' : 'accent';
      return {
        idx,
        sys,
        actionLabel: a.label,
        actionVariant: a.variant,
        matchers: compileMatchers(r),
        outbound,
        outboundLabel,
        outboundVariant,
        outboundKind,
        tooltip: sys ? systemRuleTooltip(r) : undefined,
      };
    }),
  );
</script>

<div class="table" class:bare>
  <div class="header">
    <div>#</div>
    <div>Порядок</div>
    <div>Действие</div>
    <div>Условия</div>
    <div>Выход</div>
    <div class="actions-col">Действия</div>
  </div>
  {#each rowData as row (row.idx)}
    <div
      class="row"
      class:sys={row.sys}
      class:route={!row.sys && row.outboundKind === 'route'}
      title={row.tooltip}
    >
      <div class="idx">{row.idx}</div>
      <div class="reorder">
        {#if !row.sys}
          <button
            type="button"
            class="route-reorder-btn"
            title={`Поднять правило #${row.idx}`}
            aria-label={`Поднять правило ${row.idx}`}
            disabled={row.idx === 0}
            onclick={() => onMove(row.idx, 'up')}
          >
            <ChevronUp size={15} />
          </button>
          <button
            type="button"
            class="route-reorder-btn"
            title={`Опустить правило #${row.idx}`}
            aria-label={`Опустить правило ${row.idx}`}
            disabled={row.idx === rules.length - 1}
            onclick={() => onMove(row.idx, 'down')}
          >
            <ChevronDown size={15} />
          </button>
        {/if}
      </div>
      <div class="matchers" title={row.matchers}>
        <span class="mobile-label">Условия</span>
        <span class="matcher-text">{row.matchers}</span>
      </div>
      <div class="badges-line">
        <div class="action-badge-cell">
          <span class="mobile-label">Действие</span>
          <Badge variant={row.actionVariant} size="sm" mono>{row.actionLabel}</Badge>
        </div>
        <div class="outbound-cell">
          <span class="mobile-label">Выход</span>
          {#if row.outboundKind === 'none'}
            <span class="dash">—</span>
          {:else if row.outboundKind === 'direct'}
            <Badge variant="muted" mono size="sm">direct</Badge>
          {:else if row.outboundKind === 'reject'}
            <Badge variant="error" mono size="sm">reject</Badge>
          {:else}
            <Badge variant={row.outboundVariant} mono size="sm" title={row.outbound}>{row.outboundLabel}</Badge>
          {/if}
        </div>
      </div>
      <div class="actions-col actions">
        {#if !row.sys}
          <button
            type="button"
            class="route-action-btn"
            title={`Редактировать правило #${row.idx}`}
            aria-label={`Редактировать правило ${row.idx}`}
            onclick={() => onEdit(row.idx)}
          >
            <Edit3 size={15} />
          </button>
          <button
            type="button"
            class="route-action-btn danger"
            title={`Удалить правило #${row.idx}`}
            aria-label={`Удалить правило ${row.idx}`}
            onclick={() => onDelete(row.idx)}
          >
            <Trash2 size={15} />
          </button>
        {/if}
      </div>
    </div>
  {/each}
  {#if rules.length === 0}
    <div class="empty">Нет правил</div>
  {/if}
</div>

<style>
  .table {
    width: 100%;
    min-width: 0;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    overflow: hidden;
  }
  .header,
  .row {
    display: grid;
    grid-template-columns: 24px 64px 104px minmax(0, 1fr) minmax(72px, 160px) 96px;
    align-items: center;
    gap: 8px;
    padding: 8px 14px;
    width: 100%;
    min-width: 0;
    box-sizing: border-box;
  }
  .header {
    background: var(--bg-tertiary);
    border-bottom: 1px solid var(--border);
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
    font-weight: 500;
  }
  .header > div:nth-child(2),
  .header > div:nth-child(3),
  .row > .reorder,
  .action-badge-cell,
  .header > div:nth-child(4),
  .header > div:nth-child(5),
  .outbound-cell {
    text-align: center;
  }
  .header > div:nth-child(5),
  .outbound-cell {
    min-width: 0;
  }
  .outbound-cell {
    justify-self: center;
  }
  .row > .matchers {
    min-width: 0;
  }
  .row {
    transition: background-color 0.15s ease;
    padding: 6px 14px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.04);
    font-size: 13px;
  }
  .row.sys {
    opacity: 0.6;
  }
  .row.route {
    background: rgba(122, 162, 247, 0.025);
  }

  @media (hover: hover) and (pointer: fine) {
    .row:hover {
      background: color-mix(in srgb, var(--bg-hover) 70%, transparent);
    }

    .row.sys:hover {
      background: color-mix(in srgb, var(--bg-hover) 45%, transparent);
    }
  }
  .idx {
    font-family: var(--font-mono);
    color: var(--text-muted);
    font-size: 12px;
  }
  .reorder {
    display: flex;
    gap: 2px;
    justify-content: center;
  }
  .matchers {
    min-width: 0;
    font-family: var(--font-mono);
    font-size: 11px;
    color: var(--text-muted);
    white-space: normal;
    overflow-wrap: anywhere;
    word-break: break-word;
    line-height: 1.45;
  }
  .mobile-label {
    display: none;
  }
  .matcher-text {
    display: contents;
  }
  .badges-line {
    display: contents;
  }
  .action-badge-cell {
    min-width: 0;
    justify-self: stretch;
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .outbound-cell {
    min-width: 0;
    display: flex;
    justify-content: center;
    align-items: center;
    overflow: hidden;
  }
  .action-badge-cell :global(.badge),
  .outbound-cell :global(.badge) {
    max-width: 100%;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .dash {
    color: var(--text-muted);
  }
  .actions-col {
    text-align: right;
    justify-self: stretch;
    min-width: 0;
  }
  .header > .actions-col {
    display: block;
    width: 100%;
    text-align: right;
    justify-self: stretch;
    padding-right: 0.25rem;
  }
  .actions {
    width: 100%;
    display: flex;
    flex-wrap: nowrap;
    align-items: center;
    justify-content: flex-end;
    gap: 4px;
  }
  .empty {
    padding: 14px;
    color: var(--text-muted);
    text-align: center;
    font-size: 12px;
  }
  @media (min-width: 721px) {
    .action-badge-cell,
    .outbound-cell,
    .row > .actions-col {
      display: flex;
      align-items: center;
      flex-wrap: nowrap;
      min-width: 0;
    }

    .action-badge-cell,
    .outbound-cell {
      justify-content: center;
    }

    .row > .actions-col {
      justify-content: flex-end;
    }

    .header > .actions-col {
      display: block;
      text-align: right;
    }
  }
  @media (max-width: 768px) {
    .table {
      overflow-x: visible;
      border: 0;
      background: transparent;
    }
    .header {
      display: none;
    }
    .row {
      display: grid;
      grid-template-columns: 1.5rem minmax(0, 1fr) 72px;
      grid-template-areas:
        "idx matcher actions"
        "idx badges badges"
        "idx reorder reorder";
      align-items: start;
      gap: 0.5rem 0.6rem;
      padding: 0.75rem 0.875rem;
      margin: 0;
      border: 0;
      border-radius: 0;
      background: transparent;
      border-bottom: 1px solid var(--border);
      min-width: 0;
      position: relative;
    }
    .row:last-child {
      border-bottom: 0;
    }
    .idx {
      grid-area: idx;
      position: static;
      width: auto;
      padding-top: 0.1rem;
      font-size: 11px;
      line-height: 1.2;
      text-align: center;
    }
    .matchers {
      grid-area: matcher;
      min-width: 0;
      padding-right: 0;
      display: block;
      white-space: normal;
      overflow-wrap: anywhere;
      word-break: normal;
      line-height: 1.35;
      text-align: left;
    }
    .matcher-text {
      display: inline;
      font-size: 12px;
      color: var(--text-secondary);
    }
    .actions-col {
      grid-area: actions;
      position: static;
      justify-self: end;
      align-self: start;
      text-align: right;
      min-width: 0;
    }
    .actions {
      width: auto;
      display: flex;
      flex-wrap: nowrap;
      justify-content: flex-end;
      gap: 4px;
    }
    .badges-line {
      grid-area: badges;
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      justify-content: flex-start;
      gap: 0.4rem;
      min-width: 0;
      width: 100%;
    }
    .action-badge-cell,
    .outbound-cell {
      grid-area: auto;
      display: inline-flex;
      flex: 0 1 auto;
      align-items: center;
      justify-content: flex-start;
      gap: 0.35rem;
      min-width: 0;
      max-width: 100%;
      text-align: left;
      justify-self: auto;
      overflow: visible;
    }
    .outbound-cell :global(.badge) {
      max-width: 100%;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .reorder {
      grid-area: reorder;
      display: flex;
      justify-content: flex-start;
      gap: 0.35rem;
      padding-top: 0.35rem;
      border-top: 1px dashed color-mix(in srgb, var(--border) 85%, transparent);
      min-width: 0;
    }
    .reorder::before {
      content: 'Порядок';
      align-self: center;
      margin-right: 0.35rem;
      font-size: 10px;
      font-weight: 600;
      line-height: 1.2;
      color: var(--text-muted);
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }
    .row.sys .reorder {
      display: none;
    }
    .mobile-label {
      display: none;
    }
    .route-reorder-btn {
      width: 32px;
      min-width: 32px;
      min-height: 32px;
      padding: 6px;
    }
    .route-action-btn {
      width: 32px;
      min-width: 32px;
      min-height: 32px;
      padding: 6px;
    }
    .route-reorder-btn :global(svg),
    .route-action-btn :global(svg) {
      width: 14px;
      height: 14px;
    }
  }
  /* Bare mode для embed внутри SidePanel — parent даёт chrome */
  .table.bare {
    width: 100%;
    min-width: 0;
    background: transparent;
    border: 0;
    border-radius: 0;
  }
</style>
