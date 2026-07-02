import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent, screen } from '@testing-library/svelte';
import SubscriptionImportPreview from './SubscriptionImportPreview.svelte';
import type { SubscriptionPreviewMember } from '$lib/types';

const members: SubscriptionPreviewMember[] = [
	{ key: 'k-alpha', label: 'Alpha NL', protocol: 'vless', server: 'nl.example.com', port: 443, security: 'reality', supported: true },
	{ key: 'k-bravo', label: 'Bravo DE', protocol: 'trojan', server: 'de.example.com', port: 8443, security: 'tls', supported: true },
	{ key: 'k-charlie', label: '', protocol: 'shadowsocks', server: 'us.example.com', port: 9000, supported: true },
];

function checkboxes(container: HTMLElement): HTMLInputElement[] {
	return Array.from(container.querySelectorAll('.list input[type="checkbox"]')) as HTMLInputElement[];
}

describe('SubscriptionImportPreview', () => {
	it('default empty excludedKeys keeps every selectable row checked', () => {
		const { container } = render(SubscriptionImportPreview, {
			props: {
				members,
				excludedKeys: new Set<string>(),
				ontoggle: vi.fn(),
				onselectAll: vi.fn(),
				onselectNone: vi.fn(),
			},
		});
		const boxes = checkboxes(container);
		expect(boxes).toHaveLength(3);
		expect(boxes.every((box) => box.checked)).toBe(true);
	});

	it('clicking a row checkbox calls ontoggle with that member key', async () => {
		const ontoggle = vi.fn();
		const { container } = render(SubscriptionImportPreview, {
			props: {
				members,
				excludedKeys: new Set<string>(),
				ontoggle,
				onselectAll: vi.fn(),
				onselectNone: vi.fn(),
			},
		});
		await fireEvent.click(checkboxes(container)[0]);
		expect(ontoggle).toHaveBeenCalledTimes(1);
		expect(ontoggle).toHaveBeenCalledWith('k-alpha');
	});

	it('a selectable member in excludedKeys renders unchecked with dropped style', () => {
		const { container } = render(SubscriptionImportPreview, {
			props: {
				members,
				excludedKeys: new Set<string>(['k-bravo']),
				ontoggle: vi.fn(),
				onselectAll: vi.fn(),
				onselectNone: vi.fn(),
			},
		});
		const boxes = checkboxes(container);
		expect(boxes[0].checked).toBe(true);
		expect(boxes[1].checked).toBe(false);
		expect(boxes[2].checked).toBe(true);

		const rows = Array.from(container.querySelectorAll('.row')) as HTMLElement[];
		expect(rows[1].classList.contains('dropped')).toBe(true);
		expect(rows[0].classList.contains('dropped')).toBe(false);
	});

	it('filter narrows visible rows by label/server, case-insensitively', async () => {
		const { container } = render(SubscriptionImportPreview, {
			props: {
				members,
				excludedKeys: new Set<string>(),
				ontoggle: vi.fn(),
				onselectAll: vi.fn(),
				onselectNone: vi.fn(),
			},
		});
		const input = container.querySelector('input.filter') as HTMLInputElement;

		await fireEvent.input(input, { target: { value: 'alpha' } });
		expect(checkboxes(container)).toHaveLength(1);
		expect((container.querySelector('.row .name') as HTMLElement).textContent).toContain('Alpha NL');

		await fireEvent.input(input, { target: { value: 'DE.EXAMPLE' } });
		const rows = Array.from(container.querySelectorAll('.row .addr')) as HTMLElement[];
		expect(rows).toHaveLength(1);
		expect(rows[0].textContent).toContain('de.example.com');

		await fireEvent.input(input, { target: { value: 'zzz-nomatch' } });
		expect(checkboxes(container)).toHaveLength(0);
		expect(container.querySelector('.empty-list')).toBeTruthy();
	});

	it('live counter reflects only selectable kept/excluded rows', async () => {
		const mixedMembers: SubscriptionPreviewMember[] = [
			...members.slice(0, 2),
			{
				key: 'k-unsupported',
				label: 'Banner line',
				protocol: 'unknown',
				server: '',
				port: 0,
				supported: false,
				reason: 'unsupported outbound protocol',
			},
		];

		const { container, rerender } = render(SubscriptionImportPreview, {
			props: {
				members: mixedMembers,
				excludedKeys: new Set<string>(),
				ontoggle: vi.fn(),
				onselectAll: vi.fn(),
				onselectNone: vi.fn(),
			},
		});
		expect((container.querySelector('.counter .kept') as HTMLElement).textContent).toContain('2 оставить');
		expect((container.querySelector('.counter .excluded') as HTMLElement).textContent).toContain('0 исключить');

		await rerender({
			members: mixedMembers,
			excludedKeys: new Set<string>(['k-alpha']),
			ontoggle: vi.fn(),
			onselectAll: vi.fn(),
			onselectNone: vi.fn(),
		});
		expect((container.querySelector('.counter .kept') as HTMLElement).textContent).toContain('1 оставить');
		expect((container.querySelector('.counter .excluded') as HTMLElement).textContent).toContain('1 исключить');
	});

	it('bulk actions still call onselectAll / onselectNone', async () => {
		const onselectAll = vi.fn();
		const onselectNone = vi.fn();
		render(SubscriptionImportPreview, {
			props: {
				members,
				excludedKeys: new Set<string>(),
				ontoggle: vi.fn(),
				onselectAll,
				onselectNone,
			},
		});
		await fireEvent.click(screen.getByText('Выбрать все'));
		await fireEvent.click(screen.getByText('Снять все'));
		expect(onselectAll).toHaveBeenCalledTimes(1);
		expect(onselectNone).toHaveBeenCalledTimes(1);
	});

	it('name and address fall back safely when server is empty', () => {
		const { container } = render(SubscriptionImportPreview, {
			props: {
				members: [{ key: 'empty-server', label: '', protocol: 'vless', server: '', port: 0, supported: true }],
				excludedKeys: new Set<string>(),
				ontoggle: vi.fn(),
				onselectAll: vi.fn(),
				onselectNone: vi.fn(),
			},
		});
		const name = container.querySelector('.row .name') as HTMLElement;
		const addr = container.querySelector('.row .addr') as HTMLElement;
		expect(name.textContent?.trim()).toBe('unknown-host');
		expect(addr.textContent?.trim()).toBe('unknown-host');
		expect(name.classList.contains('empty')).toBe(true);
	});

	it('renders unsupported rows with disabled checkbox and visible reason', () => {
		const { container } = render(SubscriptionImportPreview, {
			props: {
				members: [
					{
						key: 'unsupported',
						label: 'Bad proto',
						protocol: 'foo',
						server: '',
						port: 0,
						supported: false,
						reason: 'unsupported outbound protocol',
					},
				],
				excludedKeys: new Set<string>(),
				ontoggle: vi.fn(),
				onselectAll: vi.fn(),
				onselectNone: vi.fn(),
			},
		});
		const [box] = checkboxes(container);
		expect(box.disabled).toBe(true);
		expect(screen.getByText('Не поддерживается')).toBeTruthy();
		expect(screen.getByText('unsupported outbound protocol')).toBeTruthy();
	});

	it('virtualizes large preview lists instead of mounting every row at once', () => {
		const bigMembers = Array.from({ length: 1500 }, (_, index) => ({
			key: `k-${index}`,
			label: `Node ${index}`,
			protocol: index % 2 === 0 ? 'vless' : 'trojan',
			server: `node-${index}.example.com`,
			port: 443,
			supported: true,
		})) satisfies SubscriptionPreviewMember[];

		const { container } = render(SubscriptionImportPreview, {
			props: {
				members: bigMembers,
				excludedKeys: new Set<string>(),
				ontoggle: vi.fn(),
				onselectAll: vi.fn(),
				onselectNone: vi.fn(),
			},
		});

		const rows = container.querySelectorAll('.row');
		expect(rows.length).toBeGreaterThan(0);
		expect(rows.length).toBeLessThan(40);
		expect((container.querySelector('.counter .kept') as HTMLElement).textContent).toContain('1500 оставить');
		expect((rows[0] as HTMLElement).style.height).toBe('');
	});

	it('filter still works correctly with virtualization enabled', async () => {
		const bigMembers = Array.from({ length: 1200 }, (_, index) => ({
			key: `node-${index}`,
			label: `Node ${index}`,
			protocol: 'vless',
			server: `srv-${index}.example.com`,
			port: 443,
			supported: true,
		})) satisfies SubscriptionPreviewMember[];

		const { container } = render(SubscriptionImportPreview, {
			props: {
				members: bigMembers,
				excludedKeys: new Set<string>(),
				ontoggle: vi.fn(),
				onselectAll: vi.fn(),
				onselectNone: vi.fn(),
			},
		});

		await fireEvent.input(container.querySelector('input.filter') as HTMLInputElement, {
			target: { value: 'srv-1177' },
		});

		const rows = Array.from(container.querySelectorAll('.row .addr')) as HTMLElement[];
		expect(rows).toHaveLength(1);
		expect(rows[0].textContent).toContain('srv-1177.example.com');
	});
});
