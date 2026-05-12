import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, fireEvent } from '@testing-library/svelte';
import RulesSubTab from './RulesSubTab.svelte';

// Hoist Svelte writables that the mocked store will expose. Tests mutate
// the rules store directly to drive the rendered table.
const { statusStore, rulesStore, ruleSetsStore, optionsStore, pageStore } = vi.hoisted(() => {
	const { writable } = require('svelte/store') as typeof import('svelte/store');
	return {
		statusStore: writable<unknown>(null),
		rulesStore: writable<unknown[]>([]),
		ruleSetsStore: writable<unknown[]>([]),
		optionsStore: writable<unknown[]>([]),
		pageStore: writable<{ url: URL }>({ url: new URL('http://localhost/routing?sub=rules') }),
	};
});

vi.mock('$lib/stores/singboxRouter', () => ({
	singboxRouter: {
		status: { subscribe: statusStore.subscribe },
		rules: { subscribe: rulesStore.subscribe },
		ruleSets: { subscribe: ruleSetsStore.subscribe },
		options: { subscribe: optionsStore.subscribe },
		loadAll: vi.fn().mockResolvedValue(undefined),
	},
}));

vi.mock('$lib/api/client', () => ({
	api: {
		singboxRouterMoveRule: vi.fn().mockResolvedValue(undefined),
	},
}));

vi.mock('$app/navigation', () => ({
	goto: vi.fn().mockResolvedValue(undefined),
}));

vi.mock('$app/stores', () => ({
	page: { subscribe: pageStore.subscribe },
}));

vi.mock('$lib/stores/notifications', () => ({
	notifications: {
		success: vi.fn(),
		error: vi.fn(),
		warning: vi.fn(),
		info: vi.fn(),
	},
}));

// Rule shape mirrors SingboxRouterRule. Subset that RulesSubTab reads.
type R = {
	action: string;
	outbound?: string;
	protocol?: string;
	domain_suffix?: string[];
	ip_cidr?: string[];
};

// Standard ordering: 2 system rules (sniff, hijack-dns) at top, then user rules.
// firstUserRuleIndex = 2 in this layout.
const systemRules: R[] = [
	{ action: 'sniff' },
	{ action: 'hijack-dns', protocol: 'dns' },
];

function buildRules(userRules: R[]): R[] {
	return [...systemRules, ...userRules];
}

beforeEach(() => {
	vi.clearAllMocks();
	statusStore.set({ final: 'direct' });
	ruleSetsStore.set([]);
	optionsStore.set([]);
	pageStore.set({ url: new URL('http://localhost/routing?sub=rules') });
});

describe('RulesSubTab — rule ordering controls (PR #101 commit #3)', () => {
	it('renders no move/drag controls for system rules (sniff, hijack-dns)', () => {
		rulesStore.set(buildRules([{ action: 'route', outbound: 'veesp' }]));
		const { container } = render(RulesSubTab);

		const rows = container.querySelectorAll('.t-row');
		expect(rows.length).toBe(3); // 2 system + 1 user

		// System rows must NOT contain drag-handle or move-btn.
		const sysRow0 = rows[0];
		expect(sysRow0.querySelector('.drag-handle')).toBeNull();
		expect(sysRow0.querySelector('.move-btn')).toBeNull();

		const sysRow1 = rows[1];
		expect(sysRow1.querySelector('.drag-handle')).toBeNull();
		expect(sysRow1.querySelector('.move-btn')).toBeNull();

		// User row must have BOTH controls.
		const userRow = rows[2];
		expect(userRow.querySelector('.drag-handle')).toBeTruthy();
		expect(userRow.querySelectorAll('.move-btn').length).toBe(2);
	});

	it('disables ↑ on the first user rule (cannot escape system zone)', () => {
		rulesStore.set(buildRules([
			{ action: 'route', outbound: 'veesp' },
			{ action: 'route', outbound: 'awg-1' },
		]));
		const { container } = render(RulesSubTab);

		const rows = container.querySelectorAll('.t-row');
		// rows[2] is the first user rule (firstUserRuleIndex == 2).
		const firstUserBtns = rows[2].querySelectorAll('.move-btn');
		const upBtn = firstUserBtns[0] as HTMLButtonElement;
		const downBtn = firstUserBtns[1] as HTMLButtonElement;
		expect(upBtn.disabled).toBe(true); // i <= firstUserRuleIndex
		expect(downBtn.disabled).toBe(false); // can go down
	});

	it('enables ↑ on a middle user rule (room to move up within user zone)', () => {
		rulesStore.set(buildRules([
			{ action: 'route', outbound: 'veesp' },
			{ action: 'route', outbound: 'awg-1' },
			{ action: 'route', outbound: 'sub-1' },
		]));
		const { container } = render(RulesSubTab);

		const rows = container.querySelectorAll('.t-row');
		// rows[3] is the middle user rule (index 3, firstUserRuleIndex == 2).
		const midBtns = rows[3].querySelectorAll('.move-btn');
		expect((midBtns[0] as HTMLButtonElement).disabled).toBe(false);
		expect((midBtns[1] as HTMLButtonElement).disabled).toBe(false);
	});

	it('disables ↓ on the last rule (no room to go further down)', () => {
		rulesStore.set(buildRules([
			{ action: 'route', outbound: 'veesp' },
			{ action: 'route', outbound: 'awg-1' },
		]));
		const { container } = render(RulesSubTab);

		const rows = container.querySelectorAll('.t-row');
		const lastBtns = rows[rows.length - 1].querySelectorAll('.move-btn');
		expect((lastBtns[1] as HTMLButtonElement).disabled).toBe(true); // i >= rules.length-1
	});

	it('clicking ↓ on a user rule calls api.singboxRouterMoveRule(i, i+1)', async () => {
		rulesStore.set(buildRules([
			{ action: 'route', outbound: 'veesp' },
			{ action: 'route', outbound: 'awg-1' },
		]));
		const { container } = render(RulesSubTab);
		const { api } = await import('$lib/api/client');

		const rows = container.querySelectorAll('.t-row');
		// rows[2] = first user rule (index 2). Click its ↓.
		const downBtn = rows[2].querySelectorAll('.move-btn')[1] as HTMLButtonElement;
		await fireEvent.click(downBtn);

		expect(api.singboxRouterMoveRule).toHaveBeenCalledOnce();
		expect(api.singboxRouterMoveRule).toHaveBeenCalledWith(2, 3);
	});

	it('clicking ↑ on a movable rule calls api.singboxRouterMoveRule(i, i-1)', async () => {
		rulesStore.set(buildRules([
			{ action: 'route', outbound: 'veesp' },
			{ action: 'route', outbound: 'awg-1' },
		]));
		const { container } = render(RulesSubTab);
		const { api } = await import('$lib/api/client');

		const rows = container.querySelectorAll('.t-row');
		// rows[3] = second user rule (index 3). Click its ↑.
		const upBtn = rows[3].querySelectorAll('.move-btn')[0] as HTMLButtonElement;
		await fireEvent.click(upBtn);

		expect(api.singboxRouterMoveRule).toHaveBeenCalledOnce();
		expect(api.singboxRouterMoveRule).toHaveBeenCalledWith(3, 2);
	});
});
