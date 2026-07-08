import { describe, expect, it } from 'vitest';
import { detectI1ProtocolFromHex, detectVersion, parseAWG, parseI1, runChecks } from './awgConfAnalyzer';

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

	it('treats S1=80 as valid Amnezia padding without 0-64 warning', () => {
		const parsed = parseAWG(`[Interface]
PrivateKey = TEST_PRIVATE_KEY
Address = 10.0.0.2/32
S1 = 80
S2 = 19
S3 = 51
S4 = 8

[Peer]
PublicKey = TEST_PUBLIC_KEY
AllowedIPs = 0.0.0.0/0
Endpoint = demo.example.com:51820
`);
		const version = detectVersion(parsed.iface);
		const checks = runChecks(parsed.iface, parsed.peer, version);
		const s1 = checks.find((c) => c.title === 'S1 — Init prefix');

		expect(s1).toBeTruthy();
		expect(s1?.status).toBe('pass');
		expect(s1?.detail).not.toContain('0-64');
		expect(s1?.detail).toContain('15–149');

		const conflict = checks.find((c) => c.title === 'Конфликт итоговых размеров пакетов');
		expect(conflict).toBeUndefined();
	});
});
