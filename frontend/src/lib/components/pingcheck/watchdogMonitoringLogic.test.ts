import { describe, expect, it } from 'vitest';
import type {
	SingboxStatus,
	SingboxTunnel,
	SingboxWatchdogLogEntry,
	SingboxWatchdogStatus,
} from '$lib/types';
import {
	buildSingboxWatchdogCards,
	deriveSingboxBlockState,
	groupSingboxWatchdogLogs,
	isWatchdogSectionAvailable,
	isWatchdogSectionOpen,
	normalizeWatchdogOpenSections,
	resolveSingboxTrafficTag,
} from './watchdogMonitoringLogic';

const baseSingboxStatus: SingboxStatus = {
	installed: true,
	running: true,
	tunnelCount: 1,
	proxyComponent: true,
	ndmsProxyEnabled: true,
	requiredVersion: '1.10.0',
	updateAvailable: false,
};

const baseWatchdogStatus: SingboxWatchdogStatus = {
	id: 'subscription:sub-1',
	kind: 'subscription',
	ref: 'sub-1',
	name: 'America Pool',
	checkTag: 'iq0',
	trafficTag: '',
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
	interval: 60,
	timeout: 5,
	lastLatency: 155,
	failCount: 0,
	failThreshold: 3,
	restartCount: 0,
	switchCount: 2,
	recoveryMode: 'switch-member',
};

describe('watchdogMonitoringLogic', () => {
	it('normalizes watchdog spoiler preferences with sane defaults', () => {
		expect(normalizeWatchdogOpenSections(null)).toEqual({
			awg: true,
			singbox: true,
		});
		expect(normalizeWatchdogOpenSections({ awg: false })).toEqual({
			awg: false,
			singbox: true,
		});
	});

	it('groups sing-box watchdog logs by target id and skips orphaned entries', () => {
		const grouped = groupSingboxWatchdogLogs([
			{
				targetId: 'subscription:sub-1',
				timestamp: '2026-07-07T10:00:00Z',
				targetName: 'America Pool',
				kind: 'subscription',
				checkTag: 'iq0',
				success: true,
				latency: 155,
				error: '',
				failCount: 0,
				threshold: 3,
				stateChange: '',
			},
			{
				targetId: 'tunnel:sb-main',
				timestamp: '2026-07-07T10:01:00Z',
				targetName: 'sb-main',
				kind: 'tunnel',
				checkTag: 'sb-main',
				success: false,
				latency: 0,
				error: 'timeout',
				failCount: 1,
				threshold: 3,
				stateChange: 'recovering',
			},
			{
				...({
					targetId: '',
					timestamp: '2026-07-07T10:02:00Z',
					targetName: 'ignored',
					kind: 'tunnel',
					checkTag: 'ignored',
					success: false,
					latency: 0,
					error: 'timeout',
					failCount: 1,
					threshold: 3,
					stateChange: '',
				} satisfies SingboxWatchdogLogEntry),
			},
		]);

		expect(Object.keys(grouped)).toEqual(['subscription:sub-1', 'tunnel:sb-main']);
		expect(grouped['subscription:sub-1']).toHaveLength(1);
		expect(grouped['tunnel:sb-main'][0]?.error).toBe('timeout');
	});

	it('builds sing-box watchdog cards for subscriptions with fallback member history and logs', () => {
		const cards = buildSingboxWatchdogCards({
			statuses: [baseWatchdogStatus],
			logsByTargetId: {
				'subscription:sub-1': [
					{
						targetId: 'subscription:sub-1',
						timestamp: '2026-07-07T10:00:00Z',
						targetName: 'America Pool',
						kind: 'subscription',
						checkTag: 'iq0',
						success: true,
						latency: 155,
						error: '',
						failCount: 0,
						threshold: 3,
						stateChange: '',
					},
				],
			},
		});

		expect(cards).toHaveLength(1);
		expect(cards[0]).toMatchObject({
			id: 'subscription:sub-1',
			source: 'subscription',
			sourceLabel: 'Подписка',
			routeHref: '/subscriptions/sub-1',
			delayCheckTag: 'iq0',
			primaryHistoryTag: 'iq0',
			fallbackHistoryTag: 'member-1',
			tag: 'member-1',
		});
		expect(cards[0]?.logs).toHaveLength(1);
	});

	it('derives sing-box loading and error states from store snapshots', () => {
		expect(
			deriveSingboxBlockState({
				singboxSectionVisible: true,
				singboxState: { data: null, status: 'loading' },
				singboxWatchdogState: { data: null, status: 'idle' },
			}),
		).toMatchObject({
			installed: false,
			loading: true,
			errorMessage: '',
		});

		expect(
			deriveSingboxBlockState({
				singboxSectionVisible: true,
				singboxState: { data: baseSingboxStatus, status: 'fresh' },
				singboxWatchdogState: { data: null, status: 'error' },
			}),
		).toMatchObject({
			installed: true,
			loading: false,
			errorMessage: 'Не удалось загрузить Sing-box Watchdog.',
		});
	});

	it('resolves section availability and open state without mutating stored preference', () => {
		const prefs = normalizeWatchdogOpenSections({ awg: false, singbox: true });

		expect(
			isWatchdogSectionAvailable('awg', {
				awgCardCount: 0,
				singboxSectionVisible: true,
			}),
		).toBe(false);
		expect(
			isWatchdogSectionOpen('singbox', prefs, {
				awgCardCount: 0,
				singboxSectionVisible: true,
			}),
		).toBe(true);
		expect(
			isWatchdogSectionOpen('awg', prefs, {
				awgCardCount: 0,
				singboxSectionVisible: true,
			}),
		).toBe(false);
	});

	it('resolves traffic tag for subscription/member history and raw tunnel fallback', () => {
		const tunnels: SingboxTunnel[] = [
			{
				tag: 'sb-main',
				protocol: 'vless',
				server: '1.2.3.4',
				port: 443,
				security: 'reality',
				transport: 'tcp',
				listenPort: 1080,
				proxyInterface: 'proxy0',
				connectivity: { connected: true, latency: 125 },
				running: true,
			},
		];

		expect(resolveSingboxTrafficTag(baseWatchdogStatus, tunnels)).toBe('member-1');
		expect(
			resolveSingboxTrafficTag(
				{
					...baseWatchdogStatus,
					id: 'tunnel:sb-main',
					kind: 'tunnel',
					ref: 'sb-main',
					name: 'sb-main',
					checkTag: 'sb-main',
					activeMemberTag: undefined,
				},
				tunnels,
			),
		).toBe('sb-main');
	});
});
