import { brandIcons } from '$lib/generated/brandIcons';
import type { CatalogPreset } from '$lib/types';
import { isPresetInlineSlug } from '$lib/utils/service-icons';

export type IconSlugCatalogRef = Pick<CatalogPreset, 'name' | 'iconSlug'>;

const LUCIDE_SLUGS = new Set([
	'lucide-circle-slash',
	'lucide-globe',
	'lucide-sparkles',
	'lucide-film',
	'lucide-gamepad-2',
	'lucide-shield-check',
	'lucide-shield-alert',
	'lucide-cpu',
	'lucide-eye-off',
	'lucide-lock',
	'lucide-briefcase-business',
	'lucide-shield-off',
	'lucide-globe-lock',
]);

/** Name aliases → sing-box preset iconSlug (brandIcons / lucide). */
const NAME_ALIASES: Record<string, string> = {
	'twitter/x': 'x',
	twitter: 'x',
	chatgpt: 'openai',
	'x.com': 'x',
	'google gemini': 'googlegemini',
	'google-gemini': 'googlegemini',
	gemini: 'googlegemini',
	'google play': 'googleplay',
	'google-play': 'googleplay',
	торренты: 'torrents',
	'заблокировано в рф': 'rkn',
	'реклама и трекеры': 'lucide-circle-slash',
	'недоступно из рф': 'lucide-globe-lock',
	atlassian: 'atlassian',
	bitbucket: 'atlassian',
	jira: 'atlassian',
	confluence: 'atlassian',
	ubisoft: 'ubisoft',
	yandex: 'yandex',
	яндекс: 'yandex',
	alice: 'yandex',
	alisa: 'yandex',
	алиса: 'yandex',
	disney: 'disney',
	'disney+': 'disney',
	work: 'lucide-briefcase-business',
	работа: 'lucide-briefcase-business',
	'ip checkers': 'lucide-globe',
	'ip checker': 'lucide-globe',
	'ip-checkers': 'lucide-globe',
	'ip-checker': 'lucide-globe',
	'ip чекеры': 'lucide-globe',
	'ip-чекеры': 'lucide-globe',
	'ip чекер': 'lucide-globe',
	'ip-чекер': 'lucide-globe',
	ipcheckers: 'lucide-globe',
	ipchecker: 'lucide-globe',
	'российские сервисы': 'rkn',
	'russian services': 'rkn',
	'роскомнадзор': 'rkn',
};

function normalizeKey(s: string): string {
	return s.trim().toLowerCase().replace(/^geo(site|ip)[-_]/, '');
}

/** True when PresetIcon can render this slug (not the "?" fallback). */
export function isPresetIconResolvable(slug: string): boolean {
	if (!slug) return false;
	if (isPresetInlineSlug(slug) || LUCIDE_SLUGS.has(slug)) return true;
	return slug in brandIcons;
}

function resolveFromCatalog(name: string, catalog: IconSlugCatalogRef[]): string | undefined {
	const key = name.trim().toLowerCase();
	if (!key) return undefined;
	for (const preset of catalog) {
		if (preset.name.trim().toLowerCase() !== key) continue;
		if (preset.iconSlug && isPresetIconResolvable(preset.iconSlug)) return preset.iconSlug;
	}
	return undefined;
}

/** IP leak check services — globe icon (matches catalog preset ip-checkers). */
function resolveFromIpCheckerPhrase(key: string): string | undefined {
	if (!/\bip\b/i.test(key)) return undefined;
	if (!/check|чекер/i.test(key)) return undefined;
	return isPresetIconResolvable('lucide-globe') ? 'lucide-globe' : undefined;
}

/** Google Play — must beat generic `google` (stored iconSlug or token match). */
function resolveFromGooglePlayPhrase(key: string): string | undefined {
	if (!isPresetIconResolvable('googleplay')) return undefined;
	if (/\bgoogleplay\b/i.test(key)) return 'googleplay';
	if (/\bgoogle[\s-]+play\b/i.test(key)) return 'googleplay';
	return undefined;
}

/** Longest-first tokens (min 3 chars) — e.g. "Cloudflare IPs" → cloudflare. */
function resolveFromNameTokens(key: string): string | undefined {
	const googlePlay = resolveFromGooglePlayPhrase(key);
	if (googlePlay) return googlePlay;

	const tokens = key
		.split(/[^a-z0-9]+/)
		.filter((t) => t.length >= 3)
		.sort((a, b) => b.length - a.length);
	const hasPlayToken = tokens.includes('play');
	for (const token of tokens) {
		if (hasPlayToken && token === 'google') continue;
		const alias = NAME_ALIASES[token];
		if (alias && isPresetIconResolvable(alias)) return alias;
		if (isPresetIconResolvable(token)) return token;
	}
	return undefined;
}

/**
 * Resolve iconSlug for PresetIcon from explicit slug, catalog preset, or rule name.
 * Matches sing-box / unified routing catalog conventions.
 */
export function resolveIconSlug(
	name: string,
	iconSlug?: string,
	catalog?: IconSlugCatalogRef[],
): string | undefined {
	const key = normalizeKey(name);

	if (key) {
		const googlePlay = resolveFromGooglePlayPhrase(key);
		if (googlePlay) return googlePlay;
	}

	if (catalog?.length) {
		const fromCatalog = resolveFromCatalog(name, catalog);
		if (fromCatalog) return fromCatalog;
	}

	if (!key) return undefined;

	if (iconSlug && isPresetIconResolvable(iconSlug)) return iconSlug;

	if (NAME_ALIASES[key] && isPresetIconResolvable(NAME_ALIASES[key])) {
		return NAME_ALIASES[key];
	}

	const ipChecker = resolveFromIpCheckerPhrase(key);
	if (ipChecker) return ipChecker;

	const compact = key.replace(/[^a-z0-9]+/g, '');
	if (compact && isPresetIconResolvable(compact)) return compact;

	return resolveFromNameTokens(key);
}
