import { describe, expect, it } from 'vitest';

import type { MonitoringTunnel } from '$lib/types';

import { tunnelIsUp } from './status';

function tunnel(overrides: Partial<MonitoringTunnel> = {}): MonitoringTunnel {
	return {
		id: 'tun-1',
		name: 'Tunnel 1',
		ifaceName: 'nwg0',
		pingcheckTarget: '',
		selfTarget: '',
		selfMethod: 'http',
		...overrides
	};
}

describe('tunnelIsUp', () => {
	it('treats any tunnel with a real ok cell as up', () => {
		expect(tunnelIsUp(tunnel(), true)).toBe(true);
	});

	it('treats sing-box subscription with clashDelay as up without real cells', () => {
		expect(
			tunnelIsUp(
				tunnel({
					source: 'singbox',
					subscription: true,
					clashDelay: 329
				}),
				false
			)
		).toBe(true);
	});

	it('does not treat non-subscription sing-box tunnel without ok cells as up', () => {
		expect(
			tunnelIsUp(
				tunnel({
					source: 'singbox',
					subscription: false,
					clashDelay: 329
				}),
				false
			)
		).toBe(false);
	});

	it('does not treat subscription with zero clashDelay as up', () => {
		expect(
			tunnelIsUp(
				tunnel({
					source: 'singbox',
					subscription: true,
					clashDelay: 0
				}),
				false
			)
		).toBe(false);
	});
});
