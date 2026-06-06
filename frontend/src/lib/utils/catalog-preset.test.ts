import { describe, expect, it } from 'vitest';
import type { CatalogPreset } from '$lib/types';
import {
	DNS_LARGE_LIST_THRESHOLD,
	DNS_LARGE_LIST_NOTICE,
	applyPresetToggle,
	catalogPresetCardNotice,
	dnsRouteCatalogPresetFilter,
	findCoveringPreset,
	hrNeoCatalogPresetFilter,
	normalizeCatalogSelection,
	presetDnsLargeListRisk,
	resolvePresetDnsEntries,
	resolvePresetManualDomains,
	singboxRouterCatalogPresetFilter,
	splitPresetDnsEntries,
} from './catalog-preset';

const base = {
	id: 'x',
	name: 'X',
	iconSlug: 'x',
	category: 'media',
	origin: 'builtin' as const,
	engines: {},
};

describe('splitPresetDnsEntries', () => {
	it('maps domains and subnets arrays to HR editor fields', () => {
		const p: CatalogPreset = {
			...base,
			engines: {
				dns: {
					domains: ['example.com', 'geoip:ru'],
					subnets: ['91.108.4.0/22', '10.0.0.0/8'],
				},
			},
		};
		expect(splitPresetDnsEntries(p)).toEqual({
			domainLines: ['example.com'],
			cidrLines: ['geoip:ru', '91.108.4.0/22', '10.0.0.0/8'],
		});
	});
});

describe('hrNeoCatalogPresetFilter', () => {
	it('accepts subnet-only presets', () => {
		const p: CatalogPreset = {
			...base,
			engines: { dns: { subnets: ['10.0.0.0/8'] } },
		};
		expect(hrNeoCatalogPresetFilter(p)).toBe(true);
	});

	it('accepts composite parent with empty own DNS via covers', () => {
		const catalog: CatalogPreset[] = [
			{
				...base,
				id: 'meta',
				covers: ['instagram'],
				engines: { singbox: { action: 'tunnel', ruleSets: [] } },
			},
			{
				...base,
				id: 'instagram',
				engines: { dns: { domains: ['instagram.com'] } },
			},
		];
		expect(hrNeoCatalogPresetFilter(catalog[0], catalog)).toBe(true);
		expect(dnsRouteCatalogPresetFilter(catalog[0], catalog)).toBe(true);
	});
});

describe('resolvePresetDnsEntries', () => {
	const catalog: CatalogPreset[] = [
		{
			...base,
			id: 'meta',
			covers: ['instagram', 'whatsapp'],
			engines: {},
		},
		{
			...base,
			id: 'instagram',
			engines: { dns: { domains: ['instagram.com', 'cdninstagram.com'] } },
		},
		{
			...base,
			id: 'whatsapp',
			engines: { dns: { domains: ['whatsapp.com'], subnets: ['91.108.0.0/16'] } },
		},
	];

	it('expands empty parent to union of covered children', () => {
		expect(resolvePresetDnsEntries(catalog[0], catalog)).toEqual({
			domainLines: ['cdninstagram.com', 'instagram.com', 'whatsapp.com'],
			cidrLines: ['91.108.0.0/16'],
		});
	});

	it('merges own domains with covered children', () => {
		const parent: CatalogPreset = {
			...base,
			id: 'bundle',
			covers: ['instagram'],
			engines: { dns: { domains: ['meta.com'] } },
		};
		const cat = [...catalog.slice(1), parent];
		expect(resolvePresetManualDomains(parent, cat)).toEqual([
			'cdninstagram.com',
			'instagram.com',
			'meta.com',
		]);
	});
});

describe('presetDnsLargeListRisk', () => {
	it('does not flag subscription-only lists', () => {
		const p: CatalogPreset = {
			...base,
			engines: { dns: { subscriptionUrl: 'https://example.com/list.txt' } },
		};
		expect(presetDnsLargeListRisk(p)).toBe(false);
	});

	it(`flags inline lists above ${DNS_LARGE_LIST_THRESHOLD}`, () => {
		const domains = Array.from({ length: DNS_LARGE_LIST_THRESHOLD + 1 }, (_, i) => `d${i}.com`);
		const p: CatalogPreset = { ...base, engines: { dns: { domains } } };
		expect(presetDnsLargeListRisk(p)).toBe(true);
	});

	it('ignores small inline lists', () => {
		const p: CatalogPreset = { ...base, engines: { dns: { domains: ['a.com'] } } };
		expect(presetDnsLargeListRisk(p)).toBe(false);
	});
});

describe('catalogPresetCardNotice', () => {
	it('includes large-list notice for NDMS picker', () => {
		const p: CatalogPreset = {
			...base,
			notice: 'Sensitive',
			engines: {
				dns: {
					domains: Array.from(
						{ length: DNS_LARGE_LIST_THRESHOLD + 1 },
						(_, i) => `x${i}.com`,
					),
				},
			},
		};
		const text = catalogPresetCardNotice(p, true);
		expect(text).toContain(DNS_LARGE_LIST_NOTICE);
		expect(text).toContain('Sensitive');
	});

	it('omits large-list notice when disabled (sing-box catalog)', () => {
		const p: CatalogPreset = {
			...base,
			engines: { dns: { domains: Array.from({ length: 250 }, (_, i) => `x${i}.com`) } },
		};
		expect(catalogPresetCardNotice(p, false)).toBeUndefined();
	});
});

describe('singboxRouterCatalogPresetFilter', () => {
	it('accepts presets with singbox engine', () => {
		const p: CatalogPreset = {
			...base,
			engines: { singbox: { action: 'route', ruleSets: [] } },
		};
		expect(singboxRouterCatalogPresetFilter(p)).toBe(true);
	});

	it('rejects dns-only presets', () => {
		const p: CatalogPreset = {
			...base,
			engines: { dns: { domains: ['a.com'] } },
		};
		expect(singboxRouterCatalogPresetFilter(p)).toBe(false);
	});
});

describe('catalog covers selection', () => {
	const catalog: CatalogPreset[] = [
		{ ...base, id: 'meta', name: 'Meta', covers: ['instagram', 'whatsapp'], engines: {} },
		{ ...base, id: 'instagram', name: 'Instagram', engines: {} },
		{ ...base, id: 'whatsapp', name: 'WhatsApp', engines: {} },
	];

	it('drops covered children when parent is selected', () => {
		const selected = new Set(['instagram', 'whatsapp']);
		const next = applyPresetToggle(selected, 'meta', catalog, true);
		expect([...next]).toEqual(['meta']);
	});

	it('normalizes an existing parent+child selection', () => {
		const next = normalizeCatalogSelection(new Set(['meta', 'instagram']), catalog);
		expect([...next]).toEqual(['meta']);
	});

	it('finds covering preset for a child', () => {
		const parent = findCoveringPreset('instagram', new Set(['meta']), catalog);
		expect(parent?.id).toBe('meta');
	});
});
