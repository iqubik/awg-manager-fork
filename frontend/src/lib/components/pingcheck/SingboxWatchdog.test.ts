import { fireEvent, render, screen } from '@testing-library/svelte';
import { describe, expect, it, vi, beforeEach } from 'vitest';

const {
	singboxWatchdogConfigure,
	singboxDelayCheck,
	loadHistory,
	getTrafficRates,
	subscribeTraffic,
	notificationsSuccess,
	notificationsError,
	singboxDelayHistoryStore,
	goto,
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
		singboxWatchdogConfigure: vi.fn(),
		singboxDelayCheck: vi.fn(),
		loadHistory: vi.fn(),
		getTrafficRates: vi.fn(() => ({ rx: [10, 20], tx: [5, 8] })),
		subscribeTraffic: vi.fn((run: () => void) => {
			run();
			return () => {};
		}),
		notificationsSuccess: vi.fn(),
		notificationsError: vi.fn(),
		singboxDelayHistoryStore: writable(new Map()),
		goto: vi.fn(),
	};
});

vi.mock('$lib/api/client', () => ({
	api: {
		singboxWatchdogConfigure,
		singboxDelayCheck,
	},
}));

vi.mock('$lib/stores/notifications', () => ({
	notifications: {
		success: notificationsSuccess,
		error: notificationsError,
	},
}));

vi.mock('$lib/stores/traffic', () => ({
	loadHistory,
	getTrafficRates,
	subscribeTraffic,
}));

vi.mock('$lib/stores/singbox', () => ({
	singboxDelayHistory: {
		subscribe: singboxDelayHistoryStore.subscribe,
	},
}));

vi.mock('$app/navigation', () => ({
	goto,
}));

const { default: SingboxWatchdogSettingsDrawer } = await import('./SingboxWatchdogSettingsDrawer.svelte');
const { default: SingboxWatchdogCard } = await import('./SingboxWatchdogCard.svelte');

const baseTarget = {
	id: 'tunnel:sb-main',
	kind: 'tunnel',
	ref: 'sb-main',
	name: 'sb-main',
	checkTag: 'sb-main',
	trafficTag: 'sb-main',
	running: true,
	configured: true,
	enabled: true,
	status: 'alive',
	interval: 30,
	timeout: 5,
	lastLatency: 90,
	failCount: 0,
	failThreshold: 3,
	restartCount: 0,
	switchCount: 0,
	recoveryMode: 'off',
} as const;

const baseCard = {
	id: 'tunnel:sb-main',
	name: 'sb-main',
	routeHref: '/singbox/sb-main',
	source: 'tunnel' as const,
	sourceLabel: 'Sing-box',
	tag: 'sb-main',
	delayCheckTag: 'sb-main',
	primaryHistoryTag: 'sb-main',
	fallbackHistoryTag: undefined,
	trafficTag: 'sb-main',
	protocol: 'vless',
	security: 'reality',
	transport: 'tcp',
	proxyInterface: 'proxy0',
	kernelInterface: 'tun0',
	running: true,
	configured: true,
	enabled: true,
	statusKind: 'alive' as const,
	failCount: 0,
	failThreshold: 3,
	restartCount: 0,
	switchCount: 0,
	lastError: undefined,
	lastRecovery: undefined,
	status: baseTarget,
	logs: [],
};

describe('SingboxWatchdog UI', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		singboxWatchdogConfigure.mockResolvedValue(undefined);
		singboxDelayCheck.mockResolvedValue(undefined);
		singboxDelayHistoryStore.set(new Map([['sb-main', [90, 95]]]));
	});

	it('raw tunnel drawer defaults to recovery off', () => {
		render(SingboxWatchdogSettingsDrawer, {
			props: {
				open: true,
				target: { ...baseTarget, recoveryMode: undefined as never },
				onclose: () => {},
				onSaved: () => {},
			},
		});

		const select = screen.getByLabelText('Режим восстановления') as HTMLSelectElement;
		expect(select.value).toBe('off');
	});

	it('subscription drawer defaults to switch-member', () => {
		render(SingboxWatchdogSettingsDrawer, {
			props: {
				open: true,
				target: {
					...baseTarget,
					id: 'subscription:sub-1',
					kind: 'subscription',
					ref: 'sub-1',
					name: 'Pool',
					recoveryMode: undefined as never,
				},
				onclose: () => {},
				onSaved: () => {},
			},
		});

		const select = screen.getByLabelText('Режим восстановления') as HTMLSelectElement;
		expect(select.value).toBe('switch-member');
	});

	it('selecting restart-singbox shows dangerous warning and blocks save until confirmed', async () => {
		render(SingboxWatchdogSettingsDrawer, {
			props: {
				open: true,
				target: baseTarget,
				onclose: () => {},
				onSaved: () => {},
			},
		});

		const select = screen.getByLabelText('Режим восстановления');
		await fireEvent.change(select, { target: { value: 'restart-singbox' } });

		expect(
			screen.getByText(/Перезапускает весь sing-box, может кратковременно оборвать все Sing-box туннели и подписки/i),
		).toBeTruthy();

		const saveButton = screen.getByRole('button', { name: 'Сохранить' }) as HTMLButtonElement;
		expect(saveButton.disabled).toBe(true);

		await fireEvent.click(
			screen.getByLabelText(/Я понимаю, что это перезапускает весь sing-box/i),
		);
		expect(saveButton.disabled).toBe(false);
	});

	it('watchdog button uses callback, not delay check api', async () => {
		const onCheckNow = vi.fn().mockResolvedValue(undefined);
		render(SingboxWatchdogCard, {
			props: {
				card: baseCard,
				onConfigure: () => {},
				onCheckNow,
				onDisable: () => {},
				onEnable: () => {},
			},
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Проверка watchdog' }));
		expect(onCheckNow).toHaveBeenCalledTimes(1);
		expect(singboxDelayCheck).not.toHaveBeenCalled();
	});
});
