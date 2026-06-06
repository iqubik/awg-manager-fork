import {
	Shuffle,
	Home,
	Shield,
	Route,
	Wrench,
	UserRound,
	Briefcase,
	Gamepad2,
	Tv,
	Panda,
	Plug,
	Layers,
	Unplug,
	FlaskConical,
	Copy,
	Wifi,
	Heart,
	Server,
} from 'lucide-svelte';

/** Lucide icon id resolved from policy description / name. */
export type PolicyIconId =
	| 'shuffle'
	| 'home'
	| 'shield'
	| 'route'
	| 'tools'
	| 'guest'
	| 'kids'
	| 'work'
	| 'gaming'
	| 'tv'
	| 'iot'
	| 'hydraroute'
	| 'direct'
	| 'test'
	| 'backup'
	| 'wifi'
	| 'parents'
	| 'server'
	| 'north_korea';

type PolicyIconComponent = typeof Shuffle;

/** Inline SVG art (flags) — rendered instead of Lucide when set. */
export const POLICY_INLINE_SVG: Partial<
	Record<PolicyIconId, { viewBox: string; paths: string[] }>
> = {
	north_korea: {
		viewBox: '0 0 64 64',
		paths: [
			'M16.362 29.795l-5.995.018l4.842 3.721l-1.839 6.003l4.863-3.691l4.863 3.691l-1.842-6.003l4.846-3.721l-5.998-.018l-1.869-5.99z',
			'M32 2C15.432 2 2 15.432 2 32s13.432 30 30 30s30-13.432 30-30S48.568 2 32 2M9.551 48.717a28.193 28.193 0 0 1-2.45-3.934H56.9a28.251 28.251 0 0 1-2.45 3.934H9.551m44.897-33.434a28.041 28.041 0 0 1 2.45 3.934H7.102a28.1 28.1 0 0 1 2.45-3.934h44.896M29.05 32c0 5.974-4.844 10.817-10.816 10.817c-5.975 0-10.816-4.843-10.816-10.817s4.842-10.817 10.816-10.817c5.972 0 10.816 4.842 10.816 10.817',
		],
	},
};

export function getPolicyInlineSvg(
	id: PolicyIconId,
): { viewBox: string; paths: string[] } | undefined {
	return POLICY_INLINE_SVG[id];
}

export const POLICY_ICON_COMPONENTS: Record<
	Exclude<PolicyIconId, keyof typeof POLICY_INLINE_SVG>,
	PolicyIconComponent
> = {
	shuffle: Shuffle,
	home: Home,
	shield: Shield,
	route: Route,
	tools: Wrench,
	guest: UserRound,
	kids: Panda,
	work: Briefcase,
	gaming: Gamepad2,
	tv: Tv,
	iot: Plug,
	hydraroute: Layers,
	direct: Unplug,
	test: FlaskConical,
	backup: Copy,
	wifi: Wifi,
	parents: Heart,
	server: Server,
};

function escapeRegExp(s: string): string {
	return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/** Case-insensitive: descriptions may be HOME, WiFi_Guest, VPN, etc. */
function normalizeLabel(label: string): string {
	return label.trim().toLocaleLowerCase('en').replace(/[-.]/g, '_');
}

function tokenize(normalized: string): string[] {
	return normalized.split(/[_\s]+/).filter(Boolean);
}

function normalizeKeyword(keyword: string): string {
	return keyword.trim().toLocaleLowerCase('en').replace(/-/g, '_');
}

/** Match keyword as a token or as a _/-delimited segment (avoids `dom` in `freedom`). */
function matchesKeyword(keyword: string, normalized: string, tokens: string[]): boolean {
	const kw = normalizeKeyword(keyword);
	if (!kw) return false;
	if (kw.includes('_')) {
		return normalized.includes(kw);
	}
	if (tokens.includes(kw)) return true;
	const re = new RegExp(`(^|[_-])${escapeRegExp(kw)}([_-]|$)`);
	return re.test(normalized);
}

/** First matching rule wins (most specific keywords first). */
const POLICY_ICON_RULES: { icon: PolicyIconId; keywords: string[] }[] = [
	{ icon: 'hydraroute', keywords: ['hydraroute', 'hr_neo', 'hrneo', 'hrus', 'hr'] },
	{ icon: 'north_korea', keywords: ['north_korea', 'northkorea', 'dprk'] },
	{
		icon: 'direct',
		keywords: [
			'domru',
			'dom_ru',
			'beeline',
			'mts',
			'megafon',
			'megafoon',
			'rostelecom',
			'rostele',
			'rtk',
			'rek',
			'provider',
			'provayder',
			'провайдер',
		],
	},
	{
		icon: 'server',
		keywords: [
			'docker',
			'lxc',
			'vm',
			'virtual_machine',
			'server',
			'srv',
			'homelab',
			'home_lab',
			'selfhosted',
			'self_hosted',
			'nas',
			'homeassistant',
			'home_assistant',
			'plex',
			'jellyfin',
			'emby',
			'synology',
			'qnap',
			'proxmox',
			'unraid',
			'truenas',
			'pihole',
			'pi_hole',
		],
	},
	{
		icon: 'tools',
		keywords: ['nfqws', 'zapret', 'dpi_bypass', 'dpi', 'service'],
	},
	{
		icon: 'route',
		keywords: [
			'magitrickle',
			'magicitrickle',
			'singbox',
			'sing_box',
			'sb_router',
			'split',
			'splitrouting',
			'split_route',
			'sbr',
			'xkeen',
		],
	},
	{ icon: 'iot', keywords: ['smarthome', 'smart_home', 'iot', 'yandex', 'alice', 'alisa'] },
	{
		icon: 'shield',
		keywords: [
			'amnezia',
			'awgm',
			'tunnel',
			'awg_manager',
			'awg-manager',
			'awg',
			'wireguard',
			'vless',
			'adguard',
			'vmess',
			'trojan',
			'warp',
			'clash',
			'xray',
			'hysteria',
			'hy2',
			'wg',
			'vpn',
			'proxyru',
			'proxymir',
		],
	},
	{ icon: 'guest', keywords: ['wifi_guest', 'guest_wifi', 'guest', 'gost'] },
	{ icon: 'kids', keywords: ['kids', 'child'] },
	{ icon: 'parents', keywords: ['parents', 'babushka', 'family', 'wife', 'husband'] },
	{ icon: 'gaming', keywords: ['gaming', 'gamepad', 'ps5', 'xbox', 'steam'] },
	{ icon: 'tv', keywords: ['stream', 'media', 'tv', 'tube', 'kino'] },
	{ icon: 'work', keywords: ['office', 'corp', 'work', 'job', 'business'] },
	{ icon: 'home', keywords: ['default', 'home', 'dom', 'house', 'floor', 'apartment'] },
	{
		icon: 'direct',
		keywords: [
			'direct',
			'wan',
			'isp',
			'internet',
			'russia',
			'rus',
			'ru',
			'no_inet',
			'noinet',
			'south_korea',
			'korea',
			'only_makeitgreatagain',
		],
	},
	{ icon: 'test', keywords: ['test', 'tmp', 'dev'] },
	{ icon: 'backup', keywords: ['failover', 'backup', 'reserve'] },
	{ icon: 'wifi', keywords: ['wifi'] },
];

function resolveFromLabel(label: string): PolicyIconId | undefined {
	const normalized = normalizeLabel(label);
	const tokens = tokenize(normalized);

	for (const rule of POLICY_ICON_RULES) {
		if (rule.keywords.some((kw) => matchesKeyword(kw, normalized, tokens))) {
			return rule.icon;
		}
	}
	return undefined;
}

/**
 * Pick a Lucide icon for an access-policy card from description or policy name.
 * Pass `isHydraRoute` so HR-managed policies without description still get an icon.
 */
export function resolvePolicyIcon(
	label: string,
	options?: { policyName?: string; isHydraRoute?: boolean },
): PolicyIconId {
	const desc = label.trim();
	const name = options?.policyName?.trim() ?? '';

	for (const source of [desc, name]) {
		if (!source) continue;
		const matched = resolveFromLabel(source);
		if (matched) return matched;
	}

	if (options?.isHydraRoute) return 'hydraroute';
	return 'shuffle';
}

export function getPolicyIconComponent(id: PolicyIconId): PolicyIconComponent | undefined {
	if (getPolicyInlineSvg(id)) return undefined;
	return POLICY_ICON_COMPONENTS[id as keyof typeof POLICY_ICON_COMPONENTS];
}
