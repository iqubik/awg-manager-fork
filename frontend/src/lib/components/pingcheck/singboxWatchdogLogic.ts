import type {
	SingboxWatchdogConfig,
	SingboxWatchdogRecoveryMode,
	SingboxWatchdogStatus,
} from '$lib/types';

export interface SingboxWatchdogDrawerState {
	interval: number;
	failThreshold: number;
	timeout: number;
	recoveryMode: SingboxWatchdogRecoveryMode;
	persistSwitch: boolean;
	confirmDangerousRestart: boolean;
}

export function createSingboxWatchdogDrawerState(
	target: SingboxWatchdogStatus | null,
): SingboxWatchdogDrawerState {
	if (!target) {
		return {
			interval: 30,
			failThreshold: 3,
			timeout: 5,
			recoveryMode: 'off',
			persistSwitch: false,
			confirmDangerousRestart: false,
		};
	}

	return {
		interval: Math.min(3600, Math.max(5, target.interval || 30)),
		failThreshold: Math.min(20, Math.max(1, target.failThreshold || 3)),
		timeout: Math.min(30, Math.max(1, target.timeout || 5)),
		recoveryMode: target.recoveryMode || (target.kind === 'subscription' ? 'switch-member' : 'off'),
		persistSwitch: target.kind === 'subscription' && target.persistSwitch === true,
		confirmDangerousRestart: false,
	};
}

export function isDangerousSingboxRecovery(
	target: SingboxWatchdogStatus | null,
	recoveryMode: SingboxWatchdogRecoveryMode,
): boolean {
	return target?.kind === 'tunnel' && recoveryMode === 'restart-singbox';
}

export function canSaveSingboxWatchdog(options: {
	saving: boolean;
	restartIsDangerous: boolean;
	confirmDangerousRestart: boolean;
}): boolean {
	return !options.saving && (!options.restartIsDangerous || options.confirmDangerousRestart);
}

export function buildSingboxWatchdogPayload(options: {
	target: SingboxWatchdogStatus;
	interval: number;
	failThreshold: number;
	timeout: number;
	recoveryMode: SingboxWatchdogRecoveryMode;
	persistSwitch: boolean;
}): SingboxWatchdogConfig {
	return {
		id: options.target.id,
		kind: options.target.kind,
		ref: options.target.ref,
		enabled: true,
		interval: Math.min(3600, Math.max(5, Number(options.interval) || 30)),
		failThreshold: Math.min(20, Math.max(1, Number(options.failThreshold) || 3)),
		timeout: Math.min(30, Math.max(1, Number(options.timeout) || 5)),
		recoveryMode: options.recoveryMode,
		persistSwitch: options.target.kind === 'subscription' ? options.persistSwitch : false,
	};
}
