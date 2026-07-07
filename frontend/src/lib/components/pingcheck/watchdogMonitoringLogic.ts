import type {
	SingboxStatus,
	SingboxTunnel,
	SingboxWatchdogLogEntry,
	SingboxWatchdogStatus,
} from '$lib/types';

export type WatchdogSectionId = 'awg' | 'singbox';

export interface WatchdogSectionOpenState {
	awg: boolean;
	singbox: boolean;
}

export interface SingboxStoreLike<T> {
	data: T | null;
	status: 'idle' | 'loading' | 'fresh' | 'stale' | 'error';
}

export interface SingboxBlockState {
	installed: boolean;
	loading: boolean;
	errorMessage: string;
	watchdogList: SingboxWatchdogStatus[];
}

export interface WatchdogSingboxCardModel {
	id: string;
	name: string;
	routeHref: string;
	source: 'tunnel' | 'subscription';
	sourceLabel: string;
	tag: string;
	delayCheckTag: string;
	primaryHistoryTag: string;
	fallbackHistoryTag?: string;
	trafficTag: string;
	protocol?: string;
	security?: string;
	transport?: string;
	proxyInterface?: string;
	kernelInterface?: string;
	running: boolean;
	configured: boolean;
	enabled: boolean;
	statusKind: SingboxWatchdogStatus['status'];
	failCount: number;
	failThreshold: number;
	restartCount: number;
	switchCount: number;
	lastError?: string;
	lastRecovery?: string;
	status: SingboxWatchdogStatus;
	logs: SingboxWatchdogLogEntry[];
}

export function normalizeWatchdogOpenSections(
	value: Partial<Record<WatchdogSectionId, boolean>> | null | undefined,
): WatchdogSectionOpenState {
	return {
		awg: value?.awg ?? true,
		singbox: value?.singbox ?? true,
	};
}

export function isWatchdogSectionAvailable(
	id: WatchdogSectionId,
	options: {
		awgCardCount: number;
		singboxSectionVisible: boolean;
	},
): boolean {
	return id === 'awg' ? options.awgCardCount > 0 : options.singboxSectionVisible;
}

export function isWatchdogSectionOpen(
	id: WatchdogSectionId,
	openSections: WatchdogSectionOpenState,
	options: {
		awgCardCount: number;
		singboxSectionVisible: boolean;
	},
): boolean {
	return isWatchdogSectionAvailable(id, options) && openSections[id];
}

export function groupSingboxWatchdogLogs(
	logs: SingboxWatchdogLogEntry[],
): Record<string, SingboxWatchdogLogEntry[]> {
	const grouped: Record<string, SingboxWatchdogLogEntry[]> = {};
	for (const entry of logs) {
		if (!entry?.targetId) continue;
		(grouped[entry.targetId] ??= []).push(entry);
	}
	return grouped;
}

export function deriveSingboxBlockState(options: {
	singboxSectionVisible: boolean;
	singboxState: SingboxStoreLike<SingboxStatus>;
	singboxWatchdogState: SingboxStoreLike<SingboxWatchdogStatus[]>;
}): SingboxBlockState {
	const { singboxSectionVisible, singboxState, singboxWatchdogState } = options;
	const installed = singboxState.data?.installed === true;
	const watchdogList = Array.isArray(singboxWatchdogState.data) ? singboxWatchdogState.data : [];

	if (!singboxSectionVisible) {
		return {
			installed,
			loading: false,
			errorMessage: '',
			watchdogList,
		};
	}

	const loading =
		(!singboxState.data && (singboxState.status === 'idle' || singboxState.status === 'loading')) ||
		(installed &&
			!singboxWatchdogState.data &&
			(singboxWatchdogState.status === 'idle' || singboxWatchdogState.status === 'loading'));

	let errorMessage = '';
	if (!singboxState.data && singboxState.status === 'error') {
		errorMessage = 'Не удалось загрузить Sing-box.';
	} else if (installed && !singboxWatchdogState.data && singboxWatchdogState.status === 'error') {
		errorMessage = 'Не удалось загрузить Sing-box Watchdog.';
	}

	return {
		installed,
		loading,
		errorMessage,
		watchdogList,
	};
}

export function buildSingboxWatchdogCards(options: {
	statuses: SingboxWatchdogStatus[];
	logsByTargetId?: Record<string, SingboxWatchdogLogEntry[]>;
}): WatchdogSingboxCardModel[] {
	const { statuses, logsByTargetId = {} } = options;
	return statuses.map((status) => ({
		id: status.id,
		name: status.name,
		routeHref:
			status.kind === 'subscription'
				? `/subscriptions/${encodeURIComponent(status.ref)}`
				: `/singbox/${encodeURIComponent(status.ref)}`,
		source: status.kind,
		sourceLabel: status.kind === 'subscription' ? 'Подписка' : 'Sing-box',
		tag: status.activeMemberTag || status.checkTag,
		delayCheckTag: status.checkTag,
		primaryHistoryTag: status.checkTag,
		fallbackHistoryTag: status.activeMemberTag || undefined,
		trafficTag: status.trafficTag,
		protocol: status.protocol,
		security: status.security,
		transport: status.transport,
		proxyInterface: status.proxyInterface,
		kernelInterface: status.kernelInterface,
		running: status.running,
		configured: status.configured,
		enabled: status.enabled,
		statusKind: status.status,
		failCount: status.failCount,
		failThreshold: status.failThreshold,
		restartCount: status.restartCount,
		switchCount: status.switchCount,
		lastError: status.lastError,
		lastRecovery: status.lastRecovery,
		status,
		logs: logsByTargetId[status.id] ?? [],
	}));
}

export function resolveSingboxTrafficTag(
	status: SingboxWatchdogStatus,
	tunnels: SingboxTunnel[] | null | undefined,
): string {
	if (status.trafficTag) return status.trafficTag;
	if (status.kind === 'subscription' && status.activeMemberTag) return status.activeMemberTag;
	if (status.kind === 'tunnel') return status.checkTag;

	const matchingTunnel = tunnels?.find((tunnel) => tunnel.tag === status.ref || tunnel.tag === status.checkTag);
	return matchingTunnel?.tag || status.checkTag;
}
