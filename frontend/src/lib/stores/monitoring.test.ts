import { beforeEach, describe, expect, it } from 'vitest';
import { clearHistoryCache, getCachedHistory, setCachedHistory } from './monitoring';
import type { MonitoringSample } from '$lib/types';

function sample(latencyMs: number): MonitoringSample[] {
	return [
		{
			ts: new Date(2026, 5, 14, 12, 0, 0).toISOString(),
			latencyMs,
			ok: true,
		},
	];
}

describe('monitoring history cache', () => {
	beforeEach(() => {
		clearHistoryCache();
	});

	it('scopes cache by target, tunnel, and limit', () => {
		setCachedHistory('target-1', 'tunnel-1', sample(60), 60);
		setCachedHistory('target-1', 'tunnel-1', sample(1440), 1440);

		expect(getCachedHistory('target-1', 'tunnel-1', 60)).toEqual(sample(60));
		expect(getCachedHistory('target-1', 'tunnel-1', 1440)).toEqual(sample(1440));
		expect(getCachedHistory('target-1', 'tunnel-1', 60)).not.toEqual(
			getCachedHistory('target-1', 'tunnel-1', 1440),
		);
	});

	it('clearHistoryCache removes all cached windows', () => {
		setCachedHistory('target-1', 'tunnel-1', sample(60), 60);
		setCachedHistory('target-1', 'tunnel-1', sample(1440), 1440);

		clearHistoryCache();

		expect(getCachedHistory('target-1', 'tunnel-1', 60)).toBeNull();
		expect(getCachedHistory('target-1', 'tunnel-1', 1440)).toBeNull();
	});
});
