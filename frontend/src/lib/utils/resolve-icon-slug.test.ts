import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { resolveIconSlug, isPresetIconResolvable } from './resolve-icon-slug';

const catalogPath = resolve(process.cwd(), '../internal/presets/defaults.json');
const catalog = JSON.parse(readFileSync(catalogPath, 'utf8')) as {
	id: string;
	name: string;
	iconSlug: string;
}[];

// Names that MUST resolve to a brandIcons slug (so removing their inline
// duplicate in service-icons.ts is behavior-neutral).
const BRAND_PARITY: [name: string, slug: string][] = [
	['Telegram', 'telegram'],
	['YouTube', 'youtube'],
	['Google', 'google'],
	['WhatsApp', 'whatsapp'],
	['Facebook', 'facebook'],
	['Steam', 'steam'],
	['Discord', 'discord'],
	['GitHub', 'github'],
	['Samsung', 'samsung'],
	['Microsoft', 'microsoft'],
	['Spotify', 'spotify'],
	['Netflix', 'netflix'],
	['TikTok', 'tiktok'],
	['Twitch', 'twitch'],
	['Cloudflare', 'cloudflare'],
	['Roblox', 'roblox'],
	['Apple', 'apple'],
	['Twitter', 'x'],
	['Instagram', 'instagram'],
	// Alternate brand names that need an explicit alias (no compact match):
	['ChatGPT', 'openai'],
	['OpenAI', 'openai'],
	['x.com', 'x'],
];

describe('resolveIconSlug brand parity', () => {
	for (const [name, slug] of BRAND_PARITY) {
		it(`${name} → ${slug}`, () => {
			expect(resolveIconSlug(name)).toBe(slug);
			expect(isPresetIconResolvable(slug)).toBe(true);
		});
	}
});

describe('resolveIconSlug token match', () => {
		it('Cloudflare IPs → cloudflare', () => {
			expect(resolveIconSlug('Cloudflare IPs')).toBe('cloudflare');
		});

		it('Google Play → googleplay', () => {
			expect(resolveIconSlug('Google Play')).toBe('googleplay');
		});

		it('Google Play beats stored iconSlug google', () => {
			expect(resolveIconSlug('Google Play', 'google')).toBe('googleplay');
		});

		it('Google Play Store → googleplay (not generic google)', () => {
			expect(resolveIconSlug('Google Play Store')).toBe('googleplay');
		});

		it('Work VPN → lucide briefcase', () => {
			expect(resolveIconSlug('Work VPN')).toBe('lucide-briefcase-business');
		});

		it('Работа → lucide briefcase', () => {
			expect(resolveIconSlug('Работа')).toBe('lucide-briefcase-business');
		});
	});

describe('resolveIconSlug IP checker variants', () => {
	const globe = 'lucide-globe';

	for (const name of [
		'IP checkers',
		'IP checker',
		'IP-checkers',
		'IP-checker',
		'IP чекеры',
		'IP-чекеры',
		'IP-чекер',
		'ipcheckers',
		'ipchecker',
		'Маршрут IP checkers',
	]) {
		it(`${name} → globe`, () => {
			expect(resolveIconSlug(name)).toBe(globe);
		});
	}

	it('does not match unrelated names', () => {
		expect(resolveIconSlug('checkbox')).toBeUndefined();
		expect(resolveIconSlug('Spell checker')).toBeUndefined();
	});
});

describe('resolveIconSlug catalog name parity', () => {
	for (const p of catalog) {
		it(`${p.id} (${p.name}) → ${p.iconSlug}`, () => {
			expect(resolveIconSlug(p.name, undefined, catalog)).toBe(p.iconSlug);
		});
	}
});

describe('resolveIconSlug keeps custom (non-brandIcons) names unresolved', () => {
	// These have no brandIcons equivalent and must stay on the inline
	// keyword path (resolveIconSlug returns undefined).
	for (const name of ['VK', 'Mail', 'Почта', 'TMDB', 'Amazon', 'LinkedIn']) {
		it(`${name} → undefined`, () => {
			expect(resolveIconSlug(name)).toBeUndefined();
		});
	}
});
