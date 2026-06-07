import { writable } from 'svelte/store';
import { browser } from '$app/environment';
import {
	AWG_TUNNEL_SORT_DEFAULTS,
	SINGBOX_TUNNEL_SORT_DEFAULTS,
	SUBSCRIPTION_SORT_DEFAULTS,
} from '$lib/utils/tunnelTableSort';
import { cycleTableSort } from '$lib/utils/tableSort';

export type AwgTunnelSortKey = 'name' | 'status' | 'endpoint' | 'traffic' | 'handshake';
export type SingboxTunnelSortKey = 'delay' | 'name' | 'protocol' | 'server' | 'running' | 'traffic' | 'ping';
export type SubscriptionSortKey = 'delay' | 'label' | 'mode' | 'active' | 'traffic' | 'updated' | 'ping';

export interface TunnelTableSortState<T extends string> {
	sortBy: T | null;
	sortAsc: boolean;
}

function createTunnelTableSortStore<T extends string>(
	storageKey: string,
	validKeys: readonly T[],
	defaults: Record<T, boolean>,
) {
	const valid = new Set(validKeys);

	function defaultState(): TunnelTableSortState<T> {
		return { sortBy: null, sortAsc: true };
	}

	function getInitial(): TunnelTableSortState<T> {
		if (!browser) return defaultState();
		try {
			const raw = localStorage.getItem(storageKey);
			if (!raw) return defaultState();
			const parsed = JSON.parse(raw) as Partial<TunnelTableSortState<T>> | null;
			if (!parsed || typeof parsed !== 'object') return defaultState();
			const sortBy = parsed.sortBy ?? null;
			if (sortBy !== null && !valid.has(sortBy)) return defaultState();
			return {
				sortBy,
				sortAsc:
					typeof parsed.sortAsc === 'boolean'
						? parsed.sortAsc
						: sortBy !== null
							? defaults[sortBy]
							: true,
			};
		} catch {
			return defaultState();
		}
	}

	function persist(state: TunnelTableSortState<T>): void {
		if (!browser) return;
		try {
			localStorage.setItem(storageKey, JSON.stringify(state));
		} catch {
			// ignore storage issues
		}
	}

	const { subscribe, update } = writable<TunnelTableSortState<T>>(getInitial());

	return {
		subscribe,
		toggleSort(key: T) {
			update((state) => {
				const next = cycleTableSort(state, key);
				persist(next);
				return next;
			});
		},
	};
}

export const awgTunnelTableSort = createTunnelTableSortStore<AwgTunnelSortKey>(
	'awgm:tunnels:awg-table-sort',
	['name', 'status', 'endpoint', 'traffic', 'handshake'],
	AWG_TUNNEL_SORT_DEFAULTS,
);

export const singboxTunnelTableSort = createTunnelTableSortStore<SingboxTunnelSortKey>(
	'awgm:tunnels:singbox-table-sort',
	['delay', 'name', 'protocol', 'server', 'running', 'traffic', 'ping'],
	SINGBOX_TUNNEL_SORT_DEFAULTS,
);

export const singboxSubscriptionTableSort = createTunnelTableSortStore<SubscriptionSortKey>(
	'awgm:tunnels:subscriptions-table-sort',
	['delay', 'label', 'mode', 'active', 'traffic', 'updated', 'ping'],
	SUBSCRIPTION_SORT_DEFAULTS,
);
