import type { MonitoringTunnel } from '$lib/types';

export function tunnelIsUp(tunnel: MonitoringTunnel, hasOkCell: boolean): boolean {
	if (hasOkCell) return true;

	if (
		tunnel.source === 'singbox' &&
		tunnel.subscription &&
		typeof tunnel.clashDelay === 'number' &&
		tunnel.clashDelay > 0
	) {
		return true;
	}

	return false;
}
