import { describe, expect, it } from 'vitest';
import {
	DEFAULT_MONITORING_SETTINGS,
	getMonitoringHistoryCapacity,
	getMonitoringHistoryCapacityRaw,
	normalizeMonitoringSettings,
	validateMonitoringSettings,
} from './monitoring';

describe('monitoring constants helpers', () => {
	it('normalizes missing settings to defaults', () => {
		expect(normalizeMonitoringSettings()).toEqual(DEFAULT_MONITORING_SETTINGS);
	});

	it('derives history capacity from hours and sample interval', () => {
		expect(getMonitoringHistoryCapacity({
			historyHours: 24,
			sampleIntervalSec: 60,
			matrixRefreshIntervalSec: 60,
		})).toBe(1440);

		expect(getMonitoringHistoryCapacity({
			historyHours: 48,
			sampleIntervalSec: 60,
			matrixRefreshIntervalSec: 120,
		})).toBe(2880);
	});

	it('reports validation errors for out-of-range values', () => {
		const errors = validateMonitoringSettings({
			historyHours: 0,
			sampleIntervalSec: 5,
			matrixRefreshIntervalSec: 0,
		});

		expect(errors.historyHours).toBeTruthy();
		expect(errors.sampleIntervalSec).toBeTruthy();
		expect(errors.matrixRefreshIntervalSec).toBeTruthy();
	});

	it('computes raw history capacity and blocks oversized schedules', () => {
		expect(getMonitoringHistoryCapacityRaw({
			historyHours: 168,
			sampleIntervalSec: 10,
			matrixRefreshIntervalSec: 60,
		})).toBe(60480);

		const errors = validateMonitoringSettings({
			historyHours: 168,
			sampleIntervalSec: 10,
			matrixRefreshIntervalSec: 60,
		});

		expect(errors.historyCapacity).toBeTruthy();
	});
});
