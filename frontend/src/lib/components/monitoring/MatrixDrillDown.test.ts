import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/svelte';
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

	it('requests full retained history and paginates through it honestly', async () => {
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
				historyHours: 24,
				sampleIntervalSec: 60,
				historyCapacity: 1440,
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

		expect(await screen.findByText(/История замеров/i)).toBeTruthy();
		expect(screen.getByText(/Окно: 24 часа · до 1440 точек · шаг 60 секунд/i)).toBeTruthy();
		expect(screen.getByText(/1-10 из 12/i)).toBeTruthy();
		expect((screen.getByRole('button', { name: 'Новее' }) as HTMLButtonElement).disabled).toBe(true);
		expect((screen.getByRole('button', { name: 'Старее' }) as HTMLButtonElement).disabled).toBe(false);

		await fireEvent.click(screen.getByRole('button', { name: 'Старее' }));
		expect(screen.getByText(/11-12 из 12/i)).toBeTruthy();
		expect((screen.getByRole('button', { name: 'Новее' }) as HTMLButtonElement).disabled).toBe(false);
		expect((screen.getByRole('button', { name: 'Старее' }) as HTMLButtonElement).disabled).toBe(true);
	});
});
