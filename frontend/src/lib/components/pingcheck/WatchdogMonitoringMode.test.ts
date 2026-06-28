import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import type {
	SingboxStatus,
	SingboxTunnel,
	Subscription,
	TunnelListItem,
	TunnelPingStatus,
} from '$lib/types';

const {
	getTunnelsAll,
	getTunnel,
	getNativePingCheckStatus,
	triggerPingCheck,
	singboxDelayCheck,
	getSubscriptionActiveNow,
	loadPingLogs,
	goto,
	loadHistory,
	getTrafficRates,
	subscribeTraffic,
	pingStatusStore,
	pingLogsStore,
	singboxStatusStore,
	singboxTunnelsStore,
	subscriptionsStoreMock,
	singboxDelayHistoryStore,
	usageLevelStore,
} = vi.hoisted(() => {
	const { writable } = require('svelte/store') as typeof import('svelte/store');
	const matchMedia = () => ({
		matches: false,
		media: '',
		onchange: null,
		addListener: vi.fn(),
		removeListener: vi.fn(),
		addEventListener: vi.fn(),
		removeEventListener: vi.fn(),
		dispatchEvent: vi.fn(),
	});
	if (!(globalThis as { matchMedia?: unknown }).matchMedia) {
		Object.defineProperty(globalThis, 'matchMedia', { value: matchMedia, configurable: true });
	}
	return {
		getTunnelsAll: vi.fn(),
		getTunnel: vi.fn(),
		getNativePingCheckStatus: vi.fn(),
		triggerPingCheck: vi.fn(),
		singboxDelayCheck: vi.fn(),
		getSubscriptionActiveNow: vi.fn(),
		loadPingLogs: vi.fn(),
		goto: vi.fn(),
		loadHistory: vi.fn(),
		getTrafficRates: vi.fn(() => ({ rx: [64, 128], tx: [32, 96] })),
		subscribeTraffic: vi.fn((run: () => void) => {
			run();
			return () => {};
		}),
		pingStatusStore: writable<any>({
			data: [],
			status: 'fresh',
			error: null,
			lastFetchedAt: Date.now(),
			consecutiveFailures: 0,
		}),
		pingLogsStore: writable<any>([]),
		singboxStatusStore: writable<any>({
			data: null,
			status: 'idle',
			error: null,
			lastFetchedAt: 0,
			consecutiveFailures: 0,
		}),
		singboxTunnelsStore: writable<any>({
			data: null,
			status: 'idle',
			error: null,
			lastFetchedAt: 0,
			consecutiveFailures: 0,
		}),
		subscriptionsStoreMock: writable<any>({
			data: null,
			status: 'idle',
			error: null,
			lastFetchedAt: 0,
			consecutiveFailures: 0,
		}),
		singboxDelayHistoryStore: writable<any>(new Map()),
		usageLevelStore: writable<'basic' | 'advanced' | 'expert'>('advanced'),
	};
});

vi.mock('$lib/api/client', () => ({
	api: {
		getTunnelsAll,
		getTunnel,
		getNativePingCheckStatus,
		triggerPingCheck,
		singboxDelayCheck,
		getSubscriptionActiveNow,
	},
}));

vi.mock('$lib/stores/pingcheck', () => ({
	pingCheckStatus: {
		subscribe: pingStatusStore.subscribe,
		refetch: vi.fn(),
		invalidate: vi.fn(),
		applyMutationResponse: vi.fn(),
	},
	pingCheckLogs: {
		subscribe: pingLogsStore.subscribe,
	},
	loadPingLogs,
}));

vi.mock('$lib/stores/singbox', () => ({
	singboxStatus: {
		subscribe: singboxStatusStore.subscribe,
		refetch: vi.fn(),
		invalidate: vi.fn(),
		applyMutationResponse: vi.fn(),
	},
	singboxTunnels: {
		subscribe: singboxTunnelsStore.subscribe,
		refetch: vi.fn(),
		invalidate: vi.fn(),
		applyMutationResponse: vi.fn(),
	},
	singboxDelayHistory: {
		subscribe: singboxDelayHistoryStore.subscribe,
	},
}));

vi.mock('$lib/stores/subscriptions', () => ({
	subscriptionsStore: {
		subscribe: subscriptionsStoreMock.subscribe,
		refetch: vi.fn(),
		invalidate: vi.fn(),
		applyMutationResponse: vi.fn(),
	},
}));

vi.mock('$lib/stores/settings', () => ({
	usageLevel: {
		subscribe: usageLevelStore.subscribe,
	},
}));

vi.mock('$lib/stores/notifications', () => ({
	notifications: {
		error: vi.fn(),
		success: vi.fn(),
	},
}));

vi.mock('$lib/stores/traffic', () => ({
	loadHistory,
	getTrafficRates,
	subscribeTraffic,
}));

vi.mock('$app/navigation', () => ({
	goto,
}));

function setPingStatuses(data: TunnelPingStatus[]): void {
	pingStatusStore.set({
		data,
		status: 'fresh',
		error: null,
		lastFetchedAt: Date.now(),
		consecutiveFailures: 0,
	});
}

function setSingboxStatus(data: SingboxStatus | null, status: 'idle' | 'loading' | 'fresh' | 'stale' | 'error' = 'fresh'): void {
	singboxStatusStore.set({
		data,
		status,
		error: status === 'error' ? 'boom' : null,
		lastFetchedAt: status === 'fresh' || status === 'stale' ? Date.now() : 0,
		consecutiveFailures: status === 'error' ? 1 : 0,
	});
}

function setSingboxTunnels(data: SingboxTunnel[] | null, status: 'idle' | 'loading' | 'fresh' | 'stale' | 'error' = 'fresh'): void {
	singboxTunnelsStore.set({
		data,
		status,
		error: status === 'error' ? 'boom' : null,
		lastFetchedAt: status === 'fresh' || status === 'stale' ? Date.now() : 0,
		consecutiveFailures: status === 'error' ? 1 : 0,
	});
}

function setSubscriptions(data: Subscription[] | null, status: 'idle' | 'loading' | 'fresh' | 'stale' | 'error' = 'fresh'): void {
	subscriptionsStoreMock.set({
		data,
		status,
		error: status === 'error' ? 'boom' : null,
		lastFetchedAt: status === 'fresh' || status === 'stale' ? Date.now() : 0,
		consecutiveFailures: status === 'error' ? 1 : 0,
	});
}

const sampleSingboxTunnel: SingboxTunnel = {
	tag: 'sb-main',
	protocol: 'vless',
	server: '1.2.3.4',
	port: 443,
	security: 'reality',
	transport: 'tcp',
	listenPort: 1080,
	proxyInterface: 'proxy0',
	kernelInterface: 'tun0',
	connectivity: {
		connected: true,
		latency: 125,
	},
	running: true,
};

const baseSingboxStatus: SingboxStatus = {
	installed: true,
	running: true,
	tunnelCount: 0,
	proxyComponent: true,
	ndmsProxyEnabled: true,
	requiredVersion: '1.10.0',
	updateAvailable: false,
};

const sampleAwgMeta: TunnelListItem = {
	id: 'awg-1',
	name: 'AWG Alpha',
	type: 'awg',
	status: 'running',
	enabled: true,
	endpoint: '8.8.8.8:51820',
	address: '10.0.0.2/32',
	backend: 'kernel',
	pingCheck: {
		status: 'alive',
		restartCount: 0,
		failCount: 0,
		failThreshold: 3,
	},
};

const sampleSubscription: Subscription = {
	id: 'sub-1',
	label: 'America Pool',
	url: 'https://example.com/sub',
	isInline: false,
	headers: [],
	refreshHours: 12,
	lastFetched: '',
	selectorTag: 'iq0',
	inboundTag: 'default',
	listenPort: 1081,
	proxyIndex: 5,
	memberTags: ['member-1'],
	members: [
		{
			tag: 'member-1',
			label: 'US-1',
			protocol: 'vless',
			server: '5.5.5.5',
			port: 443,
			security: 'reality',
			transport: 'tcp',
		},
	],
	orphanTags: [],
	activeMember: 'member-1',
	enabled: true,
	mode: 'urltest',
};

const { default: WatchdogMonitoringMode } = await import('./WatchdogMonitoringMode.svelte');

describe('WatchdogMonitoringMode', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		localStorage.clear();
		loadPingLogs.mockResolvedValue(undefined);
		getTunnelsAll.mockResolvedValue({ tunnels: [] });
		getTunnel.mockResolvedValue({
			pingCheck: {
				enabled: true,
				method: 'icmp',
				target: '1.1.1.1',
				interval: 60,
				deadInterval: 60,
				failThreshold: 3,
				minSuccess: 1,
				timeout: 3,
				restart: true,
			},
		});
		singboxDelayCheck.mockResolvedValue(undefined);
		getSubscriptionActiveNow.mockResolvedValue({ now: 'member-1' });
		setPingStatuses([]);
		pingLogsStore.set([]);
		setSingboxStatus(null, 'idle');
		setSingboxTunnels(null, 'idle');
		setSubscriptions(null, 'idle');
		singboxDelayHistoryStore.set(new Map());
		usageLevelStore.set('advanced');
	});

	it('shows a raw sing-box card when AWG is empty', async () => {
		setSingboxStatus({ ...baseSingboxStatus, tunnelCount: 1 });
		setSingboxTunnels([sampleSingboxTunnel]);
		setSubscriptions([]);
		singboxDelayHistoryStore.set(new Map([['sb-main', [125]]])); 

		render(WatchdogMonitoringMode);

		expect(await screen.findByRole('button', { name: /Sing-box/i })).toBeTruthy();
		expect(await screen.findByText('sb-main')).toBeTruthy();
		expect(screen.getAllByText('125ms').length).toBeGreaterThan(0);
		expect(loadHistory).toHaveBeenCalledWith('sb-main');
	});

	it('keeps AWG watchdog cards alongside sing-box cards', async () => {
		setPingStatuses([
			{
				tunnelId: 'awg-1',
				tunnelName: 'AWG Alpha',
				enabled: true,
				backend: 'kernel',
				status: 'alive',
				method: 'icmp',
				lastLatency: 42,
				failCount: 0,
				failThreshold: 3,
				restartCount: 0,
			},
		]);
		getTunnelsAll.mockResolvedValue({ tunnels: [sampleAwgMeta] });
		setSingboxStatus({ ...baseSingboxStatus, tunnelCount: 1 });
		setSingboxTunnels([sampleSingboxTunnel]);
		setSubscriptions([]);
		singboxDelayHistoryStore.set(new Map([['sb-main', [125]]]));

		render(WatchdogMonitoringMode);

		await waitFor(() => {
			expect(screen.getByText('AWG Alpha')).toBeTruthy();
		});
		expect(screen.getByRole('button', { name: /AWG \/ NativeWG/i })).toBeTruthy();
		expect(screen.getByRole('button', { name: /Sing-box/i })).toBeTruthy();
		expect(screen.getByText('sb-main')).toBeTruthy();
	});

	it('shows loading notice while sing-box data is still loading', async () => {
		setSingboxStatus(null, 'loading');
		setSingboxTunnels(null, 'loading');
		setSubscriptions(null, 'loading');

		render(WatchdogMonitoringMode);

		expect(await screen.findByText('Загрузка данных Sing-box…')).toBeTruthy();
	});

	it('shows error notice when sing-box status fails to load', async () => {
		setSingboxStatus(null, 'error');
		setSingboxTunnels(null, 'idle');
		setSubscriptions(null, 'idle');

		render(WatchdogMonitoringMode);

		expect(await screen.findByText('Не удалось загрузить Sing-box.')).toBeTruthy();
	});

	it('shows not installed notice', async () => {
		setSingboxStatus({
			...baseSingboxStatus,
			installed: false,
			running: false,
			tunnelCount: 0,
		});
		setSingboxTunnels([]);
		setSubscriptions([]);

		render(WatchdogMonitoringMode);

		expect(await screen.findByText('Sing-box не установлен.')).toBeTruthy();
	});

	it('shows active subscription member with selector delay check and member-history fallback', async () => {
		setSingboxStatus({ ...baseSingboxStatus, tunnelCount: 0 });
		setSingboxTunnels([]);
		setSubscriptions([sampleSubscription]);
		singboxDelayHistoryStore.set(new Map([['member-1', [155]]]));

		render(WatchdogMonitoringMode);

		expect(await screen.findByRole('button', { name: /Sing-box/i })).toBeTruthy();
		expect(screen.getByText('America Pool')).toBeTruthy();
		expect(screen.getByText('Подписка · URLTest')).toBeTruthy();
		expect(screen.getAllByText('155ms').length).toBeGreaterThan(0);
		expect(loadHistory).toHaveBeenCalledWith('member-1');

		const card = screen.getByLabelText(/Subscription watchdog America Pool/);
		await fireEvent.click(within(card).getByRole('button', { name: 'Проверить' }));
		expect(singboxDelayCheck).toHaveBeenCalledWith('iq0');
	});

	it('disables check button while delay check is pending and sends only one request', async () => {
		let resolveCheck = () => {};
		singboxDelayCheck.mockReturnValue(
			new Promise<void>((resolve) => {
				resolveCheck = resolve;
			}),
		);

		setSingboxStatus({ ...baseSingboxStatus, tunnelCount: 1 });
		setSingboxTunnels([sampleSingboxTunnel]);
		setSubscriptions([]);

		render(WatchdogMonitoringMode);

		const button = await screen.findByRole('button', { name: 'Проверить' });
		await fireEvent.click(button);
		await fireEvent.click(button);

		expect(singboxDelayCheck).toHaveBeenCalledTimes(1);
		expect((button as HTMLButtonElement).disabled).toBe(true);

		resolveCheck();
		await waitFor(() => {
			expect((button as HTMLButtonElement).disabled).toBe(false);
		});
	});

	it('hides sing-box block below singbox usage level', async () => {
		usageLevelStore.set('basic');
		setSingboxStatus({ ...baseSingboxStatus, tunnelCount: 1 });
		setSingboxTunnels([sampleSingboxTunnel]);
		setSubscriptions([]);

		render(WatchdogMonitoringMode);

		expect(screen.queryByText(/Sing-box/)).toBeNull();
	});

	it('collapses AWG spoiler and hides AWG cards', async () => {
		setPingStatuses([
			{
				tunnelId: 'awg-1',
				tunnelName: 'AWG Alpha',
				enabled: true,
				backend: 'kernel',
				status: 'alive',
				method: 'icmp',
				lastLatency: 42,
				failCount: 0,
				failThreshold: 3,
				restartCount: 0,
			},
		]);
		getTunnelsAll.mockResolvedValue({ tunnels: [sampleAwgMeta] });

		render(WatchdogMonitoringMode);

		const awgToggle = await screen.findByRole('button', { name: /AWG \/ NativeWG/i });
		expect(screen.getByText('AWG Alpha')).toBeTruthy();

		await fireEvent.click(awgToggle);

		expect(awgToggle.getAttribute('aria-expanded')).toBe('false');
		expect(screen.queryByText('AWG Alpha')).toBeNull();
	});

	it('collapses sing-box spoiler and hides sing-box content', async () => {
		setSingboxStatus({ ...baseSingboxStatus, tunnelCount: 1 });
		setSingboxTunnels([sampleSingboxTunnel]);
		setSubscriptions([]);

		render(WatchdogMonitoringMode);

		const singboxToggle = await screen.findByRole('button', { name: /Sing-box/i });
		expect(await screen.findByText('sb-main')).toBeTruthy();

		await fireEvent.click(singboxToggle);

		expect(singboxToggle.getAttribute('aria-expanded')).toBe('false');
		expect(screen.queryByText('sb-main')).toBeNull();
	});

	it('persists collapsed sing-box spoiler state in localStorage', async () => {
		setSingboxStatus({ ...baseSingboxStatus, tunnelCount: 1 });
		setSingboxTunnels([sampleSingboxTunnel]);
		setSubscriptions([]);

		const firstRender = render(WatchdogMonitoringMode);
		const singboxToggle = await screen.findByRole('button', { name: /Sing-box/i });
		await screen.findByText('sb-main');

		await fireEvent.click(singboxToggle);

		expect(localStorage.getItem('watchdog_monitoring_sections_open_v1')).toContain('"singbox":false');

		firstRender.unmount();

		render(WatchdogMonitoringMode);

		const remountedToggle = await screen.findByRole('button', { name: /Sing-box/i });
		expect(remountedToggle.getAttribute('aria-expanded')).toBe('false');
		expect(screen.queryByText('sb-main')).toBeNull();
	});

	it('keeps AWG spoiler open by default when AWG cards appear after async tunnel snapshot load', async () => {
		setPingStatuses([]);
		getTunnelsAll.mockResolvedValue({ tunnels: [sampleAwgMeta] });
		setSingboxStatus(null, 'idle');
		setSingboxTunnels(null, 'idle');
		setSubscriptions(null, 'idle');

		render(WatchdogMonitoringMode);

		const awgToggle = await screen.findByRole('button', { name: /AWG \/ NativeWG/i });

		await waitFor(() => {
			expect(screen.getByText('AWG Alpha')).toBeTruthy();
		});

		expect(awgToggle.getAttribute('aria-expanded')).toBe('true');
	});

	it('does not overwrite stored sing-box spoiler preference while sing-box section is unavailable', async () => {
		localStorage.setItem(
			'watchdog_monitoring_sections_open_v1',
			JSON.stringify({
				awg: true,
				singbox: true,
			}),
		);

		usageLevelStore.set('basic');
		setSingboxStatus({ ...baseSingboxStatus, tunnelCount: 1 });
		setSingboxTunnels([sampleSingboxTunnel]);
		setSubscriptions([]);

		render(WatchdogMonitoringMode);

		await waitFor(() => {
			expect(localStorage.getItem('watchdog_monitoring_sections_open_v1')).toContain('"singbox":true');
		});

		expect(screen.queryByRole('button', { name: /Sing-box/i })).toBeNull();
	});

	it('runs initial auto-check once for running sing-box cards', async () => {
		vi.useFakeTimers();
		setSingboxStatus({ ...baseSingboxStatus, tunnelCount: 1 });
		setSingboxTunnels([sampleSingboxTunnel]);
		setSubscriptions([]);

		render(WatchdogMonitoringMode);

		await vi.runAllTimersAsync();

		expect(singboxDelayCheck).toHaveBeenCalledWith('sb-main');
		vi.useRealTimers();
	});

	it('deduplicates initial auto-check for cards with the same delayCheckTag', async () => {
		vi.useFakeTimers();

		const rawSelectorTunnel: SingboxTunnel = {
			...sampleSingboxTunnel,
			tag: 'iq0',
			proxyInterface: 'proxy-iq0',
			kernelInterface: 'tun-iq0',
		};

		setSingboxStatus({ ...baseSingboxStatus, tunnelCount: 1 });
		setSingboxTunnels([rawSelectorTunnel]);
		setSubscriptions([sampleSubscription]);

		singboxDelayCheck.mockClear();

		render(WatchdogMonitoringMode);

		await vi.advanceTimersByTimeAsync(1000);

		const iq0Calls = singboxDelayCheck.mock.calls.filter(([tag]) => tag === 'iq0');
		expect(iq0Calls).toHaveLength(1);

		vi.useRealTimers();
	});
});
