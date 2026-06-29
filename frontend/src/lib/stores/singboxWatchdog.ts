import { api } from '$lib/api/client';
import type { SingboxWatchdogStatus } from '$lib/types';
import { createPollingStore, type PollingStore } from './polling';
import { registerStore } from './storeRegistry';

function normalizeWatchdogStatuses(value: unknown): SingboxWatchdogStatus[] {
	if (Array.isArray(value)) {
		return value as SingboxWatchdogStatus[];
	}
	if (value && typeof value === 'object') {
		const record = value as { items?: unknown; data?: unknown };
		if (Array.isArray(record.items)) {
			return record.items as SingboxWatchdogStatus[];
		}
		if (Array.isArray(record.data)) {
			return record.data as SingboxWatchdogStatus[];
		}
	}
	return [];
}

async function fetchWatchdog(): Promise<SingboxWatchdogStatus[]> {
	return normalizeWatchdogStatuses(await api.singboxWatchdogStatus());
}

export const singboxWatchdogStatus: PollingStore<SingboxWatchdogStatus[]> =
	createPollingStore(fetchWatchdog, { staleTime: 5_000, pollInterval: 5_000 });

registerStore('singbox.watchdog', singboxWatchdogStatus);
