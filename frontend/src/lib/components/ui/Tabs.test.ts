import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/svelte';
import { readable } from 'svelte/store';
import Tabs from './Tabs.svelte';

vi.mock('$app/navigation', () => ({
	goto: vi.fn(),
}));

vi.mock('$app/stores', () => ({
	page: readable({
		url: new URL('http://localhost/routing'),
	}),
}));

class ResizeObserverMock {
	private callback: ResizeObserverCallback;

	constructor(callback: ResizeObserverCallback) {
		this.callback = callback;
	}

	observe(target: Element): void {
		this.callback(
			[
				{
					target,
					contentRect: {
						width: 100,
						height: 32,
						x: 0,
						y: 0,
						top: 0,
						left: 0,
						right: 100,
						bottom: 32,
						toJSON: () => ({}),
					},
				} as ResizeObserverEntry,
			],
			this as unknown as ResizeObserver,
		);
	}

	disconnect(): void {}
	unobserve(): void {}
}

describe('Tabs', () => {
	beforeEach(() => {
		vi.stubGlobal('ResizeObserver', ResizeObserverMock);
		vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (this: HTMLElement) {
			if (this.classList.contains('tab-separator')) {
				return { width: 8, height: 18, top: 0, left: 0, right: 8, bottom: 18, x: 0, y: 0, toJSON: () => ({}) } as DOMRect;
			}
			if (this.classList.contains('tab')) {
				return { width: 92, height: 36, top: 0, left: 0, right: 92, bottom: 36, x: 0, y: 0, toJSON: () => ({}) } as DOMRect;
			}
			return { width: 100, height: 36, top: 0, left: 0, right: 100, bottom: 36, x: 0, y: 0, toJSON: () => ({}) } as DOMRect;
		});
	});

	it('marks muted overflow dropdown items with the muted class', async () => {
		const { container } = render(Tabs, {
			props: {
				tabs: [
					{ id: 'dns', label: 'DNS' },
					{ id: 'singbox', label: 'TProxy' },
					{ id: 'fakeip', label: 'FakeIP', muted: true },
				],
				active: 'dns',
				onchange: vi.fn(),
			},
		});

		await fireEvent.click(screen.getByRole('button', { name: /\+2/ }));

		const mutedItem = container.querySelector('.dropdown-item.muted');
		expect(mutedItem?.textContent).toContain('FakeIP');
	});
});
