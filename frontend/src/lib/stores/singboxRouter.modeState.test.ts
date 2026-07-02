import { beforeEach, describe, expect, it, vi } from 'vitest';
import { get } from 'svelte/store';

vi.mock('$lib/api/client', () => ({
	api: {
		singboxRouterStagingStatus: vi.fn(),
		singboxRouterStatus: vi.fn(),
		singboxRouterGetSettings: vi.fn(),
		singboxRouterListRules: vi.fn().mockResolvedValue([]),
		singboxRouterListRuleSets: vi.fn().mockResolvedValue([]),
		singboxRouterListOutbounds: vi.fn().mockResolvedValue([]),
		singboxRouterListPresets: vi.fn().mockResolvedValue([]),
		singboxRouterListDNSServers: vi.fn().mockResolvedValue([]),
		singboxRouterListDNSRules: vi.fn().mockResolvedValue([]),
		singboxRouterListDNSRewrites: vi.fn().mockResolvedValue([]),
		singboxRouterGetDNSGlobals: vi.fn().mockResolvedValue({ final: '', strategy: '' }),
	},
}));

vi.mock('$lib/stores/awgTags', () => ({
	awgTags: { subscribe: vi.fn(() => () => {}) },
}));

vi.mock('$lib/stores/subscriptions', () => ({
	subscriptionsStore: { subscribe: vi.fn(() => () => {}) },
}));

vi.mock('$lib/stores/singbox', () => ({
	singboxTunnels: { subscribe: vi.fn(() => () => {}) },
}));

vi.mock('$lib/components/routing/singboxRouter/outboundOptions', () => ({
	buildOutboundOptions: vi.fn(() => []),
}));

import { api } from '$lib/api/client';
import { singboxRouter } from './singboxRouter';

describe('singboxRouter.primeModeState', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		singboxRouter.setSettings(null);
	});

	it('loads status and settings without marking the store initialized', async () => {
		const statusPayload = { enabled: true, ruleCount: 7 } as any;
		const settingsPayload = { routingMode: 'tproxy' } as any;

		vi.mocked(api.singboxRouterStatus).mockResolvedValue(statusPayload);
		vi.mocked(api.singboxRouterGetSettings).mockResolvedValue(settingsPayload);

		await singboxRouter.primeModeState();

		expect(api.singboxRouterStatus).toHaveBeenCalledTimes(1);
		expect(api.singboxRouterGetSettings).toHaveBeenCalledTimes(1);
		expect(get(singboxRouter.status)).toEqual(statusPayload);
		expect(get(singboxRouter.settings)).toEqual(settingsPayload);
		expect(get(singboxRouter.initialized)).toBe(false);
	});

	it('updates whichever payload succeeds', async () => {
		const statusPayload = { enabled: true, ruleCount: 3 } as any;

		vi.mocked(api.singboxRouterStatus).mockResolvedValue(statusPayload);
		vi.mocked(api.singboxRouterGetSettings).mockRejectedValue(new Error('boom'));

		await singboxRouter.primeModeState();

		expect(get(singboxRouter.status)).toEqual(statusPayload);
		expect(get(singboxRouter.settings)).toBeNull();
	});
});
