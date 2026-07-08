import type { PingLogEntry, SingboxWatchdogLogEntry } from '$lib/types';

export type WatchdogCheckEntry = {
	key: string;
	timestamp: string;
	success: boolean;
	latency?: number;
	error?: string;
	failCount?: number;
	threshold?: number;
	stateChange?: string;
	memberFrom?: string;
	memberTo?: string;
	source?: 'awg-kernel' | 'awg-nativewg' | 'singbox-tunnel' | 'singbox-subscription';
};

export function normalizePingLogEntry(entry: PingLogEntry, index = 0): WatchdogCheckEntry {
	return {
		key: `${entry.timestamp}:${entry.tunnelId}:${entry.failCount}:${entry.stateChange}:${index}`,
		timestamp: entry.timestamp,
		success: entry.success,
		latency: entry.success && entry.latency >= 0 ? entry.latency : undefined,
		error: entry.success ? (entry.latency < 0 ? 'NDMS' : '') : entry.error || (entry.backend === 'nativewg' ? 'NDMS' : 'timeout'),
		failCount: entry.failCount,
		threshold: entry.threshold,
		stateChange: entry.stateChange || '',
		source: entry.backend === 'nativewg' ? 'awg-nativewg' : 'awg-kernel',
	};
}

export function normalizeSingboxWatchdogLogEntry(
	entry: SingboxWatchdogLogEntry,
	index = 0,
): WatchdogCheckEntry {
	return {
		key: `${entry.timestamp}:${entry.targetId}:${entry.failCount}:${entry.stateChange}:${entry.memberTo ?? ''}:${index}`,
		timestamp: entry.timestamp,
		success: entry.success,
		latency: entry.success && entry.latency > 0 ? entry.latency : undefined,
		error: entry.success ? '' : entry.error || 'timeout',
		failCount: entry.failCount,
		threshold: entry.threshold,
		stateChange: entry.stateChange || '',
		memberFrom: entry.memberFrom,
		memberTo: entry.memberTo,
		source: entry.kind === 'subscription' ? 'singbox-subscription' : 'singbox-tunnel',
	};
}

export function sortWatchdogEntries(entries: WatchdogCheckEntry[]): WatchdogCheckEntry[] {
	return [...entries].sort((a, b) => Date.parse(b.timestamp) - Date.parse(a.timestamp));
}

export function formatWatchdogLatency(entry: WatchdogCheckEntry): string {
	return typeof entry.latency === 'number' && entry.latency >= 0 ? `${entry.latency}ms` : '—';
}

export function formatWatchdogState(entry: WatchdogCheckEntry): string {
	if (entry.memberFrom && entry.memberTo) return `${entry.memberFrom} → ${entry.memberTo}`;
	return entry.stateChange || '';
}
