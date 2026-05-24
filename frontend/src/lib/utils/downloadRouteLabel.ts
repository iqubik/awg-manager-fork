import type { DownloadOutbound } from '$lib/types';

function maskSensitiveToken(token: string): string {
	if (!token) return token;
	if (token.length <= 6) return '*'.repeat(token.length);
	return `${token.slice(0, 3)}${'*'.repeat(token.length - 6)}${token.slice(-3)}`;
}

export function maskSensitiveInText(text: string): string {
	if (!text) return text;
	const hostLike = /\b([a-zA-Z0-9-]+(?:\.[a-zA-Z0-9-]+)+|\d{1,3}(?:\.\d{1,3}){3})\b/g;
	return text.replace(hostLike, (m) => maskSensitiveToken(m));
}

export function kindLabel(kind?: string): string {
	const k = (kind ?? '').trim();
	if (!k) return '';
	if (k.toLowerCase() === 'subscription') return 'SUB';
	if (k.toLowerCase() === 'singbox') return 'SING';
	return k.toUpperCase();
}

export function displayRouteName(label: string, kind?: string): string {
	const base = maskSensitiveInText(label);
	const k = kindLabel(kind);
	if (!k || k.toLowerCase() === 'direct') return base;
	if (base.toUpperCase().includes(`(${k})`) || base.toUpperCase().endsWith(`- ${k}`)) {
		return base;
	}
	return `${base} (${k})`;
}

export function displayOutboundName(ob: Pick<DownloadOutbound, 'label' | 'kind'>): string {
	return displayRouteName(ob.label, ob.kind);
}
