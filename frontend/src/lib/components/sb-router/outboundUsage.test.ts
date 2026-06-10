import { describe, expect, it } from 'vitest';
import { collectOutboundReferences, outboundDeleteBlockReason } from './outboundUsage';
import type { SingboxRouterOutbound } from '$lib/types';

const emptyCtx = {
	rules: [],
	routeFinal: 'direct',
	outbounds: [],
	dnsServers: [],
	ruleSets: [],
	deviceProxyOutbounds: [],
};

describe('collectOutboundReferences', () => {
	it('finds route rule and route.final references', () => {
		const refs = collectOutboundReferences({
			tag: 'fast',
			rules: [{ outbound: 'fast' }, { outbound: 'direct' }],
			routeFinal: 'fast',
			outbounds: [],
			dnsServers: [],
			ruleSets: [],
		});
		expect(refs).toEqual(['route.rules[0]', 'route.final']);
	});

	it('finds nested route rule references', () => {
		const refs = collectOutboundReferences({
			tag: 'warp',
			...emptyCtx,
			rules: [{ action: 'hijack-dns', rules: [{ outbound: 'warp' }] }],
		});
		expect(refs).toEqual(['route.rules[0]']);
	});

	it('finds composite member references', () => {
		const refs = collectOutboundReferences({
			tag: 'warp',
			...emptyCtx,
			outbounds: [{ type: 'selector', tag: 'group', outbounds: ['warp', 'awg1'] }],
		});
		expect(refs).toEqual(['outbounds[group].members[0]']);
	});

	it('finds device-proxy selected outbound', () => {
		const refs = collectOutboundReferences({
			tag: 'custom-composite-1',
			...emptyCtx,
			deviceProxyOutbounds: ['direct', 'custom-composite-1'],
		});
		expect(refs).toEqual(['device-proxy']);
	});
});

describe('outboundDeleteBlockReason', () => {
	it('blocks subscription outbounds', () => {
		const ob: SingboxRouterOutbound = { type: 'selector', tag: 'sub-abc', source: 'subscription' };
		expect(outboundDeleteBlockReason(ob, emptyCtx)).toMatch(/Подписк/i);
	});

	it('allows unused router outbound', () => {
		const ob: SingboxRouterOutbound = { type: 'selector', tag: 'fast', source: 'router' };
		expect(outboundDeleteBlockReason(ob, emptyCtx)).toBeNull();
	});
});
