import { describe, expect, it } from 'vitest';

import type { GeoFileSettings } from '$lib/types';

import {
	buildGeoScheduleOptions,
	geoSettingsFromPreset,
	presetFromGeoSettings,
	schedulePresetLabel
} from './geoRefreshPresets';

function baseGeoSettings(): GeoFileSettings {
	return {
		autoRefreshEnabled: false,
		refreshIntervalHours: 0,
		refreshMode: 'interval',
		refreshDailyTime: '03:00'
	};
}

describe('geo refresh presets', () => {
	it('maps disabled settings to off preset', () => {
		expect(presetFromGeoSettings(baseGeoSettings())).toBe('off');
	});

	it('maps standard interval presets', () => {
		expect(
			presetFromGeoSettings({
				...baseGeoSettings(),
				autoRefreshEnabled: true,
				refreshIntervalHours: 6
			})
		).toBe('6h');
		expect(
			presetFromGeoSettings({
				...baseGeoSettings(),
				autoRefreshEnabled: true,
				refreshIntervalHours: 12
			})
		).toBe('12h');
		expect(
			presetFromGeoSettings({
				...baseGeoSettings(),
				autoRefreshEnabled: true,
				refreshIntervalHours: 24
			})
		).toBe('24h');
	});

	it('maps daily settings to daily preset', () => {
		expect(
			presetFromGeoSettings({
				...baseGeoSettings(),
				autoRefreshEnabled: true,
				refreshMode: 'daily',
				refreshDailyTime: '04:30'
			})
		).toBe('daily:04:30');
	});

	it('keeps custom interval presets round-trippable', () => {
		const preset = presetFromGeoSettings({
			...baseGeoSettings(),
			autoRefreshEnabled: true,
			refreshIntervalHours: 8
		});
		expect(preset).toBe('interval:8');
		expect(schedulePresetLabel(preset)).toBe('Каждые 8 часов');
	});

	it('builds settings from presets', () => {
		expect(geoSettingsFromPreset('off', baseGeoSettings())).toMatchObject({
			autoRefreshEnabled: false
		});
		expect(geoSettingsFromPreset('6h', baseGeoSettings())).toMatchObject({
			autoRefreshEnabled: true,
			refreshMode: 'interval',
			refreshIntervalHours: 6
		});
		expect(geoSettingsFromPreset('12h', baseGeoSettings())).toMatchObject({
			autoRefreshEnabled: true,
			refreshMode: 'interval',
			refreshIntervalHours: 12
		});
		expect(geoSettingsFromPreset('24h', baseGeoSettings())).toMatchObject({
			autoRefreshEnabled: true,
			refreshMode: 'interval',
			refreshIntervalHours: 24
		});
		expect(geoSettingsFromPreset('daily:03:00', baseGeoSettings())).toMatchObject({
			autoRefreshEnabled: true,
			refreshMode: 'daily',
			refreshDailyTime: '03:00'
		});
		expect(geoSettingsFromPreset('interval:8', baseGeoSettings())).toMatchObject({
			autoRefreshEnabled: true,
			refreshMode: 'interval',
			refreshIntervalHours: 8
		});
	});

	it('keeps current custom preset visible in dropdown options', () => {
		const options = buildGeoScheduleOptions({
			...baseGeoSettings(),
			autoRefreshEnabled: true,
			refreshMode: 'daily',
			refreshDailyTime: '04:30'
		});

		expect(options.some((option) => option.value === 'daily:04:30')).toBe(true);
	});
});
