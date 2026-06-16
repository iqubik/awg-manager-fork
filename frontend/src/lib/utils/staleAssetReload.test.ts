import { describe, expect, it } from 'vitest';

import {
	STALE_ASSET_RELOAD_KEY,
	isStaleAssetError,
	shouldReloadForStaleAssets,
} from './staleAssetReload';

describe('staleAssetReload', () => {
	it('detects stale asset import failures', () => {
		expect(isStaleAssetError(new Error('Failed to fetch dynamically imported module'))).toBe(true);
		expect(isStaleAssetError('Importing a module script failed')).toBe(true);
		expect(isStaleAssetError('GET /_app/immutable/chunks/app.js net::ERR_CONNECTION_REFUSED')).toBe(true);
		expect(isStaleAssetError('random network issue')).toBe(false);
	});

	it('allows one reload per debounce window', () => {
		const storage = new Map<string, string>();
		const session = {
			getItem: (key: string) => storage.get(key) ?? null,
			setItem: (key: string, value: string) => {
				storage.set(key, value);
			},
		};

		expect(shouldReloadForStaleAssets(session, 1000)).toBe(true);
		expect(storage.get(STALE_ASSET_RELOAD_KEY)).toBe('1000');
		expect(shouldReloadForStaleAssets(session, 5000)).toBe(false);
		expect(shouldReloadForStaleAssets(session, 17000)).toBe(true);
		expect(storage.get(STALE_ASSET_RELOAD_KEY)).toBe('17000');
	});
});
