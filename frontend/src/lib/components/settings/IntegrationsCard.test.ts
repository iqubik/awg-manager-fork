import { describe, it, expect, vi } from 'vitest';
import { render } from '@testing-library/svelte';
import IntegrationsCard from './IntegrationsCard.svelte';
import type { HydraRouteStatus, SingboxStatus } from '$lib/types';

vi.mock('$lib/stores/hydrarouteInstall', () => ({
	hydraRouteInstallProgress: { subscribe: vi.fn((run) => { run(null); return () => {}; }) },
}));

vi.mock('$lib/stores/singboxInstall', () => ({
	singboxInstallProgress: { subscribe: vi.fn((run) => { run(null); return () => {}; }) },
}));

const singboxStatus: SingboxStatus | null = null;

function renderHydra(status: HydraRouteStatus) {
	return render(IntegrationsCard, {
		expanded: true,
		singboxStatus,
		hydraStatus: status,
		singboxInstalling: false,
		singboxInstallError: null,
		oninstallSingbox: vi.fn(),
		oninstallHydra: vi.fn(),
		onupdateHydra: vi.fn(),
		showSingbox: false,
		showHydra: true,
	});
}

describe('IntegrationsCard', () => {
	it('shows official install action for legacy HydraRoute when package install is supported', () => {
		const { getByRole, getByText, queryByRole } = renderHydra({
			installed: true,
			running: false,
			legacy: true,
			managed: false,
			currentVersion: '2.4.1',
			installSupported: true,
			updateAvailable: false,
			customBuild: true,
			processState: 'stopped',
		});

		expect(getByText(/внешняя или нестандартная установка hydraroute neo/i)).toBeTruthy();
		expect(getByRole('button', { name: /установить официально/i })).toBeTruthy();
		expect(queryByRole('button', { name: /открыть/i })).toBeNull();
		expect(queryByRole('button', { name: /обновить/i })).toBeNull();
	});
});
