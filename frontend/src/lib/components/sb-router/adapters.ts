/**
 * Adapters singbox routing rule → RuleCardData.
 *
 * Источник дизайна: singbox-router/project/parts/RuleCard.jsx
 * Маппинг action: дизайн использует action='reject' для блокировки,
 * 'direct' (через outbound name) для пропуска без туннеля, 'route' default.
 *
 * resolveOutboundDisplay учитывает 3 спец-случая: direct/block/reject
 * (всегда нормализуются), composite (selector/urltest type), tunnel
 * (всё остальное где outbound найден в списке).
 */

import type { CatalogPreset, SingboxRouterPreset, SingboxRouterRule, SingboxRouterRuleSet, SingboxRouterOutbound, Subscription } from '$lib/types';
import type { OutboundGroup } from '$lib/components/routing/singboxRouter/outboundOptions';
import type {
  MatcherChip,
  OutboundDisplay,
  RuleAction,
  RuleCardData,
} from './types';
import { detectService } from './serviceDetection';
import { resolveRuleSetDisplayType } from '$lib/utils/ruleSetType';
import { COMPOSITE_OUTBOUND_TYPES, resolveCompositeMemberDisplay } from './compositeOutboundDisplay';

/* ─── System rule detection ─────────────────────────────────────────── */

export function isSystemRule(rule: SingboxRouterRule): boolean {
  if (rule.action === 'sniff' || rule.action === 'hijack-dns') return true;
  if (rule.ip_is_private && rule.outbound === 'direct') return true;
  return false;
}

/** Пояснение для системных правил — тултип в простом и экспертном режиме. */
export function systemRuleTooltip(rule: SingboxRouterRule): string | undefined {
  if (rule.action === 'sniff') {
    return 'Анализирует протокол и извлекает домен из соединения (SNI, HTTP Host). Нужно, чтобы следующие правила могли маршрутизировать по домену, а не только по IP.';
  }
  if (rule.action === 'hijack-dns') {
    return 'Перехватывает DNS-запросы на порту 53 и обрабатывает их через DNS-модуль sing-box. Без этого DNS-маршрутизация не работает.';
  }
  if (rule.ip_is_private && rule.outbound === 'direct') {
    return 'Трафик в локальные и приватные сети (RFC1918, loopback, link-local) идёт напрямую, минуя VPN. Нужен для доступа к роутеру, NAS и устройствам в LAN.';
  }
  return undefined;
}

/* ─── Action mapping ────────────────────────────────────────────────── */

function mapAction(rule: SingboxRouterRule): RuleAction {
  if (rule.action === 'reject') return 'block';
  if (rule.action === 'sniff') return 'sniff';
  if (rule.action === 'hijack-dns') return 'hijack-dns';
  if (rule.outbound === 'direct') return 'direct';
  return 'route';
}

/* ─── Outbound display ──────────────────────────────────────────────── */

const COMPOSITE_TYPES = COMPOSITE_OUTBOUND_TYPES;
const AWG_OPTION_GROUPS = new Set(['AWG туннели', 'Системные WireGuard']);

function findOutboundOption(
  tag: string,
  outboundOptions: OutboundGroup[] | undefined,
): { label: string; group: string } | null {
  for (const group of outboundOptions ?? []) {
    const item = group.items?.find((x) => x.value === tag);
    if (item) return { label: item.label, group: group.group };
  }
  return null;
}

export function resolveOutboundDisplay(
  name: string | undefined,
  action: RuleAction,
  outbounds: SingboxRouterOutbound[],
  outboundOptions: OutboundGroup[] = [],
  subscriptions: Subscription[] | null = null,
): OutboundDisplay {
  // System actions — render as mono badges instead of destination tile.
  if (action === 'sniff') {
    return { name: name ?? 'sniff', label: 'SNIFF', kind: 'sniff' };
  }
  if (action === 'hijack-dns') {
    return { name: name ?? 'hijack-dns', label: 'HIJACK-DNS', kind: 'hijack-dns' };
  }

  if (action === 'block') {
    return { name: name ?? 'block', label: 'Блок', kind: 'block' };
  }

  if (!name || name === 'direct') {
    return { name: 'direct', label: 'Прямо', kind: 'direct' };
  }
  if (name === 'block' || name === 'reject') {
    return { name, label: 'Блок', kind: 'block' };
  }

  const option = findOutboundOption(name, outboundOptions);
  const ob = outbounds.find((o) => (o as { tag?: string }).tag === name);
  if (option && AWG_OPTION_GROUPS.has(option.group)) {
    return { name, label: option.label, kind: 'awg' };
  }
  if (!ob) {
    const expanded = resolveCompositeMemberDisplay(name, outbounds, outboundOptions, subscriptions);
    if (expanded) {
      return {
        name,
        label: expanded.groupTitle,
        kind: 'composite',
        compositeType: expanded.compositeType,
        memberLabels: expanded.memberLabels,
        memberTitles: expanded.memberTitles,
      };
    }
    return { name, label: option?.label ?? name, kind: option ? 'tunnel' : 'unknown' };
  }
  const obType = (ob as { type?: string }).type ?? '';
  if (COMPOSITE_TYPES.has(obType)) {
    const expanded = resolveCompositeMemberDisplay(name, outbounds, outboundOptions, subscriptions);
    return {
      name,
      label: expanded?.groupTitle ?? option?.label ?? name,
      kind: 'composite',
      compositeType: expanded?.compositeType ?? (obType as OutboundDisplay['compositeType']),
      memberLabels: expanded?.memberLabels,
      memberTitles: expanded?.memberTitles,
    };
  }
  return { name, label: option?.label ?? name, kind: 'tunnel' };
}

/* ─── Matcher chip extraction ───────────────────────────────────────── */

export function extractMatcherChips(
  rule: SingboxRouterRule,
  rulesetLabels: Record<string, string>,
  ruleSets: SingboxRouterRuleSet[] = [],
): MatcherChip[] {
  const chips: MatcherChip[] = [];
  const rulesetTypes = new Map(
    ruleSets.filter((rs) => rs.tag).map((rs) => [rs.tag, resolveRuleSetDisplayType(rs)] as const),
  );

  for (const d of rule.domain_suffix ?? []) {
    chips.push({ kind: 'domain', label: d });
  }
  for (const c of rule.ip_cidr ?? []) {
    chips.push({ kind: 'ip', label: c, mono: true });
  }
  for (const c of rule.source_ip_cidr ?? []) {
    chips.push({ kind: 'src', label: c, mono: true });
  }
  for (const p of rule.port ?? []) {
    chips.push({ kind: 'port', label: String(p), mono: true });
  }
  for (const rs of rule.rule_set ?? []) {
    chips.push({
      kind: 'ruleset',
      label: rulesetLabels[rs] ?? rs,
      rulesetTag: rs,
      rulesetType: rulesetTypes.get(rs),
    });
  }
  if (rule.protocol) {
    chips.push({ kind: 'protocol', label: rule.protocol });
  }
  if (rule.ip_is_private) {
    chips.push({ kind: 'private', label: 'Локальная сеть' });
  }

  return chips;
}

/* ─── Title fallback ────────────────────────────────────────────────── */

function fallbackTitle(
  rule: SingboxRouterRule,
  serviceKey: string,
  index: number,
  displayName?: string,
): string {
  if (displayName) return displayName;
  if (serviceKey !== 'custom') {
    return serviceKey.charAt(0).toUpperCase() + serviceKey.slice(1).replace('_', ' ');
  }
  if (rule.ip_is_private) return 'Локальная сеть';
  if (rule.action === 'sniff') return 'Анализ протокола';
  if (rule.action === 'hijack-dns') return 'Перехват DNS';
  if (rule.domain_suffix?.length) return rule.domain_suffix[0];
  if (rule.ip_cidr?.length) return rule.ip_cidr[0];
  if (rule.rule_set?.length) return rule.rule_set[0];
  return `Правило #${index + 1}`;
}

/* ─── Subtitle (system rules show technical detail per design) ─────── */

function systemSubtitle(rule: SingboxRouterRule): string | undefined {
  if (rule.action === 'sniff') return 'sniff';
  if (rule.action === 'hijack-dns') return 'protocol=dns OR port=53';
  if (rule.ip_is_private) return 'RFC1918 · loopback · link-local · CGNAT';
  return undefined;
}

/* ─── Stable id ─────────────────────────────────────────────────────── */

function ruleId(rule: SingboxRouterRule, index: number): string {
  const sig = [
    rule.domain_suffix?.[0],
    rule.ip_cidr?.[0],
    rule.rule_set?.[0],
    rule.protocol,
    rule.ip_is_private ? 'priv' : null,
  ].filter(Boolean).join('|');
  return `${index}:${rule.outbound ?? 'no-ob'}:${sig || 'empty'}`;
}

/* ─── Main adapter ──────────────────────────────────────────────────── */

export function singboxRuleToCard(
  rule: SingboxRouterRule,
  index: number,
  outbounds: SingboxRouterOutbound[],
  rulesetLabels: Record<string, string>,
  routerPresets: SingboxRouterPreset[] = [],
  outboundOptions: OutboundGroup[] = [],
  catalog: CatalogPreset[] = [],
  ruleSets: SingboxRouterRuleSet[] = [],
  subscriptions: Subscription[] | null = null,
): RuleCardData {
  const detected = detectService(rule, routerPresets, catalog);
  const serviceKey = detected.iconSlug;
  const action = mapAction(rule);
  const outbound = resolveOutboundDisplay(rule.outbound, action, outbounds, outboundOptions, subscriptions);
  const matchers = extractMatcherChips(rule, rulesetLabels, ruleSets);
  const isSystem = isSystemRule(rule);
  const title = fallbackTitle(rule, serviceKey, index, detected.displayName);
  const subtitle = isSystem
    ? systemSubtitle(rule)
    : matchers.length > 4
      ? `${matchers.length} матчеров`
      : undefined;

  return {
    id: ruleId(rule, index),
    serviceKey,
    title,
    subtitle,
    matchers,
    action,
    outbound,
    isSystem,
    tooltip: isSystem ? systemRuleTooltip(rule) : undefined,
  };
}
