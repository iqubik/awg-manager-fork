type ThemeTokenMap = Record<string, string>;

export type XtermTheme = {
	background: string;
	foreground: string;
	cursor: string;
	cursorAccent: string;
	selectionBackground: string;
	selectionForeground: string;
	black: string;
	red: string;
	green: string;
	yellow: string;
	blue: string;
	magenta: string;
	cyan: string;
	white: string;
	brightBlack: string;
	brightRed: string;
	brightGreen: string;
	brightYellow: string;
	brightBlue: string;
	brightMagenta: string;
	brightCyan: string;
	brightWhite: string;
};

function pick(tokens: ThemeTokenMap, key: string, fallback: string): string {
	return tokens[key] ?? fallback;
}

function parseHex(hex: string): [number, number, number] | null {
	const match = /^#([0-9a-f]{6})$/i.exec(hex.trim());
	if (!match) return null;
	return [0, 2, 4].map((index) => parseInt(match[1].slice(index, index + 2), 16)) as [
		number,
		number,
		number,
	];
}

function toHex([r, g, b]: [number, number, number]): string {
	return (
		'#' +
		[r, g, b].map((channel) => Math.round(channel).toString(16).padStart(2, '0')).join('')
	);
}

function mixHex(a: string, b: string, weight: number): string {
	const ca = parseHex(a);
	const cb = parseHex(b);
	if (!ca || !cb) return a;
	const w = Math.min(1, Math.max(0, weight));
	return toHex(
		ca.map((channel, index) => channel * (1 - w) + cb[index]! * w) as [number, number, number],
	);
}

function relativeLuminance(hex: string): number {
	const channels = parseHex(hex);
	if (!channels) return 0;

	const linear = channels.map((value) => {
		const channel = value / 255;
		return channel <= 0.03928 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
	});

	return 0.2126 * linear[0]! + 0.7152 * linear[1]! + 0.0722 * linear[2]!;
}

function hexHue(hex: string): number | null {
	const channels = parseHex(hex);
	if (!channels) return null;

	const [r, g, b] = channels.map((value) => value / 255);
	const max = Math.max(r, g, b);
	const min = Math.min(r, g, b);
	const delta = max - min;

	if (delta === 0) return null;

	let hue = 0;
	if (max === r) hue = ((g - b) / delta) % 6;
	else if (max === g) hue = (b - r) / delta + 2;
	else hue = (r - g) / delta + 4;

	return ((hue * 60) + 360) % 360;
}

function hueInRange(hex: string, start: number, end: number): boolean {
	const hue = hexHue(hex);
	if (hue == null) return false;
	if (start <= end) return hue >= start && hue <= end;
	return hue >= start || hue <= end;
}

function isDarkTheme(tokens: ThemeTokenMap): boolean {
	const bg = pick(tokens, '--color-bg-primary', '#1a1b26');
	const fg = pick(tokens, '--color-text-primary', '#c0caf5');
	return relativeLuminance(bg) < relativeLuminance(fg);
}

/** Pick a hue-appropriate token or fall back to a TUI-safe default. */
function pickByHue(
	candidates: Array<{ hex: string; ranges: Array<[number, number]> }>,
	fallbackDark: string,
	fallbackLight: string,
	dark: boolean,
): string {
	for (const { hex, ranges } of candidates) {
		if (ranges.some(([start, end]) => hueInRange(hex, start, end))) {
			return hex;
		}
	}
	return dark ? fallbackDark : fallbackLight;
}

function toneForTerminal(hex: string, dark: boolean): string {
	return dark ? mixHex(hex, '#ffffff', 0.1) : mixHex(hex, '#000000', 0.14);
}

function brightVariant(hex: string, dark: boolean): string {
	return dark ? mixHex(hex, '#ffffff', 0.28) : mixHex(hex, '#000000', 0.22);
}

/**
 * Build a 16-color xterm palette tuned for ncurses TUI apps (Midnight Commander, etc.).
 * UI accent is used for the cursor only — ANSI slots keep stable hues.
 */
export function buildXtermTheme(tokens: ThemeTokenMap): XtermTheme {
	const background = pick(tokens, '--color-bg-primary', '#1a1b26');
	const foreground = pick(tokens, '--color-text-primary', '#c0caf5');
	const accent = pick(tokens, '--color-accent', '#7aa2f7');
	const info = pick(tokens, '--color-info', '#7dcfff');
	const dark = isDarkTheme(tokens);

	const red = toneForTerminal(pick(tokens, '--color-error', dark ? '#e06c75' : '#c62828'), dark);
	const green = toneForTerminal(
		pick(tokens, '--color-success', dark ? '#98c379' : '#008000'),
		dark,
	);
	const yellow = toneForTerminal(
		pick(tokens, '--color-warning', dark ? '#e5c07b' : '#af8700'),
		dark,
	);
	const blue = toneForTerminal(
		pickByHue(
			[
				{ hex: accent, ranges: [[200, 250]] },
				{ hex: info, ranges: [[200, 250]] },
			],
			'#61afef',
			'#005fd7',
			dark,
		),
		dark,
	);
	const magenta = toneForTerminal(
		pickByHue([{ hex: accent, ranges: [[280, 330]] }], '#c678dd', '#af005f', dark),
		dark,
	);
	const cyan = toneForTerminal(
		pickByHue(
			[
				{ hex: info, ranges: [[165, 200]] },
				{ hex: accent, ranges: [[165, 200]] },
			],
			'#56b6c2',
			'#008787',
			dark,
		),
		dark,
	);

	const black = dark ? mixHex(background, '#000000', 0.55) : '#585858';
	const white = dark ? mixHex(foreground, '#ffffff', 0.18) : '#e5e5e5';
	const brightBlack = dark ? mixHex(foreground, background, 0.55) : '#767676';
	const brightWhite = dark ? mixHex(foreground, '#ffffff', 0.42) : '#ffffff';

	return {
		background,
		foreground,
		cursor: accent,
		cursorAccent: background,
		selectionBackground: pick(tokens, '--color-bg-hover', dark ? '#33467c' : '#c8d6e8'),
		selectionForeground: foreground,
		black,
		red,
		green,
		yellow,
		blue,
		magenta,
		cyan,
		white,
		brightBlack,
		brightRed: brightVariant(red, dark),
		brightGreen: brightVariant(green, dark),
		brightYellow: brightVariant(yellow, dark),
		brightBlue: brightVariant(blue, dark),
		brightMagenta: brightVariant(magenta, dark),
		brightCyan: brightVariant(cyan, dark),
		brightWhite,
	};
}
