import { describe, expect, it } from 'vitest';
import { resolveNdmsCardIconStyle, resolveNeutralServiceIconStyle } from './ndms-card-icon-style';
import { getPolicyIconColor } from './policy-icon';

describe('resolveNdmsCardIconStyle', () => {
	it('uses transparent tile in strict mode', () => {
		expect(resolveNdmsCardIconStyle('strict', '#0077ff')).toEqual({
			background: 'transparent',
			foreground: 'var(--color-text-secondary, var(--text-muted))',
		});
	});

	it('uses accent colors in vivid mode', () => {
		expect(resolveNdmsCardIconStyle('vivid', '#0077ff')).toEqual({
			background: '#0077ff',
			foreground: '#fff',
		});
	});

	it('maps policy icons to accent colors', () => {
		expect(getPolicyIconColor('home')).toBe('#0077ff');
		expect(getPolicyIconColor('shield')).toBe('#00a650');
	});

	it('uses icon mode for neutral service globe fallback', () => {
		expect(resolveNeutralServiceIconStyle('strict')).toEqual(
			resolveNdmsCardIconStyle('strict', '#22c55e'),
		);
		expect(resolveNeutralServiceIconStyle('vivid')).toEqual(
			resolveNdmsCardIconStyle('vivid', '#22c55e'),
		);
	});
});
