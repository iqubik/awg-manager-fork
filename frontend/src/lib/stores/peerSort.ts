import { PEER_SORT_DEFAULTS, type PeerSortKey } from '$lib/utils/peerSort';
import { cycleTableSort } from '$lib/utils/tableSort';
import { createSortStore, type SortState } from './sortStore';

export type PeerSortState = SortState<PeerSortKey>;

const store = createSortStore<PeerSortKey>(
	'awg-manager-peer-sort',
	['name', 'traffic', 'ip', 'endpoint', 'online', 'handshake'],
	PEER_SORT_DEFAULTS,
);

export const peerSort = {
	subscribe: store.subscribe,
	setSort(sortBy: PeerSortKey | null, sortAsc: boolean) {
		store.mutate(() => ({ sortBy, sortAsc }));
	},
	setSortBy(key: PeerSortKey | null) {
		store.mutate((s) =>
			s.sortBy === key ? s : { sortBy: key, sortAsc: key ? PEER_SORT_DEFAULTS[key] : true },
		);
	},
	toggleSort(key: PeerSortKey) {
		store.mutate((state) => cycleTableSort(state, key));
	},
	toggleDir() {
		store.mutate((s) => (s.sortBy === null ? s : { ...s, sortAsc: !s.sortAsc }));
	},
};
