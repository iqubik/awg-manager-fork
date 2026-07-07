import { describe, expect, it } from 'vitest';
import type { SingboxRouterRuleSet } from '$lib/types';
import {
	applyGeoTagToggle,
	buildRuleSetPayload,
	createRuleSetDraftState,
	standardDatRuleSetTag,
} from './ruleSetAddModalLogic';

describe('ruleSetAddModalLogic', () => {
	it('opens existing dat-srs remote rule set in geosite edit mode', () => {
		const draft = createRuleSetDraftState({
			tag: 'geosite-GOOGLE',
			type: 'remote',
			format: 'binary',
			url: 'http://127.0.0.1:2222/api/singbox/router/rulesets/dat-srs?kind=geosite&tag=GOOGLE&token=test',
			update_interval: '24h',
		} satisfies SingboxRouterRuleSet);

		expect(draft.type).toBe('geosite');
		expect(draft.selectedGeoTags).toEqual(['GOOGLE']);
		expect(draft.tag).toBe('geosite-GOOGLE');
	});

	it('keeps a custom existing dat tag after picking another geo tag', () => {
		const next = applyGeoTagToggle({
			type: 'geosite',
			tag: 'custom-name',
			selectedGeoTags: ['OLD'],
			pickedTag: 'GOOGLE',
		});

		expect(next.selectedGeoTags).toEqual(['OLD', 'GOOGLE']);
		expect(next.tag).toBe('custom-name');
	});

	it('updates an auto-generated dat tag to the normalized multi-tag name', () => {
		const next = applyGeoTagToggle({
			type: 'geosite',
			tag: 'geosite-old',
			selectedGeoTags: ['OLD'],
			pickedTag: 'GOOGLE',
		});

		expect(next.selectedGeoTags).toEqual(['OLD', 'GOOGLE']);
		expect(next.tag).toBe('geosite-old-google');
		expect(standardDatRuleSetTag('geosite', next.selectedGeoTags)).toBe('geosite-old-google');
	});

	it('builds geosite selections as one remote binary dat-srs rule set', async () => {
		const payload = await buildRuleSetPayload({
			type: 'geosite',
			format: 'source',
			tag: 'geosite-google-youtube',
			url: '',
			updateInterval: '6h',
			downloadDetour: 'warp',
			path: '',
			selectedGeoTags: ['GOOGLE', 'YOUTUBE'],
			resolveDatRuleSetURL: async (kind, tags) => {
				const query = new URLSearchParams({ kind });
				for (const tag of tags) query.append('tag', tag);
				query.set('token', 'test');
				return `http://127.0.0.1:2222/api/singbox/router/rulesets/dat-srs?${query.toString()}`;
			},
		});

		expect(payload).toEqual({
			tag: 'geosite-google-youtube',
			type: 'remote',
			format: 'binary',
			url: 'http://127.0.0.1:2222/api/singbox/router/rulesets/dat-srs?kind=geosite&tag=GOOGLE&tag=YOUTUBE&token=test',
			update_interval: '24h',
			download_detour: undefined,
			path: undefined,
			rules: undefined,
		});
	});
});
