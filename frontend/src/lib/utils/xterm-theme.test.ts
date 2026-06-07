import { describe, expect, it } from 'vitest';
import { buildXtermTheme } from './xterm-theme';

const LEGACY_LIGHT_TOKENS = {
	'--color-accent': '#0096e1',
	'--color-accent-hover': '#1ba8ef',
	'--color-success': '#0d9488',
	'--color-error': '#dc2626',
	'--color-warning': '#b45309',
	'--color-info': '#0284c7',
	'--color-bg-primary': '#f0f4f8',
	'--color-bg-secondary': '#ffffff',
	'--color-bg-tertiary': '#e4edf5',
	'--color-bg-hover': '#d6e3f0',
	'--color-text-primary': '#1a212c',
	'--color-text-secondary': '#3d4d5f',
	'--color-text-muted': '#5c6b7d',
};

const LEGACY_DARK_TOKENS = {
	'--color-accent': '#7aa2f7',
	'--color-accent-hover': '#6e8bbb',
	'--color-success': '#9ece6a',
	'--color-error': '#f7768e',
	'--color-warning': '#e0af68',
	'--color-info': '#7dcfff',
	'--color-bg-primary': '#1a1b26',
	'--color-bg-secondary': '#16161e',
	'--color-bg-tertiary': '#24283b',
	'--color-bg-hover': '#292e42',
	'--color-text-primary': '#c0caf5',
	'--color-text-secondary': '#a9b1d6',
	'--color-text-muted': '#737aa2',
};

const NEO_DARK_TOKENS = {
	'--color-accent': '#faff69',
	'--color-accent-hover': '#e6eb52',
	'--color-success': '#22c55e',
	'--color-error': '#ef4444',
	'--color-warning': '#f59e0b',
	'--color-info': '#3b82f6',
	'--color-bg-primary': '#0a0a0a',
	'--color-bg-secondary': '#121212',
	'--color-bg-tertiary': '#1a1a1a',
	'--color-bg-hover': '#242424',
	'--color-text-primary': '#ffffff',
	'--color-text-secondary': '#cccccc',
	'--color-text-muted': '#888888',
};

describe('buildXtermTheme', () => {
	it('uses light palette tokens', () => {
		const theme = buildXtermTheme(LEGACY_LIGHT_TOKENS);

		expect(theme.background).toBe('#f0f4f8');
		expect(theme.foreground).toBe('#1a212c');
		expect(theme.cursor).toBe('#0096e1');
		expect(theme.black).toBe('#585858');
		expect(theme.brightRed).not.toBe(theme.red);
	});

	it('uses dark palette tokens', () => {
		const theme = buildXtermTheme(LEGACY_DARK_TOKENS);

		expect(theme.background).toBe('#1a1b26');
		expect(theme.foreground).toBe('#c0caf5');
		expect(theme.blue.toLowerCase()).not.toBe(theme.yellow.toLowerCase());
		expect(theme.brightBlue).not.toBe(theme.blue);
	});

	it('does not map ANSI blue to yellow Neo accent', () => {
		const theme = buildXtermTheme(NEO_DARK_TOKENS);

		expect(theme.cursor).toBe('#faff69');
		expect(theme.blue.toLowerCase()).not.toBe('#faff69');
		expect(theme.yellow.toLowerCase()).not.toBe('#faff69');
		expect(theme.brightBlue).not.toBe(theme.blue);
	});
});
