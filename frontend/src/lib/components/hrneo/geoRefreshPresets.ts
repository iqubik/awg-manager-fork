import type { GeoFileSettings } from '$lib/types';

export type SchedulePreset = 'off' | '6h' | '12h' | '24h' | `daily:${string}` | `interval:${number}`;

export function presetFromGeoSettings(geo: GeoFileSettings): SchedulePreset {
	if (!geo.autoRefreshEnabled) {
		return 'off';
	}
	if ((geo.refreshMode || 'interval') === 'daily') {
		return `daily:${geo.refreshDailyTime || '03:00'}`;
	}
	switch (geo.refreshIntervalHours || 6) {
		case 6:
			return '6h';
		case 12:
			return '12h';
		case 24:
			return '24h';
		default:
			return `interval:${geo.refreshIntervalHours || 6}`;
	}
}

export function schedulePresetLabel(preset: SchedulePreset): string {
	if (preset === 'off') return 'Отключено';
	if (preset === '6h') return 'Каждые 6 часов';
	if (preset === '12h') return 'Каждые 12 часов';
	if (preset === '24h') return 'Каждые 24 часа';
	if (preset.startsWith('daily:')) {
		return `Ежедневно ${preset.slice('daily:'.length)}`;
	}
	return `Каждые ${preset.slice('interval:'.length)} часов`;
}

export function geoSettingsFromPreset(
	preset: SchedulePreset,
	base: GeoFileSettings,
): GeoFileSettings {
	if (preset === 'off') {
		return {
			...base,
			autoRefreshEnabled: false
		};
	}
	if (preset === '6h' || preset === '12h' || preset === '24h') {
		return {
			...base,
			autoRefreshEnabled: true,
			refreshMode: 'interval',
			refreshIntervalHours: Number.parseInt(preset, 10)
		};
	}
	if (preset.startsWith('daily:')) {
		return {
			...base,
			autoRefreshEnabled: true,
			refreshMode: 'daily',
			refreshDailyTime: preset.slice('daily:'.length) || '03:00'
		};
	}
	const parsed = Number.parseInt(preset.slice('interval:'.length), 10);
	return {
		...base,
		autoRefreshEnabled: true,
		refreshMode: 'interval',
		refreshIntervalHours: Number.isFinite(parsed) && parsed > 0 ? parsed : 6
	};
}

export function buildGeoScheduleOptions(geo: GeoFileSettings) {
	const current = presetFromGeoSettings(geo);
	const options = [
		{ value: 'off', label: 'Отключено' },
		{ value: '6h', label: 'Каждые 6 часов' },
		{ value: '12h', label: 'Каждые 12 часов' },
		{ value: '24h', label: 'Каждые 24 часа' },
		{ value: 'daily:03:00', label: 'Ежедневно 03:00' }
	];
	if (!options.some((option) => option.value === current)) {
		options.splice(4, 0, { value: current, label: schedulePresetLabel(current) });
	}
	return options;
}
