import { writable } from 'svelte/store';
import type { MonitoringSnapshot, MonitoringSample } from '$lib/types';
import { DEFAULT_MONITORING_SETTINGS, getMonitoringHistoryCapacity } from '$lib/constants/monitoring';

const DEFAULT_HISTORY_LIMIT = getMonitoringHistoryCapacity(DEFAULT_MONITORING_SETTINGS);

const CACHE_KEY = 'awgm_monitoring_snapshot_v1';

interface MonitoringState {
	snapshot: MonitoringSnapshot | null;
	/** true when showing a cached snapshot that hasn't been confirmed fresh yet */
	stale: boolean;
	loaded: boolean;
	lastUpdatedAt: Date | null;
}

function createMonitoringStore() {
	const { subscribe, update, set } = writable<MonitoringState>({
		snapshot: null,
		stale: false,
		loaded: false,
		lastUpdatedAt: null,
	});

	return {
		subscribe,
		/** Load the last cached snapshot immediately (stale-while-revalidate). */
		loadCached() {
			if (typeof localStorage === 'undefined') return;
			try {
				const raw = localStorage.getItem(CACHE_KEY);
				if (!raw) return;
				const snap: MonitoringSnapshot = JSON.parse(raw);
				update((s) => ({
					...s,
					snapshot: snap,
					stale: true,
					loaded: false,
					lastUpdatedAt: snap.updatedAt ? new Date(snap.updatedAt) : null,
				}));
			} catch {
				// ignore corrupt cache
			}
		},
		setSnapshot(snap: MonitoringSnapshot) {
			try {
				localStorage.setItem(CACHE_KEY, JSON.stringify(snap));
			} catch {
				// ignore storage quota errors
			}
			update((s) => ({
				...s,
				snapshot: snap,
				stale: false,
				loaded: true,
				lastUpdatedAt: new Date(),
			}));
		},
		setLoaded(v: boolean) {
			update((s) => ({ ...s, loaded: v }));
		},
		reset() {
			set({ snapshot: null, stale: false, loaded: false, lastUpdatedAt: null });
		},
	};
}

export const monitoringStore = createMonitoringStore();

// History cache scoped to drawer-open lifetime — avoids refetching when the
// user re-opens the same cell quickly. Cleared on full page reload.
const historyCache = new Map<string, MonitoringSample[]>();

function cacheKey(targetId: string, tunnelId: string, limit = DEFAULT_HISTORY_LIMIT): string {
	return `${targetId}|${tunnelId}|${limit}`;
}

export function getCachedHistory(targetId: string, tunnelId: string, limit = DEFAULT_HISTORY_LIMIT): MonitoringSample[] | null {
	return historyCache.get(cacheKey(targetId, tunnelId, limit)) ?? null;
}

export function setCachedHistory(
	targetId: string,
	tunnelId: string,
	samples: MonitoringSample[],
	limit = DEFAULT_HISTORY_LIMIT,
) {
	historyCache.set(cacheKey(targetId, tunnelId, limit), samples);
}

export function clearHistoryCache() {
	historyCache.clear();
}
