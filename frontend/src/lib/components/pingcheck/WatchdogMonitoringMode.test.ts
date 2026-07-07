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
	singboxWatchdogCheckNow,
	singboxWatchdogEnable,
	singboxWatchdogDisable,
	singboxWatchdogLogs,
	loadPingLogs,
	goto,
	loadHistory,
	getTrafficRates,
	subscribeTraffic,
	pingStatusStore,
	pingLogsStore,
	singboxStatusStore,
	singboxTunnelsStore,
	singboxWatchdogStore,
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
		singboxWatchdogCheckNow: vi.fn(),
		singboxWatchdogEnable: vi.fn(),
		singboxWatchdogDisable: vi.fn(),
		singboxWatchdogLogs: vi.fn(),
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
		singboxWatchdogStore: writable<any>({
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
		singboxWatchdogCheckNow,
		singboxWatchdogEnable,
		singboxWatchdogDisable,
		singboxWatchdogLogs,
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

vi.mock('$lib/stores/singboxWatchdog', () => ({
	singboxWatchdogStatus: {
		subscribe: singboxWatchdogStore.subscribe,
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

function setWatchdogStatuses(data: any[] | null, status: 'idle' | 'loading' | 'fresh' | 'stale' | 'error' = 'fresh'): void {
	singboxWatchdogStore.set({
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

const sampleWatchdogTunnel = {
	id: 'tunnel:sb-main',
	kind: 'tunnel',
	ref: 'sb-main',
	name: 'sb-main',
	checkTag: 'sb-main',
	trafficTag: 'sb-main',
	protocol: 'vless',
	security: 'reality',
	transport: 'tcp',
	proxyInterface: 'proxy0',
	kernelInterface: 'tun0',
	running: true,
	configured: true,
	enabled: true,
	status: 'alive',
	lastLatency: 125,
	failCount: 0,
	failThreshold: 3,
	restartCount: 1,
	switchCount: 0,
	recoveryMode: 'restart-singbox',
};

const sampleWatchdogSubscription = {
	id: 'subscription:sub-1',
	kind: 'subscription',
	ref: 'sub-1',
	name: 'America Pool',
	checkTag: 'iq0',
	trafficTag: 'member-1',
	selectorTag: 'iq0',
	activeMemberTag: 'member-1',
	protocol: 'vless',
	security: 'reality',
	transport: 'tcp',
	proxyInterface: 'Proxy5',
	kernelInterface: 't2s5',
	running: true,
	configured: true,
	enabled: true,
	status: 'alive',
	lastLatency: 155,
	failCount: 0,
	failThreshold: 3,
	restartCount: 0,
	switchCount: 2,
	recoveryMode: 'switch-member',
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
		singboxWatchdogCheckNow.mockResolvedValue(undefined);
		singboxWatchdogEnable.mockResolvedValue(undefined);
		singboxWatchdogDisable.mockResolvedValue(undefined);
		singboxWatchdogLogs.mockResolvedValue([]);
		setPingStatuses([]);
		pingLogsStore.set([]);
		setSingboxStatus(null, 'idle');
		setSingboxTunnels(null, 'idle');
		setWatchdogStatuses(null, 'idle');
		singboxDelayHistoryStore.set(new Map());
		usageLevelStore.set('advanced');
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
		setWatchdogStatuses([sampleWatchdogTunnel]);
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
		setWatchdogStatuses(null, 'loading');

		render(WatchdogMonitoringMode);

		expect(await screen.findByText('Загрузка данных Sing-box…')).toBeTruthy();
	});

	it('shows error notice when sing-box status fails to load', async () => {
		setSingboxStatus(null, 'error');
		setSingboxTunnels(null, 'idle');
		setWatchdogStatuses(null, 'idle');

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
		setWatchdogStatuses([]);

		render(WatchdogMonitoringMode);

		expect(await screen.findByText('Sing-box не установлен.')).toBeTruthy();
	});

	it('shows subscription watchdog card from backend status', async () => {
		setSingboxStatus({ ...baseSingboxStatus, tunnelCount: 0 });
		setWatchdogStatuses([sampleWatchdogSubscription]);
		singboxDelayHistoryStore.set(new Map([['member-1', [155]]]));

		render(WatchdogMonitoringMode);

		expect(await screen.findByRole('button', { name: /Sing-box/i })).toBeTruthy();
		expect(screen.getByText('America Pool')).toBeTruthy();
		expect(screen.getByText('Подписка')).toBeTruthy();
		expect(screen.getAllByText('155ms').length).toBeGreaterThan(0);
		expect(loadHistory).toHaveBeenCalledWith('member-1');

		const card = screen.getByLabelText(/Subscription watchdog America Pool/);
		await fireEvent.click(within(card).getByRole('button', { name: 'Проверка watchdog' }));
		expect(singboxWatchdogCheckNow).toHaveBeenCalledWith('subscription:sub-1');
	});

	it('loads sing-box watchdog logs with one shared request for multiple cards', async () => {
		setSingboxStatus({ ...baseSingboxStatus, tunnelCount: 3 });
		setWatchdogStatuses([
			sampleWatchdogTunnel,
			{ ...sampleWatchdogSubscription },
			{ ...sampleWatchdogTunnel, id: 'tunnel:sb-alt', ref: 'sb-alt', name: 'sb-alt', checkTag: 'sb-alt', trafficTag: 'sb-alt' },
		]);
		singboxWatchdogLogs.mockResolvedValue([
			{ targetId: 'tunnel:sb-main', timestamp: new Date().toISOString() },
			{ targetId: 'subscription:sub-1', timestamp: new Date().toISOString() },
		]);

		render(WatchdogMonitoringMode);

		await screen.findByText('sb-main');
		await screen.findByText('America Pool');
		await screen.findByText('sb-alt');

		await waitFor(() => {
			expect(singboxWatchdogLogs).toHaveBeenCalledTimes(1);
		});
		expect(singboxWatchdogLogs).toHaveBeenCalledWith();
	});

	it('hides sing-box block below singbox usage level', async () => {
		usageLevelStore.set('basic');
		setSingboxStatus({ ...baseSingboxStatus, tunnelCount: 1 });
		setWatchdogStatuses([sampleWatchdogTunnel]);

		render(WatchdogMonitoringMode);

		expect(screen.queryByText(/Sing-box/)).toBeNull();
	});

	it('persists collapsed sing-box spoiler state in localStorage', async () => {
		setSingboxStatus({ ...baseSingboxStatus, tunnelCount: 1 });
		setWatchdogStatuses([sampleWatchdogTunnel]);

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
		setWatchdogStatuses(null, 'idle');

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
		setWatchdogStatuses([sampleWatchdogTunnel]);

		render(WatchdogMonitoringMode);

		await waitFor(() => {
			expect(localStorage.getItem('watchdog_monitoring_sections_open_v1')).toContain('"singbox":true');
		});

		expect(screen.queryByRole('button', { name: /Sing-box/i })).toBeNull();
	});

	it('enables and disables sing-box watchdog via backend actions', async () => {
		setSingboxStatus({ ...baseSingboxStatus, tunnelCount: 1 });
		setWatchdogStatuses([{ ...sampleWatchdogTunnel, enabled: false, status: 'disabled' }]);

		render(WatchdogMonitoringMode);

		const enableButton = await screen.findByRole('button', { name: 'Включить' });
		await fireEvent.click(enableButton);
		expect(singboxWatchdogEnable).toHaveBeenCalledWith('tunnel:sb-main');

		setWatchdogStatuses([sampleWatchdogTunnel]);
		await waitFor(() => {
			expect(screen.getByRole('button', { name: 'Выключить' })).toBeTruthy();
		});
		await fireEvent.click(screen.getByRole('button', { name: 'Выключить' }));
		expect(singboxWatchdogDisable).toHaveBeenCalledWith('tunnel:sb-main');
	});
});
