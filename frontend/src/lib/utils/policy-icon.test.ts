import { describe, expect, it } from 'vitest';
import { resolvePolicyIcon } from './policy-icon';

describe('resolvePolicyIcon', () => {
	it('maps common role labels', () => {
		expect(resolvePolicyIcon('home')).toBe('home');
		expect(resolvePolicyIcon('kids')).toBe('kids');
		expect(resolvePolicyIcon('work')).toBe('work');
	});

	it('is case-insensitive (lowercase matching)', () => {
		expect(resolvePolicyIcon('HOME')).toBe('home');
		expect(resolvePolicyIcon('VPN')).toBe('shield');
		expect(resolvePolicyIcon('WiFi_Guest')).toBe('guest');
		expect(resolvePolicyIcon('AMNEZIA_FOR_AWG')).toBe('shield');
		expect(resolvePolicyIcon('SmartHome')).toBe('iot');
		expect(resolvePolicyIcon('NFQWS')).toBe('tools');
		expect(resolvePolicyIcon('SingBox')).toBe('route');
		expect(resolvePolicyIcon('', { policyName: 'HydraRoute' })).toBe('hydraroute');
	});

	it('matches guest inside compound names', () => {
		expect(resolvePolicyIcon('wifi_guest')).toBe('guest');
		expect(resolvePolicyIcon('guest_wifi')).toBe('guest');
		expect(resolvePolicyIcon('GOST')).toBe('guest');
	});

	it('prefers guest over wifi when both tokens present', () => {
		expect(resolvePolicyIcon('wifi_guest')).toBe('guest');
	});

	it('maps routing, tunnel and tool stacks', () => {
		expect(resolvePolicyIcon('sbr')).toBe('route');
		expect(resolvePolicyIcon('magitrickle')).toBe('route');
		expect(resolvePolicyIcon('xray')).toBe('shield');
		expect(resolvePolicyIcon('vless')).toBe('shield');
		expect(resolvePolicyIcon('amnezia_for_awg')).toBe('shield');
		expect(resolvePolicyIcon('awg-de')).toBe('shield');
		expect(resolvePolicyIcon('nfqws')).toBe('tools');
		expect(resolvePolicyIcon('zapret')).toBe('tools');
	});

	it('avoids false positives on short substrings', () => {
		expect(resolvePolicyIcon('freedom')).toBe('shuffle');
	});

	it('uses policy name when description is empty', () => {
		expect(resolvePolicyIcon('', { policyName: 'HydraRoute' })).toBe('hydraroute');
	});

	it('falls back to shuffle for unknown labels', () => {
		expect(resolvePolicyIcon('my-custom-thing')).toBe('shuffle');
		expect(resolvePolicyIcon('')).toBe('shuffle');
	});

	it('uses hydraroute fallback when HR flag set and no label', () => {
		expect(resolvePolicyIcon('', { isHydraRoute: true })).toBe('hydraroute');
		expect(resolvePolicyIcon('', { policyName: 'Policy0', isHydraRoute: true })).toBe('hydraroute');
	});

	it('maps feedback policy name keywords', () => {
		expect(resolvePolicyIcon('XKeen')).toBe('route');
		expect(resolvePolicyIcon('ProxyRU')).toBe('shield');
		expect(resolvePolicyIcon('sing-box')).toBe('route');
		expect(resolvePolicyIcon('HRUS')).toBe('hydraroute');
		expect(resolvePolicyIcon('IoT_VPN')).toBe('iot');
		expect(resolvePolicyIcon('IoT_Xyandex')).toBe('iot');
		expect(resolvePolicyIcon('AWG-Manager-VPN')).toBe('shield');
		expect(resolvePolicyIcon('Only_MakeItGreatAgain')).toBe('direct');
		expect(resolvePolicyIcon('No_INet')).toBe('direct');
		expect(resolvePolicyIcon('dom.ru')).toBe('direct');
		expect(resolvePolicyIcon('beeline')).toBe('direct');
		expect(resolvePolicyIcon('splitrouting')).toBe('route');
		expect(resolvePolicyIcon('Docker')).toBe('server');
		expect(resolvePolicyIcon('homelab')).toBe('server');
		expect(resolvePolicyIcon('Alice')).toBe('iot');
		expect(resolvePolicyIcon('North_Korea')).toBe('north_korea');
	});
});
