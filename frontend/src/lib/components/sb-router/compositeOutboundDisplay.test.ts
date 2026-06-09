import { describe, expect, it } from 'vitest';
import type { SingboxRouterOutbound, Subscription } from '$lib/types';
import { resolveCompositeMemberDisplay } from './compositeOutboundDisplay';

const subs: Subscription[] = [
	{
		id: 'sub-demo0001',
		label: 'Provider Demo',
		url: 'https://example/sub',
		isInline: false,
		headers: [],
		refreshHours: 24,
		lastFetched: '',
		selectorTag: 'sub-demo0001',
		inboundTag: 'sub-demo0001-in',
		listenPort: 11001,
		proxyIndex: 1,
		memberTags: ['sub-demo0001-a', 'sub-demo0001-b'],
		members: [
			{ tag: 'sub-demo0001-a', protocol: 'vless', server: 'de01.demo.example', port: 443 },
			{ tag: 'sub-demo0001-b', protocol: 'vless', server: 'nl02.demo.example', port: 443 },
		],
		orphanTags: [],
		activeMember: 'sub-demo0001-a',
		enabled: true,
		mode: 'selector',
	},
];

describe('resolveCompositeMemberDisplay', () => {
	it('expands router selector members', () => {
		const outbounds: SingboxRouterOutbound[] = [
			{
				type: 'selector',
				tag: 'manual-eu',
				outbounds: ['awg-de', 'awg-nl'],
				source: 'router',
			},
		];
		const result = resolveCompositeMemberDisplay('manual-eu', outbounds, [], null);
		expect(result?.compositeType).toBe('selector');
		expect(result?.memberLabels).toEqual(['awg-de', 'awg-nl']);
	});

	it('expands subscription composite via outbounds list', () => {
		const outbounds: SingboxRouterOutbound[] = [
			{
				type: 'selector',
				tag: 'sub-demo0001',
				outbounds: ['sub-demo0001-a', 'sub-demo0001-b'],
				source: 'subscription',
			},
		];
		const result = resolveCompositeMemberDisplay('sub-demo0001', outbounds, [], subs);
		expect(result?.groupTitle).toBe('Provider Demo');
		expect(result?.memberLabels).toEqual(['de01.demo.example', 'nl02.demo.example']);
	});

	it('falls back to subscription memberTags when outbound missing from list', () => {
		const result = resolveCompositeMemberDisplay('sub-demo0001', [], [], subs);
		expect(result?.memberLabels).toEqual(['de01.demo.example', 'nl02.demo.example']);
		expect(result?.compositeType).toBe('selector');
	});
});
