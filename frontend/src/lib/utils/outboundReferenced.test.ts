import { describe, expect, it, vi, beforeEach } from 'vitest';
import {
	isOutboundReferencedError,
	OUTBOUND_REFERENCED_CODE,
	showOutboundReferencedError,
} from './outboundReferenced';
import { outboundReferenced } from '$lib/stores/outboundReferenced';
import { get } from 'svelte/store';

describe('outboundReferenced utils', () => {
	beforeEach(() => {
		outboundReferenced.close();
	});

	it('detects structured referenced error', () => {
		const err = new Error(OUTBOUND_REFERENCED_CODE) as Error & {
			details: { tunnelId: string; deviceProxy: boolean; routerRules: number[]; routerOther: string[] };
		};
		err.details = {
			tunnelId: 'x',
			deviceProxy: false,
			routerRules: [0],
			routerOther: ['route.final'],
		};
		expect(isOutboundReferencedError(err)).toBe(true);
	});

	it('opens modal via showOutboundReferencedError', () => {
		const err = new Error(OUTBOUND_REFERENCED_CODE) as Error & {
			details: { tunnelId: string; deviceProxy: boolean; routerRules: null; routerOther: string[] };
		};
		err.details = {
			tunnelId: 'vpn-1',
			deviceProxy: false,
			routerRules: null,
			routerOther: [`outbounds[0="grp"].outbounds[0]`],
		};
		expect(showOutboundReferencedError(err, 'vpn-1', 'Туннель')).toBe(true);
		expect(get(outboundReferenced)?.name).toBe('vpn-1');
		expect(get(outboundReferenced)?.entityLabel).toBe('Туннель');
	});
});
