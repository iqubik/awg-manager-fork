import { describe, expect, it } from 'vitest';
import type { AmneziaPremiumIssuedConfig } from '$lib/types';
import {
	isPremiumCountryConfigStale,
	isPremiumCountryIssued,
	isPremiumIssuedConfigActiveDevice,
	isPremiumIssuedConfigReissuable,
	premiumActiveDevicesForCountry,
	premiumIssuedConfigSourceType,
	premiumIssuedConfigsForCountry,
} from './amneziaPremiumVpnPaste';

function issuedConfig(patch: Partial<AmneziaPremiumIssuedConfig> = {}): AmneziaPremiumIssuedConfig {
	return {
		server_country_code: 'de',
		server_country_name: 'Germany',
		worker_last_updated: '2026-05-30T10:00:00Z',
		last_downloaded: '2026-05-30T09:00:00Z',
		...patch,
	};
}

describe('amneziaPremiumVpnPaste issued config helpers', () => {
	it('does not treat gateway_account as an issued country config', () => {
		const issued = [issuedConfig({ source_type: 'gateway_account' })];

		expect(isPremiumIssuedConfigReissuable(issued[0])).toBe(false);
		expect(premiumIssuedConfigsForCountry(issued, 'de')).toEqual([]);
		expect(isPremiumCountryIssued(issued, 'de')).toBe(false);
		expect(isPremiumCountryConfigStale(issued, 'de')).toBe(false);
	});

	it('normalizes gateway_account source_type before filtering', () => {
		const issued = [issuedConfig({ source_type: ' Gateway_Account ' })];

		expect(isPremiumIssuedConfigReissuable(issued[0])).toBe(false);
		expect(isPremiumCountryIssued(issued, 'DE')).toBe(false);
		expect(isPremiumCountryConfigStale(issued, 'DE')).toBe(false);
	});

	it('keeps legacy issued configs without source_type as reissuable', () => {
		const issued = [issuedConfig({ source_type: undefined })];

		expect(isPremiumIssuedConfigReissuable(issued[0])).toBe(true);
		expect(premiumIssuedConfigsForCountry(issued, 'de')).toHaveLength(1);
		expect(isPremiumCountryIssued(issued, 'de')).toBe(true);
		expect(isPremiumCountryConfigStale(issued, 'de')).toBe(true);
	});

	it('keeps non-gateway issued configs as reissuable', () => {
		const issued = [issuedConfig({ source_type: 'downloaded_config' })];

		expect(isPremiumIssuedConfigReissuable(issued[0])).toBe(true);
		expect(isPremiumCountryIssued(issued, 'de')).toBe(true);
		expect(isPremiumCountryConfigStale(issued, 'de')).toBe(true);
	});

	it('treats a mixed country as issued only when it has a real config entry', () => {
		const issued = [
			issuedConfig({ source_type: 'gateway_account' }),
			issuedConfig({
				source_type: 'downloaded_config',
				last_downloaded: '2026-05-30T11:00:00Z',
			}),
		];

		expect(premiumIssuedConfigsForCountry(issued, 'de')).toHaveLength(1);
		expect(isPremiumCountryIssued(issued, 'de')).toBe(true);
		expect(isPremiumCountryConfigStale(issued, 'de')).toBe(false);
	});

	it('returns active devices separately without mixing them into reissuable configs', () => {
		const issued = [
			issuedConfig({ source_type: 'gateway_account' }),
			issuedConfig({ source_type: 'downloaded_config' }),
			issuedConfig({ source_type: ' gateway_account ', server_country_code: 'nl' }),
		];

		expect(premiumActiveDevicesForCountry(issued, 'de')).toHaveLength(1);
		expect(premiumIssuedConfigsForCountry(issued, 'de')).toHaveLength(1);
		expect(premiumActiveDevicesForCountry(issued, 'nl')).toHaveLength(1);
	});

	it('normalizes source_type for active-device detection', () => {
		const active = issuedConfig({ source_type: ' Gateway_Account ' });
		const config = issuedConfig({ source_type: 'downloaded_config' });

		expect(premiumIssuedConfigSourceType(active)).toBe('gateway_account');
		expect(isPremiumIssuedConfigActiveDevice(active)).toBe(true);
		expect(isPremiumIssuedConfigActiveDevice(config)).toBe(false);
	});

	it('keeps active-device-only countries available for direct download flow', () => {
		const issued = [issuedConfig({ source_type: 'gateway_account' })];

		expect(isPremiumCountryIssued(issued, 'de')).toBe(false);
		expect(isPremiumCountryConfigStale(issued, 'de')).toBe(false);
		expect(premiumActiveDevicesForCountry(issued, 'de')).toHaveLength(1);
	});
});
