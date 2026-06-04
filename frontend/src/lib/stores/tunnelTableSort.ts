import { writable } from 'svelte/store';
import { browser } from '$app/environment';
import {
	AWG_TUNNEL_SORT_DEFAULTS,
	SINGBOX_TUNNEL_SORT_DEFAULTS,
	SUBSCRIPTION_SORT_DEFAULTS,
} from '$lib/utils/tunnelTableSort';

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
						: sortBy === null
							? true
							: defaults[sortBy],
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

	const { subscribe, set, update } = writable<TunnelTableSortState<T>>(getInitial());

	return {
		subscribe,
		setSort(sortBy: T | null, sortAsc: boolean) {
			const next: TunnelTableSortState<T> = { sortBy, sortAsc };
			persist(next);
			set(next);
		},
		setSortBy(sortBy: T | null) {
			update((state) => {
				const next: TunnelTableSortState<T> = {
					sortBy,
					sortAsc: sortBy === null ? true : defaults[sortBy],
				};
				persist(next);
				return next;
			});
		},
		toggleSort(key: T) {
			update((state) => {
				const next: TunnelTableSortState<T> =
					state.sortBy === key
						? { sortBy: key, sortAsc: !state.sortAsc }
						: { sortBy: key, sortAsc: defaults[key] };
				persist(next);
				return next;
			});
		},
		toggleDir() {
			update((state) => {
				const next = { ...state, sortAsc: !state.sortAsc };
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
