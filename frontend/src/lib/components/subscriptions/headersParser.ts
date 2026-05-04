import type { SubscriptionHeader } from '$lib/types';

export function parseHeadersText(text: string): SubscriptionHeader[] {
	const lines = text.split('\n');
	const out: SubscriptionHeader[] = [];
	for (const raw of lines) {
		const line = raw.trim();
		if (!line || line.startsWith('#')) continue;
		const idx = line.indexOf(':');
		if (idx <= 0) continue;
		const name = line.slice(0, idx).trim();
		const value = line.slice(idx + 1).trim();
		if (name && value) out.push({ name, value });
	}
	return out;
}

export function serializeHeaders(headers: SubscriptionHeader[]): string {
	return headers.map((h) => `${h.name}: ${h.value}`).join('\n');
}

export const HAPP_PRESET = `User-Agent: Happ/4.6.0/ios/2603181556604
X-Device-OS: iOS
X-HWID: d1c1da1b1b111111
X-Device-Locale: ru
X-Ver-OS: 26.4
X-App-Version: 4.6.0
X-Device-Model: iPhone 17 Pro Max`;
