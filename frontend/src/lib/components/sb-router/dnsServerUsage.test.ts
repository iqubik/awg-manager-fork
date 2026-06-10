import { describe, expect, it } from 'vitest';
import { collectDnsServerReferences, dnsServerDeleteBlockReason } from './dnsServerUsage';
import type { SingboxRouterDNSServer } from '$lib/types';

const emptyCtx = {
	rules: [],
	servers: [],
	dnsFinal: '',
};

describe('collectDnsServerReferences', () => {
	it('finds rule, domain_resolver and final references', () => {
		const refs = collectDnsServerReferences({
			tag: 'bootstrap',
			rules: [{ server: 'bootstrap' }],
			servers: [
				{ tag: 'bootstrap', type: 'udp', server: '1.1.1.1' },
				{ tag: 'doh', type: 'https', server: 'dns.example', domain_resolver: { server: 'bootstrap' } },
			],
			dnsFinal: 'bootstrap',
		});
		expect(refs).toEqual(['rule[0]', 'server[doh].domain_resolver', 'final']);
	});
});

describe('dnsServerDeleteBlockReason', () => {
	it('allows unused server', () => {
		const s: SingboxRouterDNSServer = { tag: 'free', type: 'udp', server: '8.8.8.8' };
		expect(dnsServerDeleteBlockReason(s, emptyCtx)).toBeNull();
	});

	it('blocks referenced server', () => {
		const s: SingboxRouterDNSServer = { tag: 'used', type: 'udp', server: '8.8.8.8' };
		expect(
			dnsServerDeleteBlockReason(s, {
				...emptyCtx,
				rules: [{ server: 'used' }],
			}),
		).toMatch(/используется/i);
	});
});
