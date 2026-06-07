import { writable } from 'svelte/store';
import { browser } from '$app/environment';
import { PEER_SORT_DEFAULTS, type PeerSortKey } from '$lib/utils/peerSort';
import { cycleTableSort } from '$lib/utils/tableSort';

const storageKey = 'awg-manager-peer-sort';

const VALID_KEYS = new Set<PeerSortKey>(['name', 'traffic', 'ip', 'endpoint', 'online', 'handshake']);

export interface PeerSortState {
	sortBy: PeerSortKey | null;
	sortAsc: boolean;
}

function defaultState(): PeerSortState {
	return { sortBy: null, sortAsc: true };
}

function getInitial(): PeerSortState {
	if (!browser) return defaultState();
	try {
		const raw = localStorage.getItem(storageKey);
		if (!raw) return defaultState();
		const parsed: unknown = JSON.parse(raw);
		if (!parsed || typeof parsed !== 'object') return defaultState();
		const { sortBy, sortAsc } = parsed as Partial<PeerSortState>;
		if (sortBy !== null && sortBy !== undefined && !VALID_KEYS.has(sortBy)) return defaultState();
		return {
			sortBy: sortBy ?? null,
			sortAsc:
				typeof sortAsc === 'boolean'
					? sortAsc
					: sortBy != null
						? PEER_SORT_DEFAULTS[sortBy]
						: true,
		};
	} catch {
		return defaultState();
	}
}

function persist(state: PeerSortState) {
	if (!browser) return;
	try {
		localStorage.setItem(storageKey, JSON.stringify(state));
	} catch {
		// quota / private-mode: silently ignore
	}
}

function createPeerSortStore() {
	const { subscribe, set, update } = writable<PeerSortState>(getInitial());

	return {
		subscribe,
		setSort(sortBy: PeerSortKey | null, sortAsc: boolean) {
			const next: PeerSortState = { sortBy, sortAsc };
			persist(next);
			set(next);
		},
		setSortBy(key: PeerSortKey | null) {
			update((s) => {
				if (s.sortBy === key) return s;
				const next: PeerSortState = {
					sortBy: key,
					sortAsc: true,
				};
				persist(next);
				return next;
			});
		},
		toggleSort(key: PeerSortKey) {
			update((state) => {
				const next = cycleTableSort(state, key);
				persist(next);
				return next;
			});
		},
		toggleDir() {
			update((s) => {
				if (s.sortBy === null) return s;
				const next: PeerSortState = { ...s, sortAsc: !s.sortAsc };
				persist(next);
				return next;
			});
		},
	};
}

export const peerSort = createPeerSortStore();
