import { describe, it, expect } from 'vitest';
import { groupLogsByTunnel, computeCardStats } from './pingStats';
import type { PingLogEntry, TunnelPingStatus } from '$lib/types';

function log(tunnelId: string, success: boolean, latency: number, ts = '14:00:00'): PingLogEntry {
	return {
		timestamp: ts, tunnelId, tunnelName: 't', success, latency,
		error: '', failCount: 0, threshold: 3, stateChange: '', backend: 'kernel',
	};
}
function status(over: Partial<TunnelPingStatus> = {}): TunnelPingStatus {
	return {
		tunnelId: 'a', tunnelName: 't', enabled: true, backend: 'kernel',
		status: 'alive', method: 'icmp', lastLatency: 0, failCount: 1,
		failThreshold: 3, restartCount: 2, ...over,
	};
}

describe('groupLogsByTunnel', () => {
	it('groups preserving order', () => {
		const m = groupLogsByTunnel([log('a', true, 10), log('b', true, 20), log('a', false, 0)]);
		expect(m.get('a')!.length).toBe(2);
		expect(m.get('b')!.length).toBe(1);
	});
});

describe('computeCardStats', () => {
	it('normal mix: avg over successes, loss as fail %, chronological bars', () => {
		// newest-first input
		const entries = [log('a', true, 50), log('a', false, 0), log('a', true, 100)];
		const s = computeCardStats(entries, status({ failCount: 1, failThreshold: 3, restartCount: 2 }));
		expect(s.avgMs).toBe(75);        // (50+100)/2
		expect(s.minMs).toBe(50);
		expect(s.maxMs).toBe(100);
		expect(s.lossPct).toBe(33);      // 1 fail / 3
		expect(s.failsLabel).toBe('1/3');
		expect(s.restarts).toBe(2);
		// bars chronological (oldest→newest): reverse of newest-first slice
		expect(s.history).toEqual([100, 0, 50]);
		expect(s.logs.length).toBe(3); // newest-first window, untouched
		expect(s.logs[0].latency).toBe(50);
	});

	it('empty window: nulls + 0 loss + empty bars', () => {
		const s = computeCardStats([], status({ failCount: 0 }));
		expect(s.avgMs).toBeNull();
		expect(s.minMs).toBeNull();
		expect(s.maxMs).toBeNull();
		expect(s.lossPct).toBe(0);
		expect(s.history).toEqual([]);
		expect(s.logs).toEqual([]);
	});

	it('all-fail: nulls for latency, 100% loss, zero-bars', () => {
		const entries = [log('a', false, 0), log('a', false, 0)];
		const s = computeCardStats(entries, status());
		expect(s.avgMs).toBeNull();
		expect(s.lossPct).toBe(100);
		expect(s.history).toEqual([0, 0]);
		expect(s.logs).toHaveLength(2);
	});

	it('keeps full log list for navigator while stats window stays limited', () => {
		const entries = Array.from({ length: 40 }, (_, index) =>
			log('a', true, index + 1, `14:00:${String(index).padStart(2, '0')}`),
		);
		const s = computeCardStats(entries, status());
		expect(s.logs).toHaveLength(40);
		expect(s.history).toHaveLength(12);
	});
});
