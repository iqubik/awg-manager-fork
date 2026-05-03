import { writable } from 'svelte/store';
import type { LogEntry } from '$lib/types';
import type { LogEntryEvent } from '$lib/api/events';

export type LogBucket = 'app' | 'singbox';

const MAX_ENTRIES = 5000;

function keyOf(e: LogEntry): string {
  return `${e.timestamp}|${e.level}|${e.group}|${e.subgroup}|${e.action}|${e.target}|${e.message}`;
}

export interface BufferStats {
  size: number;
  capacity: number;
  oldest?: string;
}

function createLogStore(bucket: LogBucket) {
  const { subscribe, update, set } = writable<LogEntry[]>([]);
  const logsEnabled = writable(true);
  const logsTotal = writable(0);
  const loaded = writable(false);
  const lastSeenTs = writable<number>(0);
  const stats = writable<BufferStats>({ size: 0, capacity: MAX_ENTRIES });

  return {
    bucket,
    subscribe,
    enabled: { subscribe: logsEnabled.subscribe },
    total: { subscribe: logsTotal.subscribe },
    loaded: { subscribe: loaded.subscribe },
    lastSeenTs: { subscribe: lastSeenTs.subscribe },
    stats: { subscribe: stats.subscribe },
    /** SSE-driven head append (newest at front). */
    append(entry: LogEntryEvent) {
      const logEntry: LogEntry = {
        ...entry,
        subgroup: entry.subgroup ?? '',
      };
      const ts = new Date(entry.timestamp).getTime();
      lastSeenTs.update((cur) => (ts > cur ? ts : cur));
      update((entries) => {
        const updated = [logEntry, ...entries];
        if (updated.length > MAX_ENTRIES) updated.length = MAX_ENTRIES;
        return updated;
      });
      logsTotal.update((n) => n + 1);
    },
    /** Catch-up merge after SSE reconnect — keeps entries newest-first, dedups by composite key. */
    appendMany(arr: LogEntry[]) {
      update((entries) => {
        const seen = new Set(entries.map(keyOf));
        const newOnes: LogEntry[] = [];
        for (const e of arr) {
          if (!seen.has(keyOf(e))) newOnes.push(e);
        }
        if (newOnes.length === 0) return entries;
        const newestTs = newOnes.reduce((m, e) => Math.max(m, new Date(e.timestamp).getTime()), 0);
        if (newestTs > 0) lastSeenTs.update((cur) => (newestTs > cur ? newestTs : cur));
        const merged = [...newOnes, ...entries].sort(
          (a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime(),
        );
        if (merged.length > MAX_ENTRIES) merged.length = MAX_ENTRIES;
        return merged;
      });
    },
    /** Pagination tail append — concatenates older entries below the newest, dedups. */
    appendPage(arr: LogEntry[]) {
      update((entries) => {
        const seen = new Set(entries.map(keyOf));
        const newOnes: LogEntry[] = [];
        for (const e of arr) {
          if (!seen.has(keyOf(e))) newOnes.push(e);
        }
        if (newOnes.length === 0) return entries;
        return [...entries, ...newOnes];
      });
    },
    clear() {
      set([]);
      logsTotal.set(0);
      lastSeenTs.set(0);
      stats.set({ size: 0, capacity: MAX_ENTRIES });
    },
    setEntries(entries: LogEntry[]) {
      set(entries);
      const newest = entries.reduce((m, e) => Math.max(m, new Date(e.timestamp).getTime()), 0);
      lastSeenTs.set(newest);
    },
    setTotal(n: number) {
      logsTotal.set(n);
    },
    setEnabled(v: boolean) {
      logsEnabled.set(v);
    },
    setLoaded(v: boolean) {
      loaded.set(v);
    },
    setStats(s: BufferStats) {
      stats.set(s);
    },
  };
}

export type LogStore = ReturnType<typeof createLogStore>;

export const appLogEntries = createLogStore('app');
export const singboxLogEntries = createLogStore('singbox');

export function logStoreFor(bucket: LogBucket): LogStore {
  return bucket === 'singbox' ? singboxLogEntries : appLogEntries;
}

/** Backwards-compat alias for callers that haven't migrated yet. */
export const logEntries = appLogEntries;
