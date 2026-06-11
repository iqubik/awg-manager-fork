import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  applyCatalogPresetsAsRuleSets,
  fullyAddedPresetNames,
} from './rulesetCatalogActions';
import type { CatalogPreset, SingboxRouterRuleSet } from '$lib/types';

vi.mock('$lib/api/client', () => ({
  api: {
    singboxRouterAddRuleSet: vi.fn(),
  },
}));

import { api } from '$lib/api/client';

const netflixPreset: CatalogPreset = {
  id: 'netflix',
  name: 'Netflix',
  iconSlug: 'netflix',
  category: 'media',
  origin: 'builtin',
  engines: {
    singbox: {
      action: 'tunnel',
      ruleSets: [{ tag: 'geosite-netflix', url: 'https://example.com/netflix.srs' }],
    },
  },
};

const youtubePreset: CatalogPreset = {
  id: 'youtube',
  name: 'YouTube',
  iconSlug: 'youtube',
  category: 'media',
  origin: 'builtin',
  engines: {
    singbox: {
      action: 'tunnel',
      ruleSets: [{ tag: 'geosite-youtube', url: 'https://example.com/youtube.srs' }],
    },
  },
};

const dnsOnlyPreset: CatalogPreset = {
  id: 'dns-only',
  name: 'DNS only',
  iconSlug: 'dns',
  category: 'media',
  origin: 'builtin',
  engines: {
    dns: { domains: ['example.com'] },
  },
};

describe('fullyAddedPresetNames', () => {
  it('returns names when all preset rule sets exist', () => {
    const names = fullyAddedPresetNames(
      [netflixPreset, youtubePreset],
      new Set(['geosite-netflix']),
    );
    expect(names).toEqual(['Netflix']);
  });

  it('ignores presets without singbox engine', () => {
    const names = fullyAddedPresetNames([dnsOnlyPreset], new Set(['geosite-netflix']));
    expect(names).toEqual([]);
  });
});

describe('applyCatalogPresetsAsRuleSets', () => {
  beforeEach(() => {
    vi.mocked(api.singboxRouterAddRuleSet).mockReset();
    vi.mocked(api.singboxRouterAddRuleSet).mockResolvedValue(undefined);
  });

  it('adds missing remote rule sets from selected presets', async () => {
    const existing: SingboxRouterRuleSet[] = [
      { tag: 'geosite-netflix', type: 'remote', url: 'https://example.com/netflix.srs' },
    ];

    const result = await applyCatalogPresetsAsRuleSets([youtubePreset], existing);

    expect(api.singboxRouterAddRuleSet).toHaveBeenCalledTimes(1);
    expect(api.singboxRouterAddRuleSet).toHaveBeenCalledWith({
      tag: 'geosite-youtube',
      type: 'remote',
      format: 'binary',
      url: 'https://example.com/youtube.srs',
      update_interval: '24h',
    });
    expect(result.added).toEqual(['geosite-youtube']);
    expect(result.skipped).toEqual([]);
    expect(result.failures).toEqual([]);
  });

  it('skips tags that already exist', async () => {
    const existing: SingboxRouterRuleSet[] = [
      { tag: 'geosite-netflix', type: 'remote', url: 'https://example.com/netflix.srs' },
    ];

    const result = await applyCatalogPresetsAsRuleSets([netflixPreset], existing);

    expect(api.singboxRouterAddRuleSet).not.toHaveBeenCalled();
    expect(result.added).toEqual([]);
    expect(result.skipped).toEqual(['geosite-netflix']);
  });

  it('records presets without singbox rule sets as empty', async () => {
    const result = await applyCatalogPresetsAsRuleSets([dnsOnlyPreset], []);

    expect(api.singboxRouterAddRuleSet).not.toHaveBeenCalled();
    expect(result.emptyPresets).toEqual(['dns-only']);
  });

  it('collects failures without stopping the batch', async () => {
    vi.mocked(api.singboxRouterAddRuleSet)
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce(undefined);

    const result = await applyCatalogPresetsAsRuleSets([netflixPreset, youtubePreset], []);

    expect(result.failures).toEqual([{ tag: 'geosite-netflix', error: 'boom' }]);
    expect(result.added).toEqual(['geosite-youtube']);
  });
});
