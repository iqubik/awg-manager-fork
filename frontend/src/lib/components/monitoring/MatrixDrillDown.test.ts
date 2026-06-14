import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/svelte';
import MatrixDrillDown from './MatrixDrillDown.svelte';
import type { MonitoringSample } from '$lib/types';

const { getMonitoringHistory, getCachedHistory, setCachedHistory } = vi.hoisted(() => ({
	getMonitoringHistory: vi.fn(),
	getCachedHistory: vi.fn(),
	setCachedHistory: vi.fn(),
}));

vi.mock('$lib/api/client', () => ({
	api: {
		getMonitoringHistory,
	},
}));

vi.mock('$lib/stores/monitoring', () => ({
	getCachedHistory,
	setCachedHistory,
}));

function makeSamples(count: number): MonitoringSample[] {
	return Array.from({ length: count }, (_, i) => ({
		ts: new Date(2026, 5, 14, 12, i, 0).toISOString(),
		latencyMs: 50 + i,
		ok: true,
	}));
}

describe('MatrixDrillDown', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		getCachedHistory.mockReturnValue(null);
		getMonitoringHistory.mockResolvedValue(makeSamples(12));
	});

	it('requests 1440-point history and shows honest 24h footer', async () => {
		render(MatrixDrillDown, {
			props: {
				target: { id: 'cf-1.1.1.1', host: '1.1.1.1', name: 'Cloudflare DNS' },
				tunnel: {
					id: 'tun-1',
					name: 'Tunnel 1',
					ifaceName: 'awg0',
					pingcheckTarget: '',
					selfTarget: '',
					selfMethod: 'http',
				},
				onClose: () => {},
			},
		});

		await waitFor(() => {
			expect(getMonitoringHistory).toHaveBeenCalledWith({
				target: 'cf-1.1.1.1',
				tunnelId: 'tun-1',
				limit: 1440,
			});
		});

		expect(await screen.findByText(/Последние 10 замеров/i)).toBeTruthy();
		expect(screen.getByText(/Окно: 24 часа · до 1440 точек · шаг 60 секунд/i)).toBeTruthy();
	});
});
