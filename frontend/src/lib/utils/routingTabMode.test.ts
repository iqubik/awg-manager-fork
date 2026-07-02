import { describe, expect, it } from 'vitest';
import { isFakeIPTabMuted, isTProxyTabMuted, resolveRoutingMode } from './routingTabMode';

describe('routingTabMode', () => {
	it('falls back to tproxy for missing or unknown mode', () => {
		expect(resolveRoutingMode(undefined)).toBe('tproxy');
		expect(resolveRoutingMode(null)).toBe('tproxy');
		expect(resolveRoutingMode('legacy')).toBe('tproxy');
	});

	it('mutes FakeIP when tproxy is active', () => {
		expect(isFakeIPTabMuted('tproxy')).toBe(true);
		expect(isTProxyTabMuted('tproxy')).toBe(false);
	});

	it('mutes TProxy when fakeip-tun is active', () => {
		expect(isTProxyTabMuted('fakeip-tun')).toBe(true);
		expect(isFakeIPTabMuted('fakeip-tun')).toBe(false);
	});
});
