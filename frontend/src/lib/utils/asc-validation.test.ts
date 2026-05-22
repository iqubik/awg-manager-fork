import { describe, expect, it } from 'vitest';
import type { ASCParams, ASCParamsExtended } from '$lib/types';
import { applyDisabledASCState, isZeroASCState, validateASCBeforeSave } from '$lib/utils/asc-validation';

function baseValid(): ASCParams {
	return {
		jc: 3,
		jmin: 77,
		jmax: 266,
		s1: 18,
		s2: 29,
		h1: '103994526',
		h2: '1201929360',
		h3: '2403636727',
		h4: '3602647725',
	};
}

describe('validateASCBeforeSave', () => {
	it('accepts valid base ASC', () => {
		expect(validateASCBeforeSave(baseValid())).toEqual([]);
	});

	it('reports all empty H1-H4 fields', () => {
		const p = { ...baseValid(), h1: '', h2: '', h3: '', h4: '' };
		const errs = validateASCBeforeSave(p);
		expect(errs.join('\n')).toContain('H1');
		expect(errs.join('\n')).toContain('H2');
		expect(errs.join('\n')).toContain('H3');
		expect(errs.join('\n')).toContain('H4');
	});

	it('reports multiple empty H fields, not only first', () => {
		const p = { ...baseValid(), h2: '', h3: '', h4: '' };
		const errs = validateASCBeforeSave(p);
		const merged = errs.join('\n');
		expect(merged).toContain('H2');
		expect(merged).toContain('H3');
		expect(merged).toContain('H4');
	});

	it('accepts zero disabled state', () => {
		const p: ASCParams = {
			jc: 0,
			jmin: 0,
			jmax: 0,
			s1: 0,
			s2: 0,
			h1: '',
			h2: '',
			h3: '',
			h4: '',
		};
		expect(validateASCBeforeSave(p)).toEqual([]);
	});

	it('rejects partial disabled state', () => {
		const p: ASCParams = {
			jc: 0,
			jmin: 0,
			jmax: 0,
			s1: 0,
			s2: 0,
			h1: '100',
			h2: '',
			h3: '',
			h4: '',
		};
		const errs = validateASCBeforeSave(p).join('\n');
		expect(errs).toContain('JMIN');
		expect(errs).toContain('JMAX');
		expect(errs).toContain('H2');
		expect(errs).toContain('H3');
		expect(errs).toContain('H4');
	});

	it('rejects jmax <= jmin', () => {
		const p = { ...baseValid(), jmin: 200, jmax: 200 };
		expect(validateASCBeforeSave(p).join('\n')).toContain('Jmax должен быть больше Jmin');
	});

	it('validates extended pair s3/s4 together', () => {
		const p = {
			...baseValid(),
			s3: 8,
		} as ASCParamsExtended;
		const errs = validateASCBeforeSave(p).join('\n');
		expect(errs).toContain('S4');
	});

	it('applyDisabledASCState sets disabled base ASC', () => {
		const p: ASCParams = baseValid();
		applyDisabledASCState(p);
		expect(isZeroASCState(p)).toBe(true);
		expect(p.jc).toBe(0);
		expect(p.jmin).toBe(0);
		expect(p.jmax).toBe(0);
		expect(p.s1).toBe(0);
		expect(p.s2).toBe(0);
		expect(p.h1).toBe('');
		expect(p.h2).toBe('');
		expect(p.h3).toBe('');
		expect(p.h4).toBe('');
	});

	it('applyDisabledASCState sets disabled extended ASC and clears I-fields', () => {
		const p = {
			...baseValid(),
			s3: 12,
			s4: 9,
			i1: 'A',
			i2: 'B',
			i3: 'C',
			i4: 'D',
			i5: 'E',
		} as ASCParamsExtended;
		applyDisabledASCState(p);
		expect(isZeroASCState(p)).toBe(true);
		expect(p.s3).toBe(0);
		expect(p.s4).toBe(0);
		expect(p.i1).toBe('');
		expect(p.i2).toBe('');
		expect(p.i3).toBe('');
		expect(p.i4).toBe('');
		expect(p.i5).toBe('');
	});

	it('generated-like ASC is not zero state', () => {
		const p = baseValid();
		expect(isZeroASCState(p)).toBe(false);
	});
});
