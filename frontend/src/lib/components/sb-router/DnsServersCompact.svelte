<!--
  Источник дизайна: singbox-router/project/screens/MainExpert.jsx (DnsServersCompact)
-->

<script lang="ts">
  import type { SingboxRouterDNSServer, SingboxRouterDNSRule } from '$lib/types';
  import { Badge, Button } from '$lib/components/ui';
  import { ArrowRight, Edit3 } from 'lucide-svelte';

  interface Props {
    servers: SingboxRouterDNSServer[];
    rules: SingboxRouterDNSRule[];
    onEditServer: (tag: string) => void;
    onEditRule: (idx: number) => void;
    onAddRule?: () => void;
    addRuleDisabled?: boolean;
    addRuleTitle?: string;
  }

  let {
    servers, rules, onEditServer, onEditRule, onAddRule, addRuleDisabled = false, addRuleTitle,
  }: Props = $props();

  function subFor(s: SingboxRouterDNSServer): string {
    return `${s.type ?? 'dns'} · ${s.server}`;
  }

  function detourFor(s: SingboxRouterDNSServer): string {
    return s.detour ?? 'direct';
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
        <Badge variant={detourFor(s) === 'direct' ? 'default' : 'accent'} size="sm" mono>
          {detourFor(s)}
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
    <div class="rules">
      {#each rules as r, i (i)}
        <div class="rule-row mobile-row">
          <button
            type="button"
            class="rule-main mobile-row-main"
            title={`Открыть DNS-правило #${i}`}
            aria-label={`Открыть DNS-правило ${i}`}
            onclick={() => onEditRule(i)}
          >
            <span class="rule-matchers">{matcherSummary(r)}</span>
            <ArrowRight size={11} color="var(--text-muted)" />
            <span class="rule-server">{r.server ?? '—'}</span>
          </button>
          <div class="mobile-row-actions">
            <button
              type="button"
              class="route-action-btn"
              title={`Редактировать DNS-правило #${i}`}
              aria-label={`Редактировать DNS-правило ${i}`}
              onclick={() => onEditRule(i)}
            >
              <Edit3 size={12} />
            </button>
          </div>
        </div>
      {/each}
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
  .servers, .rules {
    display: flex;
    flex-direction: column;
  }
  .row {
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
  .row:hover {
    background: var(--bg-tertiary);
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
  }
  .sub {
    font-size: 11px;
    color: var(--text-muted);
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
  .rule-row {
    font-family: var(--font-mono);
    font-size: 11.5px;
    --route-action-color: var(--accent);
    border-bottom: 1px solid rgba(255, 255, 255, 0.04);
    display: grid;
    grid-template-columns: minmax(0, 1fr) 40px;
    align-items: center;
    gap: 8px;
    padding: 8px 14px;
    width: 100%;
    min-width: 0;
  }

  .rule-main {
    width: 100%;
    text-align: left;
    background: transparent;
    border: 0;
    color: inherit;
    font: inherit;
    display: grid;
    grid-template-columns: minmax(0, 1fr) 12px minmax(0, 108px);
    align-items: center;
    gap: 8px;
    min-width: 0;
    cursor: pointer;
    padding: 0;
  }

  .rule-main:hover {
    background: var(--bg-tertiary);
  }
  .rule-matchers {
    min-width: 0;
    color: var(--text-secondary);
    white-space: normal;
    overflow: hidden;
    text-overflow: initial;
    overflow-wrap: anywhere;
    word-break: break-word;
    line-height: 1.25;
    display: -webkit-box;
    line-clamp: 2;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
  }
  .rule-server {
    color: var(--accent);
    justify-self: end;
    min-width: 0;
    max-width: 108px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mobile-row-actions {
    width: 40px;
    min-width: 40px;
    display: flex;
    justify-content: center;
    align-items: center;
    justify-self: end;
  }

  .route-action-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    min-width: 32px;
    height: 18px;
    padding: 0;
    border-radius: 9px;
    border: 1px solid color-mix(in srgb, var(--route-action-color, var(--accent)) 50%, transparent);
    background: color-mix(in srgb, var(--route-action-color, var(--accent)) 8%, transparent);
    color: color-mix(in srgb, var(--route-action-color, var(--accent)) 58%, transparent);
    box-shadow: 0 0 8px color-mix(in srgb, var(--route-action-color, var(--accent)) 18%, transparent);
    cursor: pointer;
    transition:
      color 0.16s ease,
      border-color 0.16s ease,
      background 0.16s ease,
      box-shadow 0.16s ease,
      transform 0.12s ease;
  }

  .route-action-btn :global(svg) {
    width: 13px;
    height: 13px;
    flex-shrink: 0;
  }

  .route-action-btn:hover:not(:disabled) {
    color: var(--route-action-color, var(--accent));
    border-color: color-mix(in srgb, var(--route-action-color, var(--accent)) 80%, transparent);
    background: color-mix(in srgb, var(--route-action-color, var(--accent)) 16%, transparent);
    box-shadow: 0 0 10px color-mix(in srgb, var(--route-action-color, var(--accent)) 34%, transparent);
  }

  .route-action-btn:active:not(:disabled) {
    transform: translateY(1px);
  }

  .route-action-btn:focus-visible {
    outline: 1px solid color-mix(in srgb, var(--route-action-color, var(--accent)) 90%, transparent);
    outline-offset: 2px;
  }

  .route-action-btn:disabled {
    opacity: 0.35;
    cursor: not-allowed;
    box-shadow: none;
  }
  .empty {
    padding: 14px;
    color: var(--text-muted);
    text-align: center;
    font-size: 12px;
  }

  @media (max-width: 720px) {
    .mobile-row {
      display: grid;
      grid-template-columns: minmax(0, 1fr) 40px;
      align-items: center;
      gap: 0.625rem;
      width: 100%;
      min-width: 0;
    }

    .mobile-row-main {
      min-width: 0;
      max-width: 100%;
    }

    .mobile-row-actions {
      flex-direction: column;
      gap: 0.375rem;
      align-self: center;
    }
  }
</style>
