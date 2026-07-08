import { describe, expect, it } from 'vitest';
import type { PingLogEntry, SingboxWatchdogLogEntry } from '$lib/types';
import {
	formatWatchdogLatency,
	formatWatchdogState,
	normalizePingLogEntry,
	normalizeSingboxWatchdogLogEntry,
	sortWatchdogEntries,
} from './watchdogHistory';

describe('watchdogHistory', () => {
	it('normalizes NativeWG latency -1 as unavailable', () => {
		const entry: PingLogEntry = {
			timestamp: '2026-07-08T20:42:07Z',
			tunnelId: 'awg-1',
			tunnelName: 'AWG 1',
			success: true,
			latency: -1,
			error: '',
			failCount: 0,
			threshold: 3,
			stateChange: '',
			backend: 'nativewg',
		};

		const normalized = normalizePingLogEntry(entry);
		expect(normalized.latency).toBeUndefined();
		expect(normalized.error).toBe('NDMS');
		expect(formatWatchdogLatency(normalized)).toBe('—');
	});

	it('formats subscription switch labels from memberFrom/memberTo', () => {
		const entry: SingboxWatchdogLogEntry = {
			timestamp: '2026-07-08T20:42:07Z',
			targetId: 'subscription:demo',
			targetName: 'Demo',
			kind: 'subscription',
			checkTag: 'iq0',
			success: false,
			latency: 0,
			error: 'timeout',
			failCount: 2,
			threshold: 3,
			stateChange: 'switch-member',
			memberFrom: 'old-node',
			memberTo: 'new-node',
		};

		const normalized = normalizeSingboxWatchdogLogEntry(entry);
		expect(formatWatchdogState(normalized)).toBe('old-node → new-node');
		expect(normalized.source).toBe('singbox-subscription');
	});

	it('sorts watchdog entries newest-first', () => {
		const sorted = sortWatchdogEntries([
			{ key: 'a', timestamp: '2026-07-08T20:40:00Z', success: true },
			{ key: 'b', timestamp: '2026-07-08T20:42:00Z', success: true },
			{ key: 'c', timestamp: '2026-07-08T20:41:00Z', success: true },
		]);

		expect(sorted.map((entry) => entry.key)).toEqual(['b', 'c', 'a']);
	});
});
