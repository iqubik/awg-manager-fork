import { describe, expect, it } from 'vitest';
import { calcByteSize, calcTotalSize, getSignaturePackets, protocols } from '../protocols';

describe('Architect signature adapter', () => {
	it('generates non-empty CPS chains for every profile', () => {
		for (const key of Object.keys(protocols)) {
			const packets = getSignaturePackets(key as keyof typeof protocols, 1280);
			expect(packets.i1.length, key).toBeGreaterThan(0);
			expect(packets.i1, key).toMatch(/<b 0x[0-9a-fA-F]+>/);
			for (const field of ['i2', 'i3', 'i4', 'i5'] as const) {
				expect(packets[field].length, `${key}.${field}`).toBeGreaterThan(0);
			}
		}
	});

	it('stays within the 4096-byte I1–I5 budget', () => {
		for (const key of Object.keys(protocols)) {
			for (let i = 0; i < 30; i++) {
				const packets = getSignaturePackets(key as keyof typeof protocols, 1420);
				expect(calcTotalSize(packets)).toBeLessThanOrEqual(4096);
			}
		}
	});

	it('counts CPS tag overhead in calcByteSize', () => {
		const pattern = '<b 0x0102><r 10><t><rc 5>';
		expect(calcByteSize(pattern)).toBe(2 + 10 + 4 + 5);
	});
});
