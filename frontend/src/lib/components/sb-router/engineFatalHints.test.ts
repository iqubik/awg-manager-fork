import { describe, it, expect } from 'vitest';
import { engineFatalHint } from './engineFatalHints';

describe('engineFatalHint', () => {
	it('маппит Address Filter Fields на подсказку про IP-наборы в DNS', () => {
		const raw =
			'FATAL[0000] start service: initialize rule-set[58]: cloudflare_2: ' +
			'validate dns rule[2]: Legacy Address Filter Fields in DNS rules is deprecated';
		expect(engineFatalHint(raw)).toContain('IP-набор');
	});

	it('возвращает null для неизвестного FATAL', () => {
		expect(engineFatalHint('FATAL[0000] something unfamiliar')).toBeNull();
	});

	it('возвращает null для пустого/отсутствующего ввода', () => {
		expect(engineFatalHint('')).toBeNull();
		expect(engineFatalHint(null)).toBeNull();
		expect(engineFatalHint(undefined)).toBeNull();
	});
});
