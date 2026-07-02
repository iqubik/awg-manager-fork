import { describe, expect, it, vi } from 'vitest';
import { render } from '@testing-library/svelte';
import { readable } from 'svelte/store';
import MobileTabRail from './MobileTabRail.svelte';

vi.mock('$app/navigation', () => ({
	goto: vi.fn(),
}));

vi.mock('$app/stores', () => ({
	page: readable({
		url: new URL('http://localhost/routing'),
	}),
}));

describe('MobileTabRail', () => {
	it('marks muted tabs with the muted class', () => {
		const { container } = render(MobileTabRail, {
			props: {
				tabs: [
					{ id: 'dns', label: 'DNS' },
					{ id: 'fakeip', label: 'FakeIP', muted: true },
				],
				active: 'dns',
				onchange: vi.fn(),
			},
		});

		const fakeip = container.querySelector('[data-tab-id="fakeip"]');
		expect(fakeip?.classList.contains('is-muted')).toBe(true);
		expect(fakeip?.classList.contains('is-active')).toBe(false);
	});
});
