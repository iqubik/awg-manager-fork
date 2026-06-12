export const STALE_ASSET_RELOAD_KEY = 'awgm:stale-asset-reload-at';
const STALE_ASSET_RELOAD_DEBOUNCE_MS = 15_000;

function staleAssetErrorMessage(value: unknown): string {
	if (value instanceof Error) {
		return value.message;
	}

	if (typeof value === 'string') {
		return value;
	}

	return String((value as { message?: unknown })?.message ?? value ?? '');
}

export function isStaleAssetError(value: unknown): boolean {
	const message = staleAssetErrorMessage(value);

	return (
		message.includes('Failed to fetch dynamically imported module') ||
		message.includes('Importing a module script failed') ||
		message.includes('/_app/immutable/')
	);
}

export function shouldReloadForStaleAssets(
	storage: Pick<Storage, 'getItem' | 'setItem'>,
	now = Date.now(),
): boolean {
	const lastRaw = storage.getItem(STALE_ASSET_RELOAD_KEY);
	const last = Number(lastRaw || '0');

	if (lastRaw && now - last < STALE_ASSET_RELOAD_DEBOUNCE_MS) {
		return false;
	}

	storage.setItem(STALE_ASSET_RELOAD_KEY, String(now));
	return true;
}

export function reloadOnceForStaleAssets(reason: string): void {
	let shouldReload = true;

	try {
		shouldReload = shouldReloadForStaleAssets(sessionStorage);
	} catch {
		shouldReload = true;
	}

	if (!shouldReload) {
		return;
	}

	console.warn(`[awgm] stale frontend assets detected, reloading: ${reason}`);
	location.reload();
}
