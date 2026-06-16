import type { MonitoringSettings } from '$lib/types';

export const DEFAULT_MONITORING_SETTINGS: MonitoringSettings = {
	historyHours: 24,
	sampleIntervalSec: 60,
	matrixRefreshIntervalSec: 60,
};

export const MIN_MONITORING_HISTORY_HOURS = 1;
export const MAX_MONITORING_HISTORY_HOURS = 168;
export const MIN_MONITORING_SAMPLE_INTERVAL_SEC = 10;
export const MAX_MONITORING_SAMPLE_INTERVAL_SEC = 3600;
export const MIN_MONITORING_MATRIX_REFRESH_INTERVAL_SEC = 10;
export const MAX_MONITORING_MATRIX_REFRESH_INTERVAL_SEC = 3600;
export const MAX_MONITORING_HISTORY_CAPACITY = 10080;

export const MONITORING_RECENT_ROWS = 10;
export const MONITORING_SPARKLINE_POINTS = 240;

export function getMonitoringHistoryCapacityRaw(value: MonitoringSettings): number {
	return Math.ceil((value.historyHours * 3600) / value.sampleIntervalSec);
}

export function normalizeMonitoringSettings(raw?: Partial<MonitoringSettings> | null): MonitoringSettings {
	const normalized: MonitoringSettings = {
		historyHours: raw?.historyHours ?? DEFAULT_MONITORING_SETTINGS.historyHours,
		sampleIntervalSec: raw?.sampleIntervalSec ?? DEFAULT_MONITORING_SETTINGS.sampleIntervalSec,
		matrixRefreshIntervalSec: raw?.matrixRefreshIntervalSec ?? DEFAULT_MONITORING_SETTINGS.matrixRefreshIntervalSec,
	};

	if (!Number.isInteger(normalized.historyHours) || normalized.historyHours < MIN_MONITORING_HISTORY_HOURS || normalized.historyHours > MAX_MONITORING_HISTORY_HOURS) {
		normalized.historyHours = DEFAULT_MONITORING_SETTINGS.historyHours;
	}
	if (!Number.isInteger(normalized.sampleIntervalSec) || normalized.sampleIntervalSec < MIN_MONITORING_SAMPLE_INTERVAL_SEC || normalized.sampleIntervalSec > MAX_MONITORING_SAMPLE_INTERVAL_SEC) {
		normalized.sampleIntervalSec = DEFAULT_MONITORING_SETTINGS.sampleIntervalSec;
	}
	if (!Number.isInteger(normalized.matrixRefreshIntervalSec) || normalized.matrixRefreshIntervalSec < MIN_MONITORING_MATRIX_REFRESH_INTERVAL_SEC || normalized.matrixRefreshIntervalSec > MAX_MONITORING_MATRIX_REFRESH_INTERVAL_SEC) {
		normalized.matrixRefreshIntervalSec = DEFAULT_MONITORING_SETTINGS.matrixRefreshIntervalSec;
	}
	if (getMonitoringHistoryCapacityRaw(normalized) > MAX_MONITORING_HISTORY_CAPACITY) {
		return { ...DEFAULT_MONITORING_SETTINGS };
	}

	return normalized;
}

export function getMonitoringHistoryCapacity(value: MonitoringSettings): number {
	return getMonitoringHistoryCapacityRaw(normalizeMonitoringSettings(value));
}

export function validateMonitoringSettings(value: MonitoringSettings): Record<string, string> {
	const errors: Record<string, string> = {};

	if (!Number.isInteger(value.historyHours) || value.historyHours < MIN_MONITORING_HISTORY_HOURS || value.historyHours > MAX_MONITORING_HISTORY_HOURS) {
		errors.historyHours = `Введите целое число от ${MIN_MONITORING_HISTORY_HOURS} до ${MAX_MONITORING_HISTORY_HOURS}.`;
	}
	if (!Number.isInteger(value.sampleIntervalSec) || value.sampleIntervalSec < MIN_MONITORING_SAMPLE_INTERVAL_SEC || value.sampleIntervalSec > MAX_MONITORING_SAMPLE_INTERVAL_SEC) {
		errors.sampleIntervalSec = `Введите целое число от ${MIN_MONITORING_SAMPLE_INTERVAL_SEC} до ${MAX_MONITORING_SAMPLE_INTERVAL_SEC}.`;
	}
	if (!Number.isInteger(value.matrixRefreshIntervalSec) || value.matrixRefreshIntervalSec < MIN_MONITORING_MATRIX_REFRESH_INTERVAL_SEC || value.matrixRefreshIntervalSec > MAX_MONITORING_MATRIX_REFRESH_INTERVAL_SEC) {
		errors.matrixRefreshIntervalSec = `Введите целое число от ${MIN_MONITORING_MATRIX_REFRESH_INTERVAL_SEC} до ${MAX_MONITORING_MATRIX_REFRESH_INTERVAL_SEC}.`;
	}
	if (!errors.historyHours && !errors.sampleIntervalSec && getMonitoringHistoryCapacityRaw(value) > MAX_MONITORING_HISTORY_CAPACITY) {
		errors.historyCapacity = 'Слишком большой объём истории. Увеличьте шаг замера или уменьшите окно истории.';
	}

	return errors;
}
