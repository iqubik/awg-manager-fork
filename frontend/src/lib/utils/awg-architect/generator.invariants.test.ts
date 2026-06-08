import { describe, expect, it } from 'vitest';
import { genCfg, hexPad, splitPad, type GeneratorInput } from './generator';
import { generateSignaturePackets } from './signature';
import { getSignaturePackets, protocols } from '../protocols';

const baseInput: GeneratorInput = {
	version: '2.0',
	intensity: 'medium',
	profile: 'quic_initial',
	customHost: '',
	mimicAll: false,
	useTagC: false,
	useTagT: true,
	useTagR: true,
	useTagRC: true,
	useTagRD: true,
	useBrowserFp: false,
	browserProfile: '',
	mtu: 1420,
	junkLevel: 5,
	iterCount: 0,
	routerMode: false,
	useExtremeMax: false,
};

describe('AWG generator invariants', () => {
	// AmneziaWG kernel rejects a single <r>/<rc>/<rd> tag with N > 1000.
	describe('splitPad respects the 1000-byte per-tag limit', () => {
		it('zero → empty', () => expect(splitPad(0)).toBe(''));
		it('under limit → single tag', () => expect(splitPad(500)).toBe('<r 500>'));
		it('over limit → split, total preserved', () =>
			expect(splitPad(2500)).toBe('<r 1000><r 1000><r 500>'));
		it('honors the tag kind', () => expect(splitPad(64, 'rc')).toBe('<rc 64>'));
		it('no chunk exceeds 1000 and bytes sum to N across sizes', () => {
			for (const n of [1, 999, 1000, 1001, 3333, 9999]) {
				const nums = [...splitPad(n).matchAll(/<r (\d+)>/g)].map((m) => Number(m[1]));
				expect(Math.max(...nums)).toBeLessThanOrEqual(1000);
				expect(nums.reduce((a, b) => a + b, 0)).toBe(n);
			}
		});
	});

	describe('hexPad guarantees even-length hex', () => {
		it('pads to byteLen*2 chars', () => {
			expect(hexPad(5, 2)).toBe('0005');
			expect(hexPad(255, 1)).toBe('ff');
			for (let v = 0; v < 300; v += 7) {
				expect(hexPad(v, 2).length).toBe(4);
			}
		});
	});

	// Odd-length hex inside a <b 0x..> tag is rejected by the kernel module.
	it('every <b 0x..> payload in generated signatures has even hex', () => {
		for (const key of Object.keys(protocols)) {
			for (let i = 0; i < 20; i++) {
				const p = getSignaturePackets(key as keyof typeof protocols, 1420);
				for (const field of ['i1', 'i2', 'i3', 'i4', 'i5'] as const) {
					for (const m of p[field].matchAll(/<b 0x([0-9a-fA-F]*)>/g)) {
						expect(m[1].length % 2, `${key}.${field} hex=${m[1]}`).toBe(0);
					}
				}
			}
		}
	});

	// S1+56 ≠ S2 (Init packet size must differ from Response size) is enforced
	// by an unbounded retry loop, so it must hold for every seed.
	it('genCfg keeps S1+56 ≠ S2 across many seeds', () => {
		for (let i = 0; i < 200; i++) {
			const cfg = genCfg(baseInput);
			expect(cfg.s2, `iter ${i}`).not.toBe(cfg.s1 + 56);
		}
	});

	// <c> (counter tag) breaks old clients with ErrorCode 1000; the adapter
	// defaults useTagC=false, so no signature field may contain it.
	it('signature packets contain no <c> tag (useTagC=false default)', () => {
		for (let i = 0; i < 30; i++) {
			const p = generateSignaturePackets('quic_initial', 1420);
			for (const field of ['i1', 'i2', 'i3', 'i4', 'i5'] as const) {
				expect(p[field], field).not.toMatch(/<c>/);
			}
		}
	});
});
