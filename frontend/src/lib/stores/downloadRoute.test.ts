import { describe, expect, it } from 'vitest';
import type { DownloadOutbound, Settings } from '$lib/types';
import { resolveDownloadRouteLabel } from './downloadRoute';

const directOutbound: DownloadOutbound = {
	tag: 'direct',
	kind: 'direct',
	label: 'Direct (WAN)',
	available: true,
};

function settings(routeTag: string, routeKind?: Settings['download']['routeKind']): Settings {
	return {
		download: { routeTag, routeKind },
	} as Settings;
}

describe('resolveDownloadRouteLabel', () => {
	it('resolves direct without routeKind (no false unavailable)', () => {
		const label = resolveDownloadRouteLabel(settings('direct'), [directOutbound]);
		expect(label).toBe('Direct (WAN)');
		expect(label).not.toContain('недоступен');
	});

	it('falls back to WAN label when outbounds list is empty', () => {
		const label = resolveDownloadRouteLabel(settings('direct'), []);
		expect(label).toBe('Direct (WAN) — без туннеля');
	});

	it('marks unavailable outbound in label', () => {
		const label = resolveDownloadRouteLabel(settings('awg-work', 'awg'), [
			{ tag: 'awg-work', kind: 'awg', label: 'Work', available: false },
		]);
		expect(label).toBe('Work (AWG) (недоступен)');
	});
});
