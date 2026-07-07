import type { MonitoringSample } from '$lib/types';

export function downsampleMonitoringPoints(
	points: MonitoringSample[],
	maxPoints: number,
): MonitoringSample[] {
	if (points.length <= maxPoints) return points;
	const step = points.length / maxPoints;
	const sampled: MonitoringSample[] = [];
	for (let i = 0; i < maxPoints; i++) {
		const idx = Math.min(points.length - 1, Math.floor(i * step));
		sampled.push(points[idx]!);
	}
	return sampled;
}

export function computeMonitoringStats(samples: MonitoringSample[]): {
	avg: number | null;
	min: number | null;
	max: number | null;
	lossPct: number;
} {
	const ok = samples.filter((sample) => sample.ok && sample.latencyMs !== null);
	if (ok.length === 0) {
		return { avg: null, min: null, max: null, lossPct: 100 };
	}
	const lats = ok.map((sample) => sample.latencyMs as number);
	const sum = lats.reduce((a, b) => a + b, 0);
	return {
		avg: Math.round(sum / lats.length),
		min: Math.min(...lats),
		max: Math.max(...lats),
		lossPct: Math.round(((samples.length - ok.length) / samples.length) * 100),
	};
}

export function getMonitoringRecentPage(options: {
	samples: MonitoringSample[];
	recentPage: number;
	rowsPerPage: number;
}) {
	const newestFirstSamples = [...options.samples].reverse();
	const recentPageCount = Math.max(1, Math.ceil(options.samples.length / options.rowsPerPage));
	const recentPageStart = options.recentPage * options.rowsPerPage;
	const recentPageEnd = Math.min(options.samples.length, recentPageStart + options.rowsPerPage);
	return {
		recentPageCount,
		recentPageStart,
		recentPageEnd,
		recent: newestFirstSamples.slice(recentPageStart, recentPageEnd),
		recentRangeLabel:
			options.samples.length === 0 ? '0-0' : `${recentPageStart + 1}-${recentPageEnd}`,
	};
}
