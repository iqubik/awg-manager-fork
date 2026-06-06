import type { CatalogPreset } from '$lib/types';

/** Preset with optional composite covers (catalog + sing-box router). */
export type PresetCoverRef = Pick<CatalogPreset, 'id' | 'name' | 'covers'>;

/** Inline DNS entries above this — warn in NDMS / HR Neo pickers. */
export const DNS_LARGE_LIST_THRESHOLD = 300;

/** Shown on catalog tiles with large DNS lists (NDMS / HR Neo only). */
export const DNS_LARGE_LIST_NOTICE =
	'Список содержит много записей и может работать нестабильно — рекомендуется для использования только в sing-box';

export function presetDnsEntryCount(p: CatalogPreset): number {
	const dns = p.engines.dns;
	return (dns?.domains?.length ?? 0) + (dns?.subnets?.length ?? 0);
}

function catalogById(catalog: CatalogPreset[]): Map<string, CatalogPreset> {
	return new Map(catalog.map((p) => [p.id, p]));
}

/** Own inline DNS only (no `covers` expansion). */
export function splitPresetDnsEntries(p: CatalogPreset): {
	domainLines: string[];
	cidrLines: string[];
} {
	const dns = p.engines.dns;
	const domainLines: string[] = [];
	const cidrLines: string[] = [];

	for (const e of dns?.domains ?? []) {
		if (e.startsWith('geoip:') || /^[\d.:a-fA-F]+\/\d+$/.test(e)) cidrLines.push(e);
		else domainLines.push(e);
	}
	for (const e of dns?.subnets ?? []) {
		cidrLines.push(e);
	}

	return { domainLines, cidrLines };
}

/**
 * Effective DNS for apply: own entries plus all covered children (recursive).
 * Parent with empty inline DNS but `covers` resolves to the union of children.
 */
export function resolvePresetDnsEntries(
	preset: CatalogPreset,
	catalog: CatalogPreset[],
	stack: Set<string> = new Set(),
): { domainLines: string[]; cidrLines: string[] } {
	if (stack.has(preset.id)) {
		return { domainLines: [], cidrLines: [] };
	}
	const nextStack = new Set(stack);
	nextStack.add(preset.id);

	const domainSet = new Set<string>();
	const cidrSet = new Set<string>();

	const own = splitPresetDnsEntries(preset);
	for (const d of own.domainLines) domainSet.add(d);
	for (const c of own.cidrLines) cidrSet.add(c);

	if (preset.covers?.length) {
		const byId = catalogById(catalog);
		for (const childId of preset.covers) {
			const child = byId.get(childId);
			if (!child) continue;
			const childDns = resolvePresetDnsEntries(child, catalog, nextStack);
			for (const d of childDns.domainLines) domainSet.add(d);
			for (const c of childDns.cidrLines) cidrSet.add(c);
		}
	}

	return {
		domainLines: [...domainSet].sort(),
		cidrLines: [...cidrSet].sort(),
	};
}

export function resolvePresetManualDomains(
	preset: CatalogPreset,
	catalog: CatalogPreset[],
): string[] {
	const { domainLines, cidrLines } = resolvePresetDnsEntries(preset, catalog);
	return [...domainLines, ...cidrLines];
}

export function resolvedPresetDnsEntryCount(
	preset: CatalogPreset,
	catalog: CatalogPreset[],
): number {
	const { domainLines, cidrLines } = resolvePresetDnsEntries(preset, catalog);
	return domainLines.length + cidrLines.length;
}

/** NDMS / HR: inline or subscription DNS, or non-empty resolved list via `covers`. */
export function isDnsApplicablePreset(p: CatalogPreset, catalog: CatalogPreset[]): boolean {
	if (p.engines.dns?.subscriptionUrl) return true;
	return resolvedPresetDnsEntryCount(p, catalog) > 0;
}

/** Large DNS list risk for NDMS/HR: >300 inline domain/CIDR entries (not subscription-only). */
export function presetDnsLargeListRisk(
	p: CatalogPreset,
	catalog: CatalogPreset[] = [],
): boolean {
	if (p.engines.dns?.subscriptionUrl) return false;
	const count =
		catalog.length > 0 ? resolvedPresetDnsEntryCount(p, catalog) : presetDnsEntryCount(p);
	return count > DNS_LARGE_LIST_THRESHOLD;
}

/** Tooltip / footer text for a catalog card (builtin notice + optional large-list warn). */
export function catalogPresetCardNotice(
	p: CatalogPreset,
	warnLargeDnsLists: boolean,
	catalog: CatalogPreset[] = [],
): string | undefined {
	const parts: string[] = [];
	if (warnLargeDnsLists && presetDnsLargeListRisk(p, catalog)) {
		parts.push(DNS_LARGE_LIST_NOTICE);
	}
	if (p.notice?.trim()) parts.push(p.notice.trim());
	return parts.length > 0 ? parts.join('\n\n') : undefined;
}

/** Remove child ids when a composite parent is already selected. */
export function normalizeCatalogSelection(
	selected: Set<string>,
	catalog: PresetCoverRef[],
): Set<string> {
	const next = new Set(selected);
	for (const p of catalog) {
		if (!next.has(p.id) || !p.covers?.length) continue;
		for (const id of p.covers) next.delete(id);
	}
	return next;
}

/** Parent preset id that currently covers `presetId`, if any. */
export function findCoveringPreset(
	presetId: string,
	selected: Set<string>,
	catalog: PresetCoverRef[],
): PresetCoverRef | undefined {
	for (const p of catalog) {
		if (selected.has(p.id) && p.covers?.includes(presetId)) return p;
	}
	return undefined;
}

/** Toggle one preset; selecting a parent drops covered children from the set. */
export function applyPresetToggle(
	selected: Set<string>,
	presetId: string,
	catalog: PresetCoverRef[],
	multiple: boolean,
): Set<string> {
	if (!multiple) {
		return selected.has(presetId) ? new Set() : new Set([presetId]);
	}
	const next = new Set(selected);
	if (next.has(presetId)) {
		next.delete(presetId);
		return next;
	}
	next.add(presetId);
	const preset = catalog.find((p) => p.id === presetId);
	for (const id of preset?.covers ?? []) next.delete(id);
	return next;
}

/** HR Neo: resolved inline domain/CIDR (no subscription-only lists). */
export function hrNeoCatalogPresetFilter(
	p: CatalogPreset,
	catalog: CatalogPreset[] = [],
): boolean {
	if (p.engines.dns?.subscriptionUrl) return false;
	if (catalog.length === 0) return presetDnsEntryCount(p) > 0;
	return resolvedPresetDnsEntryCount(p, catalog) > 0;
}

export function dnsRouteCatalogPresetFilter(
	p: CatalogPreset,
	catalog: CatalogPreset[] = [],
): boolean {
	if (catalog.length === 0) return !!p.engines.dns;
	return isDnsApplicablePreset(p, catalog);
}

/** sing-box router: presets with a singbox engine (same set as ListPresets). */
export function singboxRouterCatalogPresetFilter(p: CatalogPreset): boolean {
	return !!p.engines.singbox;
}
