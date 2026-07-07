import { describe, expect, it } from 'vitest';
import {
	extractSingboxFatalLines,
	getHydraAction,
	getHydraNoSpaceMessage,
	getIntegrationsErrorModalTitle,
	getIntegrationsHeaderMeta,
	getSingboxAction,
} from './integrationsCardLogic';

describe('integrationsCardLogic', () => {
	it('builds compact header meta for installed integrations', () => {
		expect(getIntegrationsHeaderMeta({ singboxInstalled: true, hydraInstalled: true })).toBe('Sing-box · HydraRoute');
		expect(getIntegrationsHeaderMeta({ singboxInstalled: true, hydraInstalled: false })).toBe('Sing-box');
		expect(getIntegrationsHeaderMeta({ singboxInstalled: false, hydraInstalled: false })).toBe('Не установлены');
	});

	it('extracts only real sing-box fatal stderr lines', () => {
		const raw = 'info line\n+0300 2026-07-07 FATAL[1234] broken config\n{\"type\":\"fatal\"}\nFATAL[0001] second fatal';
		expect(extractSingboxFatalLines(raw)).toBe('+0300 2026-07-07 FATAL[1234] broken config\nFATAL[0001] second fatal');
	});

	it('derives sing-box actions correctly', () => {
		expect(getSingboxAction({ statusLoading: true, installed: false, needsUpdate: false, hasUpdateHandler: false })).toBe('wait');
		expect(getSingboxAction({ statusLoading: false, installed: true, needsUpdate: true, hasUpdateHandler: true })).toBe('update');
		expect(getSingboxAction({ statusLoading: false, installed: true, needsUpdate: false, hasUpdateHandler: false })).toBe('open');
		expect(getSingboxAction({ statusLoading: true, installed: true, needsUpdate: false, hasUpdateHandler: false })).toBe('open');
		expect(getSingboxAction({ statusLoading: true, installed: true, needsUpdate: true, hasUpdateHandler: true })).toBe('update');
	});

	it('derives hydra actions correctly for legacy, update, and unavailable cases', () => {
		expect(
			getHydraAction({
				statusLoading: true,
				installed: true,
				managed: false,
				legacy: true,
				installSupported: true,
				needsUpdate: false,
				noSpace: false,
				hydraInstalling: false,
				hydraUpdating: false,
				hasInstallHandler: true,
				hasUpdateHandler: true,
			}),
		).toBe('official-install');

		expect(
			getHydraAction({
				statusLoading: true,
				installed: true,
				managed: true,
				legacy: false,
				installSupported: true,
				needsUpdate: true,
				noSpace: false,
				hydraInstalling: false,
				hydraUpdating: false,
				hasInstallHandler: true,
				hasUpdateHandler: true,
			}),
		).toBe('update');

		expect(
			getHydraAction({
				statusLoading: true,
				installed: true,
				managed: true,
				legacy: false,
				installSupported: true,
				needsUpdate: false,
				noSpace: false,
				hydraInstalling: false,
				hydraUpdating: false,
				hasInstallHandler: true,
				hasUpdateHandler: true,
			}),
		).toBe('open');

		expect(
			getHydraAction({
				statusLoading: false,
				installed: false,
				managed: false,
				legacy: false,
				installSupported: false,
				needsUpdate: false,
				noSpace: true,
				hydraInstalling: false,
				hydraUpdating: false,
				hasInstallHandler: false,
				hasUpdateHandler: false,
			}),
		).toBe('unavailable');
	});

	it('formats no-space warning and error modal title', () => {
		expect(
			getHydraNoSpaceMessage({ requiredBytes: 1024, freeBytes: 512 } as never),
		).toContain('Недостаточно места');
		expect(getIntegrationsErrorModalTitle({ hydraUpdateError: 'boom' })).toBe('Не удалось обновить HydraRoute');
	});
});
