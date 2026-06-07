export const DEFAULT_SORT_VALUE = '__default__';

export interface TableSortState<T extends string> {
	sortBy: T | null;
	sortAsc: boolean;
}

/** asc → desc → unsorted */
export function cycleTableSort<T extends string>(
	state: TableSortState<T>,
	key: T,
): TableSortState<T> {
	if (state.sortBy !== key) {
		return { sortBy: key, sortAsc: true };
	}
	if (state.sortAsc) {
		return { sortBy: key, sortAsc: false };
	}
	return { sortBy: null, sortAsc: true };
}
