import { describe, expect, it } from 'vitest';
import { detectI1ProtocolFromHex, parseI1 } from './awgConfAnalyzer';

describe('awgConfAnalyzer I1 parsing', () => {
	it('accepts random prefix before b tag in I1', () => {
		const p = parseI1(
			'<r 2><b 0x858000010001000000000669636c6f756403636f6d0000010001c00c000100010000105a00044d583737>',
		);

		expect(p.errors).not.toContain(expect.stringContaining('Первый тег'));
		expect(p.hasRandomPrefixBeforeB).toBe(true);
		expect(p.randomPrefixBytes).toBe(2);
		expect(p.protocol).toBe('DNS');
	});

	it('still warns for unsupported non-random tags before b', () => {
		const p = parseI1('<t><b 0x1603010200>');

		expect(p.errors.length).toBeGreaterThan(0);
	});

	it('detects dns when transaction id is moved into random prefix', () => {
		expect(
			detectI1ProtocolFromHex(
				'858000010001000000000669636c6f756403636f6d0000010001c00c000100010000105a00044d583737',
				2,
			),
		).toBe('DNS');
	});
});
