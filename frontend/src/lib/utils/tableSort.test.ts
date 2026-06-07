import { describe, it, expect } from 'vitest';
import { cycleTableSort } from './tableSort';

describe('cycleTableSort', () => {
	it('activates any column ascending', () => {
		expect(cycleTableSort({ sortBy: null, sortAsc: true }, 'traffic')).toEqual({
			sortBy: 'traffic',
			sortAsc: true,
		});
		expect(cycleTableSort({ sortBy: null, sortAsc: true }, 'name')).toEqual({
			sortBy: 'name',
			sortAsc: true,
		});
	});

	it('cycles asc → desc → null', () => {
		expect(cycleTableSort({ sortBy: 'name', sortAsc: true }, 'name')).toEqual({
			sortBy: 'name',
			sortAsc: false,
		});
		expect(cycleTableSort({ sortBy: 'name', sortAsc: false }, 'name')).toEqual({
			sortBy: null,
			sortAsc: true,
		});
		expect(cycleTableSort({ sortBy: 'traffic', sortAsc: true }, 'traffic')).toEqual({
			sortBy: 'traffic',
			sortAsc: false,
		});
		expect(cycleTableSort({ sortBy: 'traffic', sortAsc: false }, 'traffic')).toEqual({
			sortBy: null,
			sortAsc: true,
		});
	});

	it('switches column and starts ascending', () => {
		expect(cycleTableSort({ sortBy: 'name', sortAsc: false }, 'traffic')).toEqual({
			sortBy: 'traffic',
			sortAsc: true,
		});
	});
});
