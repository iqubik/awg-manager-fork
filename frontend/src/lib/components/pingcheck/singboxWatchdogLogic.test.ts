import { describe, expect, it } from 'vitest';
import type { SingboxWatchdogStatus } from '$lib/types';
import {
	buildSingboxWatchdogPayload,
	canSaveSingboxWatchdog,
	createSingboxWatchdogDrawerState,
	isDangerousSingboxRecovery,
} from './singboxWatchdogLogic';

const baseTarget: SingboxWatchdogStatus = {
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
};

describe('singboxWatchdogLogic', () => {
	it('defaults raw tunnel recovery to off', () => {
		const target = {
			...baseTarget,
			recoveryMode: undefined,
		} as unknown as SingboxWatchdogStatus;

		expect(
			createSingboxWatchdogDrawerState(target),
		).toMatchObject({
			recoveryMode: 'off',
			persistSwitch: false,
		});
	});

	it('defaults subscription recovery to switch-member', () => {
		const target = {
			...baseTarget,
			id: 'subscription:sub-1',
			kind: 'subscription',
			ref: 'sub-1',
			name: 'Pool',
			recoveryMode: undefined,
			persistSwitch: true,
		} as unknown as SingboxWatchdogStatus;

		expect(
			createSingboxWatchdogDrawerState(target),
		).toMatchObject({
			recoveryMode: 'switch-member',
			persistSwitch: true,
		});
	});

	it('requires explicit confirmation for dangerous restart-singbox mode', () => {
		const dangerous = isDangerousSingboxRecovery(baseTarget, 'restart-singbox');
		expect(dangerous).toBe(true);
		expect(
			canSaveSingboxWatchdog({
				saving: false,
				restartIsDangerous: dangerous,
				confirmDangerousRestart: false,
			}),
		).toBe(false);
		expect(
			canSaveSingboxWatchdog({
				saving: false,
				restartIsDangerous: dangerous,
				confirmDangerousRestart: true,
			}),
		).toBe(true);
	});

	it('builds bounded payload values and only persists switch for subscriptions', () => {
		expect(
			buildSingboxWatchdogPayload({
				target: baseTarget,
				interval: 1,
				failThreshold: 50,
				timeout: 0,
				recoveryMode: 'off',
				persistSwitch: true,
			}),
		).toMatchObject({
			interval: 5,
			failThreshold: 20,
			timeout: 5,
			persistSwitch: false,
		});
	});
});
