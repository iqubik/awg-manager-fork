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

// Empty mihomo / Clash template — every recognised header listed but
// blank, so the user only fills in what their provider actually checks.
// Lines with empty values are silently dropped by parseHeadersText.
export const MIHOMO_PRESET = `# Шаблон Clash / mihomo. Заполни нужные строки и сохрани.
# Пустые строки и строки с # игнорируются.
User-Agent: mihomo/v1.19.20
X-HWID:
X-Device-OS: Android
X-Device-Locale: ru
X-Device-Model:
X-Ver-OS:
X-App-Version:
Accept-Encoding: gzip
X-Real-IP:
X-Forwarded-For:
# Connection / Host / Content-Length / Transfer-Encoding / Upgrade
# управляются Go-клиентом и игнорируются.`;
