import { browser } from '$app/environment';
import type { AmneziaPremiumIssuedConfig } from '$lib/types';
import { classifyVpnLink, isVpnLink } from '$lib/utils/vpnlink';

export type VpnPastePresentation =
	| { kind: 'neutral'; label: string }
	| { kind: 'regular'; label: string }
	| { kind: 'premium'; label: string };

export function getVpnPastePresentation(raw: string): VpnPastePresentation {
	const trimmed = raw.trim();
	if (!trimmed || !isVpnLink(trimmed)) {
		return { kind: 'neutral', label: 'Вставить ссылку' };
	}
	if (classifyVpnLink(trimmed) === 'regular') {
		return { kind: 'regular', label: 'Вставить ссылку' };
	}
	return { kind: 'premium', label: 'Amnezia Premium' };
}

export function shouldShowPremiumChrome(raw: string): boolean {
	const trimmed = raw.trim();
	if (!trimmed || !isVpnLink(trimmed)) return false;
	return classifyVpnLink(trimmed) !== 'regular';
}

/** Единый ключ localStorage для «Запомнить ключ» (создание и замена туннеля). */
export const PREMIUM_VPN_KEY_STORAGE = 'awgm.tunnels.premiumVpnKey';

const LEGACY_PREMIUM_VPN_KEY_STORAGES = [
	'awgm.tunnels.new.premiumVpnKey',
	'awgm.tunnels.replace.premiumVpnKey'
] as const;

export function readStoredPremiumVpnKey(
	storageKey: string = PREMIUM_VPN_KEY_STORAGE
): string | null {
	if (!browser) return null;
	try {
		const direct = localStorage.getItem(storageKey)?.trim();
		if (direct) return direct;

		for (const legacy of LEGACY_PREMIUM_VPN_KEY_STORAGES) {
			if (legacy === storageKey) continue;
			const migrated = localStorage.getItem(legacy)?.trim();
			if (!migrated) continue;
			try {
				localStorage.setItem(storageKey, migrated);
				localStorage.removeItem(legacy);
			} catch {
				/* keep legacy value readable */
			}
			return migrated;
		}

		return null;
	} catch {
		return null;
	}
}

export function savePremiumVpnKeyToStorage(storageKey: string, key: string): void {
	localStorage.setItem(storageKey, key);
}

export function clearStoredPremiumVpnKeyFromStorage(storageKey: string): void {
	localStorage.removeItem(storageKey);
}

export function premiumIssuedConfigSourceType(ic: AmneziaPremiumIssuedConfig): string {
	return String(ic.source_type ?? '').trim().toLowerCase();
}

export function isPremiumIssuedConfigActiveDevice(ic: AmneziaPremiumIssuedConfig): boolean {
	return premiumIssuedConfigSourceType(ic) === 'gateway_account';
}

export function isPremiumIssuedConfigReissuable(ic: AmneziaPremiumIssuedConfig): boolean {
	return !isPremiumIssuedConfigActiveDevice(ic);
}

function premiumCountryCode(value: unknown): string {
	return String(value ?? '').trim().toLowerCase();
}

export function premiumIssuedConfigsForCountry(
	issued: AmneziaPremiumIssuedConfig[],
	code: string
): AmneziaPremiumIssuedConfig[] {
	const cc = premiumCountryCode(code);
	return issued.filter((ic) => {
		if (!isPremiumIssuedConfigReissuable(ic)) return false;
		return premiumCountryCode(ic.server_country_code) === cc;
	});
}

export function premiumActiveDevicesForCountry(
	issued: AmneziaPremiumIssuedConfig[],
	code: string
): AmneziaPremiumIssuedConfig[] {
	const cc = premiumCountryCode(code);
	return issued.filter((ic) => {
		if (!isPremiumIssuedConfigActiveDevice(ic)) return false;
		return premiumCountryCode(ic.server_country_code) === cc;
	});
}

export function isPremiumCountryIssued(issued: AmneziaPremiumIssuedConfig[], code: string): boolean {
	return premiumIssuedConfigsForCountry(issued, code).length > 0;
}

/** worker_last_updated позже last_downloaded — адрес на сервере меняли после последней выдачи. */
export function isPremiumCountryConfigStale(issued: AmneziaPremiumIssuedConfig[], code: string): boolean {
	return premiumIssuedConfigsForCountry(issued, code).some((ic) => {
		const workerRaw = ic.worker_last_updated?.trim();
		const downloadedRaw = ic.last_downloaded?.trim();
		if (!workerRaw || !downloadedRaw) return false;
		const workerMs = Date.parse(workerRaw);
		const downloadedMs = Date.parse(downloadedRaw);
		if (!Number.isFinite(workerMs) || !Number.isFinite(downloadedMs)) return false;
		return workerMs > downloadedMs;
	});
}
