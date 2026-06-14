import { describe, it, expect } from 'vitest';
import { engineFatalHint, ENGINE_FATAL_FALLBACK } from './engineFatalHints';

describe('engineFatalHint', () => {
	const cases: [string, string][] = [
		['... Legacy Address Filter Fields in DNS rules is deprecated', 'IP-набор'],
		['FATAL[0000] start service: initialize cache-file: timeout', 'killall sing-box'],
		['initialize router: parse rule-set[2]: open /opt/x.srs: no such file or directory', '.srs'],
		['start service: ... outbound not found: awg-vpn0', 'outbound'],
		['missing fakeip record, try enable `experimental.cache_file`', 'FakeIP'],
		['initialize inbound[0]: listen tcp 0.0.0.0:51272: bind: address already in use', 'Порт'],
	];
	for (const [raw, needle] of cases) {
		it(`maps ${needle}`, () => {
			expect(engineFatalHint(raw)).toContain(needle);
		});
	}

	it('returns null for unknown FATAL', () => {
		expect(engineFatalHint('FATAL[0000] something unfamiliar')).toBeNull();
	});
	it('returns null for empty/missing input', () => {
		expect(engineFatalHint('')).toBeNull();
		expect(engineFatalHint(null)).toBeNull();
		expect(engineFatalHint(undefined)).toBeNull();
	});
	it('exposes a non-empty fallback', () => {
		expect(ENGINE_FATAL_FALLBACK.length).toBeGreaterThan(0);
	});
});
