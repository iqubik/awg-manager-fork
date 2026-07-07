import { formatBytes } from '$lib/utils/format';
import { stripAnsi } from '$lib/utils/ansi';
import type { HydraRouteStatus, SingboxStatus } from '$lib/types';

export type IntegrationAction = 'install' | 'update' | 'open' | 'wait' | 'official-install' | 'unavailable';

export function getIntegrationsHeaderMeta(options: {
	singboxInstalled: boolean;
	hydraInstalled: boolean;
}): string {
	if (options.singboxInstalled && options.hydraInstalled) return 'Sing-box · HydraRoute';
	if (options.singboxInstalled) return 'Sing-box';
	if (options.hydraInstalled) return 'HydraRoute';
	return 'Не установлены';
}

export function extractSingboxFatalLines(lastError?: string): string {
	const raw = stripAnsi(lastError ?? '').trim();
	if (!raw) return '';
	const fatal = raw.split('\n').filter((line) => {
		const upper = line.toUpperCase();
		if (!upper.includes('FATAL')) return false;
		if (upper.includes('FATAL[')) return true;
		return /^\s*\+[0-9]{1,4}\s+\d{4}-\d{2}-\d{2}\b/.test(line);
	});
	return fatal.join('\n');
}

export function getSingboxAction(options: {
	statusLoading: boolean;
	installed: boolean;
	needsUpdate: boolean;
	hasUpdateHandler: boolean;
}): IntegrationAction {
	if (options.installed && options.needsUpdate && options.hasUpdateHandler) return 'update';
	if (options.installed) return 'open';
	if (options.statusLoading) return 'wait';
	return 'install';
}

export function getHydraAction(options: {
	statusLoading: boolean;
	installed: boolean;
	managed: boolean;
	legacy: boolean;
	installSupported: boolean;
	needsUpdate: boolean;
	noSpace: boolean;
	hydraInstalling: boolean;
	hydraUpdating: boolean;
	hasInstallHandler: boolean;
	hasUpdateHandler: boolean;
}): IntegrationAction {
	if (options.installed && options.managed && options.installSupported && options.needsUpdate && !options.hydraInstalling && options.hasUpdateHandler) {
		return 'update';
	}
	if (options.installed && options.legacy && options.installSupported && !options.hydraInstalling && options.hasInstallHandler) {
		return 'official-install';
	}
	if (options.installed && !options.hydraUpdating) return 'open';
	if (options.statusLoading) return 'wait';
	if (options.installSupported && !options.noSpace && options.hasInstallHandler) return 'install';
	return 'unavailable';
}

export function getHydraNoSpaceMessage(status: HydraRouteStatus | null): string {
	if (!status) return '';
	return `Недостаточно места: нужно ${formatBytes(status.requiredBytes ?? 0)}, доступно ${formatBytes(status.freeBytes ?? 0)}`;
}

export function getIntegrationsErrorModalTitle(errors: {
	hydraUpdateError?: string | null;
	hydraInstallError?: string | null;
	singboxUpdateError?: string | null;
	singboxInstallError?: string | null;
}): string {
	if (errors.hydraUpdateError) return 'Не удалось обновить HydraRoute';
	if (errors.hydraInstallError) return 'Не удалось установить HydraRoute';
	if (errors.singboxUpdateError) return 'Не удалось обновить sing-box';
	return 'Не удалось установить sing-box';
}
