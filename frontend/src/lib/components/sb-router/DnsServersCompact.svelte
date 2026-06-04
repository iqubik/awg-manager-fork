<!--
  Источник дизайна: singbox-router/project/screens/MainExpert.jsx (DnsServersCompact)
-->

<script lang="ts">
  import type { SingboxRouterDNSServer, SingboxRouterDNSRule } from '$lib/types';
  import type { OutboundGroup } from '$lib/components/routing/singboxRouter/outboundOptions';
  import { Badge, Button } from '$lib/components/ui';
  import { ArrowRight, Trash2, Edit3 } from 'lucide-svelte';
  import { resolveMemberLabel } from '$lib/utils/memberLabel';
  import { dnsRuleTarget } from './dnsRuleLabel';

  const AWG_OPTION_GROUPS = new Set(['AWG туннели', 'Системные WireGuard']);

  interface Props {
    servers: SingboxRouterDNSServer[];
    rules: SingboxRouterDNSRule[];
    onEditServer: (tag: string) => void;
    onEditRule: (idx: number) => void;
    onDeleteRule?: (idx: number) => void;
    onAddRule?: () => void;
    addRuleDisabled?: boolean;
    addRuleTitle?: string;
    outboundOptions?: OutboundGroup[];
  }

  let {
    servers, rules, onEditServer, onEditRule, onDeleteRule, onAddRule, addRuleDisabled = false, addRuleTitle,
    outboundOptions = [],
  }: Props = $props();

  function subFor(s: SingboxRouterDNSServer): string {
    return `${s.type ?? 'dns'} · ${s.server}`;
  }

  function detourFor(s: SingboxRouterDNSServer): string {
    return s.detour ?? 'direct';
  }

  function detourLabelFor(s: SingboxRouterDNSServer): string {
    const detour = detourFor(s);
    if (detour === 'direct') return detour;
    return resolveMemberLabel(detour, null, outboundOptions);
  }

  function detourVariantFor(s: SingboxRouterDNSServer): 'default' | 'accent' | 'purple' {
    const detour = detourFor(s);
    if (detour === 'direct') return 'default';
    return outboundOptions.some((g) =>
      AWG_OPTION_GROUPS.has(g.group) && g.items.some((i) => i.value === detour)
    ) ? 'purple' : 'accent';
  }

  function matcherSummary(r: SingboxRouterDNSRule): string {
    const parts: string[] = [];
    if (r.rule_set?.length) parts.push(`rule_set: ${r.rule_set.join(', ')}`);
    if (r.domain_suffix?.length) parts.push(`suffix: ${r.domain_suffix[0]}${r.domain_suffix.length > 1 ? ` +${r.domain_suffix.length - 1}` : ''}`);
    if (r.domain_keyword?.length) parts.push(`keyword: ${r.domain_keyword[0]}`);
    if (r.query_type?.length) parts.push(`query_type=${r.query_type[0]}`);
    return parts.length > 0 ? parts.join(' · ') : '—';
  }
</script>

<div class="wrap">
  <div class="servers">
    {#each servers as s (s.tag)}
      <button type="button" class="row" onclick={() => onEditServer(s.tag)}>
        <span class="dot"></span>
        <div class="meta">
          <div class="tag">{s.tag}</div>
          <div class="sub">{subFor(s)}</div>
        </div>
        <Badge variant={detourVariantFor(s)} size="sm" mono title={detourFor(s)}>
          {detourLabelFor(s)}
        </Badge>
      </button>
    {/each}
    {#if servers.length === 0}
      <div class="empty">Нет серверов</div>
    {/if}
  </div>

  <div class="rules-cap">
    <span class="rules-cap-label">DNS-правила · {rules.length}</span>
    {#if onAddRule}
      <Button variant="primary" size="sm" onclick={onAddRule} disabled={addRuleDisabled}>+ Правило</Button>
    {/if}
  </div>
  {#if rules.length > 0}
    <div class="rules-table">
      <div class="rules-rows">
        {#each rules as r, i (i)}
          {@const tgt = dnsRuleTarget(r)}
          <div class="rule-row">
            <button
              type="button"
              class="rule-content"
              onclick={() => onEditRule(i)}
              title={`${matcherSummary(r)} → ${tgt.label}`}
            >
              <span class="rule-match">{matcherSummary(r)}</span>
              <span class="rule-arrow">→</span>
              <span
                class="rule-server mono"
                class:block={tgt.kind === 'block'}
                class:none={tgt.kind === 'none'}
              >
                {tgt.label}
              </span>
            </button>

            <div class="rule-actions">
              <button
                type="button"
                class="route-action-btn"
                onclick={() => onEditRule(i)}
                aria-label={`Редактировать DNS-правило #${i + 1}`}
                title={`Редактировать DNS-правило #${i + 1}`}
              >
                <Edit3 size={15} />
              </button>

              {#if onDeleteRule}
                <button
                  type="button"
                  class="route-action-btn danger"
                  onclick={() => onDeleteRule(i)}
                  aria-label={`Удалить DNS-правило #${i + 1}`}
                  title={`Удалить DNS-правило #${i + 1}`}
                >
                  <Trash2 size={15} />
                </button>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    </div>
  {:else}
    <div class="rules-empty">нет правил</div>
  {/if}
</div>

<style>
  .wrap {
    display: flex;
    flex-direction: column;
  }
  .servers {
    display: flex;
    flex-direction: column;
  }
  .row {
    transition: background-color 0.15s ease;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 14px;
    background: transparent;
    border: 0;
    border-bottom: 1px solid rgba(255, 255, 255, 0.04);
    cursor: pointer;
    font-family: inherit;
    color: inherit;
    width: 100%;
    text-align: left;
  }
  @media (hover: hover) and (pointer: fine) {
    .row:hover,
    .rule-row:hover {
      background: color-mix(in srgb, var(--bg-hover) 70%, transparent);
    }
  }
  .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--text-muted);
    flex-shrink: 0;
  }
  .meta {
    flex: 1;
    min-width: 0;
  }
  .tag {
    font-family: var(--font-mono);
    font-size: 12px;
    font-weight: 600;
    white-space: normal;
    overflow-wrap: anywhere;
  }
  .sub {
    font-size: 11px;
    color: var(--text-muted);
    white-space: normal;
    overflow-wrap: anywhere;
  }
  .rules-cap {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 8px 14px;
    background: var(--bg-tertiary);
    font-size: 11px;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    font-weight: 600;
  }
  .rules-empty {
    padding: 12px 14px;
    color: var(--text-muted);
    text-align: center;
    font-size: 11.5px;
    font-style: italic;
  }
  .rules-table {
    display: grid;
    min-width: 0;
  }
  .rules-rows {
    display: grid;
    gap: 0.25rem;
    min-width: 0;
    padding-top: 0.25rem;
  }
  .rule-row {
    transition: background-color 0.15s ease;
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    align-items: center;
    column-gap: 0.5rem;
    min-width: 0;
    background: var(--surface-bg);
    padding: 0.55rem 0.75rem;
    border-radius: 4px;
  }
  .rule-content {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 0.35rem;
    background: transparent;
    border: 0;
    padding: 0;
    color: inherit;
    font-family: var(--font-mono);
    text-align: left;
    cursor: pointer;
  }
  .rule-match {
    flex: 1 1 auto;
    min-width: 0;
    color: var(--text);
    font-size: 12px;
    line-height: 1.25;
    white-space: normal;
    overflow-wrap: anywhere;
  }
  .rule-arrow {
    flex: 0 0 auto;
    color: var(--muted-text);
    line-height: 1;
    opacity: 0.85;
    transform: translateY(-0.02em);
  }
  .rule-server {
    flex: 0 1 6.5rem;
    min-width: 0;
    color: var(--accent);
    font-size: 12px;
    line-height: 1.25;
    white-space: normal;
    overflow-wrap: anywhere;
    word-break: normal;
  }
  .rule-server.block {
    color: var(--text-secondary);
    font-weight: 600;
  }
  .rule-server.none {
    color: var(--text-muted);
  }
  .rule-actions {
    display: inline-flex;
    align-items: center;
    justify-content: flex-end;
    gap: 4px;
    flex-shrink: 0;
    white-space: nowrap;
  }
  .empty {
    padding: 14px;
    color: var(--text-muted);
    text-align: center;
    font-size: 12px;
  }

  @media (max-width: 720px) {
    .rule-row {
      grid-template-columns: minmax(0, 1fr) auto;
      align-items: center;
      gap: 0.5rem;
      padding: 0.65rem 0.75rem;
      border: 1px solid var(--border);
    }

    .rule-content {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 0.3rem;
    }

    .rule-match {
      flex: 1 1 100%;
    }

    .rule-arrow {
      flex: 0 0 auto;
    }

    .rule-server {
      flex: 1 1 auto;
    }

    .rule-actions {
      align-self: center;
    }
  }
</style>
