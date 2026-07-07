import { describe, expect, it } from 'vitest';
import type { MonitoringSample } from '$lib/types';
import {
	computeMonitoringStats,
	downsampleMonitoringPoints,
	getMonitoringRecentPage,
} from './matrixDrillDownLogic';

function makeSamples(count: number): MonitoringSample[] {
	return Array.from({ length: count }, (_, i) => ({
		ts: new Date(2026, 5, 14, 12, i, 0).toISOString(),
		latencyMs: 50 + i,
		ok: true,
	}));
}

describe('matrixDrillDownLogic', () => {
	it('computes avg/min/max/loss honestly', () => {
		expect(
			computeMonitoringStats([
				{ ts: '1', latencyMs: 100, ok: true },
				{ ts: '2', latencyMs: 200, ok: true },
				{ ts: '3', latencyMs: null, ok: false },
			]),
		).toEqual({
			avg: 150,
			min: 100,
			max: 200,
			lossPct: 33,
		});
	});

	it('downsamples retained history for sparkline only', () => {
		const samples = makeSamples(12);
		const reduced = downsampleMonitoringPoints(samples, 5);
		expect(reduced).toHaveLength(5);
		expect(reduced[0]?.latencyMs).toBe(50);
		expect(reduced[4]?.latencyMs).toBeGreaterThanOrEqual(58);
	});

	it('paginates newest-first history honestly', () => {
		const page0 = getMonitoringRecentPage({
			samples: makeSamples(12),
			recentPage: 0,
			rowsPerPage: 10,
		});
		expect(page0.recentRangeLabel).toBe('1-10');
		expect(page0.recent).toHaveLength(10);
		expect(page0.recent[0]?.latencyMs).toBe(61);

		const page1 = getMonitoringRecentPage({
			samples: makeSamples(12),
			recentPage: 1,
			rowsPerPage: 10,
		});
		expect(page1.recentRangeLabel).toBe('11-12');
		expect(page1.recent).toHaveLength(2);
		expect(page1.recentPageCount).toBe(2);
	});
});
