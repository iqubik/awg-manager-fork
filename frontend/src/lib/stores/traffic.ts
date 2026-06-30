/**
 * Traffic history store — SSE-driven rate accumulator.
 *
 * Card-scoped flow:
 * - loadHistory(id) fetches the last hour once on card mount; live SSE
 *   events via feedTraffic append additional points.
 * - getTrafficRates(id) returns the last CARD_WINDOW_POINTS points, forming
 *   the sliding "last hour" window the card chart renders.
 *
 * Modal-scoped flow:
 * - fetchTrafficDetail(id, period) does a one-shot fetch with server-side
 *   aggregate stats. The result is held locally by the modal and
 *   discarded on close. The shared SSE buffer is not touched.
 *
 * Storage cap: MAX_POINTS limits per-tunnel memory. With 10s SSE interval,
 * 6000 points comfortably covers the card's 1h window plus slack for
 * bursts and clock skew.
 */

import { api } from '$lib/api/client';
import type { TrafficPeriod } from '$lib/api/client';

const MAX_POINTS = 6000;

interface Snapshot {
	timestamp: number;
	rxBytes: number;
	txBytes: number;
}

interface TunnelTraffic {
	lastSnapshot: Snapshot | null;
	timestamps: number[];
	rxRates: number[];
	txRates: number[];
}

const history = new Map<string, TunnelTraffic>();

/** Listeners notified on every update */
const listeners = new Set<() => void>();

/** Tracks tunnels that have completed initial loadHistory to avoid duplicate fetches. */
const initialized = new Set<string>();

let notifyScheduled = false;

/** Коалесцирует пачку feedTraffic (N SSE-сообщений/сек) в одно уведомление. */
function notify() {
	if (notifyScheduled) return;
	notifyScheduled = true;
	queueMicrotask(() => {
		notifyScheduled = false;
		for (const fn of listeners) fn();
	});
}

/**
 * Feed new poll data for a tunnel. Called from the SSE tunnel:traffic handler.
 * Calculates rate from delta between snapshots and appends to the rates array.
 * Always appends — period is purely a display window.
 */
export function feedTraffic(tunnelId: string, rxBytes: number, txBytes: number): void {
	const now = Date.now();
	let entry = history.get(tunnelId);

	if (!entry) {
		entry = { lastSnapshot: null, timestamps: [], rxRates: [], txRates: [] };
		history.set(tunnelId, entry);
	}

	const prev = entry.lastSnapshot;
	const snap: Snapshot = { timestamp: now, rxBytes, txBytes };

	if (prev) {
		const dtSec = (now - prev.timestamp) / 1000;
		if (dtSec > 0.5) {
			const dRx = rxBytes - prev.rxBytes;
			const dTx = txBytes - prev.txBytes;

			// Counter reset (tunnel restart) — skip this point
			if (dRx >= 0 && dTx >= 0) {
				entry.timestamps.push(now / 1000);
				entry.rxRates.push(dRx / dtSec);
				entry.txRates.push(dTx / dtSec);
				if (entry.timestamps.length > MAX_POINTS) {
					entry.timestamps = entry.timestamps.slice(-MAX_POINTS);
					entry.rxRates = entry.rxRates.slice(-MAX_POINTS);
					entry.txRates = entry.txRates.slice(-MAX_POINTS);
				}
				notify();
			}
		}
	}

	entry.lastSnapshot = snap;
}

/**
 * Load the last hour of server-side history once per tunnel on card mount.
 * Subsequent updates flow via feedTraffic from the tunnel:traffic SSE event.
 *
 * The card chart shows a live sliding window of the last hour (360 points
 * at 10-sec resolution). Longer history for the detail modal comes from a
 * separate one-shot call — see fetchTrafficDetail.
 */
export async function loadHistory(tunnelId: string): Promise<void> {
	if (initialized.has(tunnelId)) {
		return;
	}
	initialized.add(tunnelId);

	try {
		const resp = await api.getTraffic(tunnelId, '1h');

		if (!initialized.has(tunnelId)) {
			return;
		}

		let entry = history.get(tunnelId);
		if (!entry) {
			entry = { lastSnapshot: null, timestamps: [], rxRates: [], txRates: [] };
			history.set(tunnelId, entry);
		}

		const serverTimestamps = resp.points.map((p) => p.t);
		const serverRx = resp.points.map((p) => p.rx);
		const serverTx = resp.points.map((p) => p.tx);
		entry.timestamps = serverTimestamps;
		entry.rxRates = serverRx;
		entry.txRates = serverTx;

		if (entry.timestamps.length > MAX_POINTS) {
			entry.timestamps = entry.timestamps.slice(-MAX_POINTS);
			entry.rxRates = entry.rxRates.slice(-MAX_POINTS);
			entry.txRates = entry.txRates.slice(-MAX_POINTS);
		}

		notify();
	} catch {
		initialized.delete(tunnelId);
	}
}

/**
 * Fetch history + stats for the detail modal. Returns raw
 * rate points and aggregates without touching the card-scoped SSE buffer.
 * Callers typically invoke this when the modal opens and discard the
 * result when it closes.
 */
export async function fetchTrafficDetail(tunnelId: string, period: TrafficPeriod): Promise<{
	timestamps: number[];
	rxRates: number[];
	txRates: number[];
	stats: {
		points: number;
		peakRate: number;
		avgRx: number;
		avgTx: number;
		currentRx: number;
		currentTx: number;
		volumeRx?: number;
		volumeTx?: number;
	};
}> {
	const resp = await api.getTraffic(tunnelId, period);
	return {
		timestamps: resp.points.map((p) => p.t),
		rxRates: resp.points.map((p) => p.rx),
		txRates: resp.points.map((p) => p.tx),
		stats: resp.stats
	};
}

function sliceRecentAligned(
	entry: TunnelTraffic,
	maxPoints: number,
): { timestamps: number[]; rxRates: number[]; txRates: number[] } {
	const total = Math.min(entry.timestamps.length, entry.rxRates.length, entry.txRates.length);
	if (total === 0) {
		return { timestamps: [], rxRates: [], txRates: [] };
	}
	const start = Math.max(0, total - maxPoints);
	return {
		timestamps: entry.timestamps.slice(start, total),
		rxRates: entry.rxRates.slice(start, total),
		txRates: entry.txRates.slice(start, total),
	};
}

export function getTrafficSessionDetail(tunnelId: string): {
	timestamps: number[];
	rxRates: number[];
	txRates: number[];
	stats: {
		points: number;
		peakRate: number;
		avgRx: number;
		avgTx: number;
		currentRx: number;
		currentTx: number;
		volumeRx?: number;
		volumeTx?: number;
	};
} {
	const entry = history.get(tunnelId);
	if (!entry) {
		return {
			timestamps: [],
			rxRates: [],
			txRates: [],
			stats: {
				points: 0,
				peakRate: 0,
				avgRx: 0,
				avgTx: 0,
				currentRx: 0,
				currentTx: 0,
			},
		};
	}

	const { timestamps, rxRates, txRates } = sliceRecentAligned(entry, MAX_POINTS);
	const points = Math.min(timestamps.length, rxRates.length, txRates.length);
	if (points === 0) {
		return {
			timestamps: [],
			rxRates: [],
			txRates: [],
			stats: {
				points: 0,
				peakRate: 0,
				avgRx: 0,
				avgTx: 0,
				currentRx: 0,
				currentTx: 0,
			},
		};
	}

	let peakRate = 0;
	let sumRx = 0;
	let sumTx = 0;
	let volumeRx = 0;
	let volumeTx = 0;

	for (let i = 0; i < points; i++) {
		const rx = rxRates[i];
		const tx = txRates[i];
		sumRx += rx;
		sumTx += tx;
		if (rx > peakRate) peakRate = rx;
		if (tx > peakRate) peakRate = tx;

		if (i > 0) {
			const dtSec = Math.max(0, timestamps[i] - timestamps[i - 1]);
			volumeRx += rx * dtSec;
			volumeTx += tx * dtSec;
		}
	}

	return {
		timestamps,
		rxRates,
		txRates,
		stats: {
			points,
			peakRate,
			avgRx: sumRx / points,
			avgTx: sumTx / points,
			currentRx: rxRates[points - 1] ?? 0,
			currentTx: txRates[points - 1] ?? 0,
			volumeRx,
			volumeTx,
		},
	};
}

// Card window: last hour of raw 10s samples = 360 points. Live sliding.
const CARD_WINDOW_POINTS = 360;

// Visual downsample target for the card sparkline. 60 points ≈ one
// bucket per minute over the 1h window — keeps ~5 px per node on a
// typical ~300 px card viewport, which is the sweet spot where Bezier
// smoothing reads as a clean curve instead of a comb. Peaks are
// preserved via max-per-bucket; peak-readout at the top of the chart
// stays accurate because max-of-maxes equals max-of-raw.
const CARD_DISPLAY_POINTS = 60;

function downsampleMax(src: number[], target: number): number[] {
	if (src.length <= target) return src;
	const bucket = src.length / target;
	const out = new Array<number>(target);
	for (let i = 0; i < target; i++) {
		const start = Math.floor(i * bucket);
		const end = Math.min(src.length, Math.floor((i + 1) * bucket));
		let m = src[start];
		for (let j = start + 1; j < end; j++) {
			if (src[j] > m) m = src[j];
		}
		out[i] = m;
	}
	return out;
}

export function getTrafficRates(tunnelId: string): { rx: number[]; tx: number[] } {
	const entry = history.get(tunnelId);
	if (!entry) return { rx: [], tx: [] };

	const rxRaw =
		entry.rxRates.length > CARD_WINDOW_POINTS
			? entry.rxRates.slice(-CARD_WINDOW_POINTS)
			: entry.rxRates;
	const txRaw =
		entry.txRates.length > CARD_WINDOW_POINTS
			? entry.txRates.slice(-CARD_WINDOW_POINTS)
			: entry.txRates;

	return {
		rx: downsampleMax(rxRaw, CARD_DISPLAY_POINTS),
		tx: downsampleMax(txRaw, CARD_DISPLAY_POINTS)
	};
}

/** Aligned RX/TX slices for inline sparklines (list rows, compact headers). */
export function getTrafficSparklineSeries(
	tunnelId: string,
	maxPoints = 28,
): { rx: number[]; tx: number[] } {
	const { rx, tx } = getTrafficRates(tunnelId);
	const n = Math.min(rx.length, tx.length);
	if (n === 0) return { rx: [], tx: [] };
	const start = n - Math.min(maxPoints, n);
	return {
		rx: rx.slice(start, n),
		tx: tx.slice(start, n),
	};
}

/** Clear history for a tunnel (e.g. on delete). */
export function clearTraffic(tunnelId: string): void {
	history.delete(tunnelId);
	initialized.delete(tunnelId);
}

/** Subscribe to traffic updates. Returns unsubscribe function. */
export function subscribeTraffic(fn: () => void): () => void {
	listeners.add(fn);
	return () => listeners.delete(fn);
}
