import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('$lib/api/client', () => ({
  api: { singboxRouterPutSettings: vi.fn() },
}));

vi.mock('$lib/stores/singboxRouter', () => {
  const settings = { subscribe: vi.fn(() => () => {}) };
  return {
    singboxRouter: {
      settings,
      loadAll: vi.fn(async () => {}),
    },
  };
});

vi.mock('svelte/store', async () => {
  const actual = await vi.importActual<typeof import('svelte/store')>('svelte/store');
  return {
    ...actual,
    get: vi.fn(() => null),
  };
});

import { get } from 'svelte/store';
import { api } from '$lib/api/client';
import { singboxRouter } from '$lib/stores/singboxRouter';
import { BYPASS_PRESETS, mergeAndSaveSettings } from './settingsActions';

describe('settingsActions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('BYPASS_PRESETS has 3 entries with correct ids', () => {
    expect(BYPASS_PRESETS).toHaveLength(3);
    const ids = BYPASS_PRESETS.map((p) => p.id).sort();
    expect(ids).toEqual(['l2tp', 'netbios-smb', 'ntp']);
    for (const p of BYPASS_PRESETS) {
      expect(p.label).toBeTruthy();
      expect(p.desc).toBeTruthy();
    }
  });

  it('mergeAndSaveSettings: null current + patch → put with patch only', async () => {
    (get as ReturnType<typeof vi.fn>).mockReturnValue(null);
    await mergeAndSaveSettings({ deviceMode: 'all' });
    expect(api.singboxRouterPutSettings).toHaveBeenCalledWith(
      expect.objectContaining({ deviceMode: 'all' }),
    );
    expect(singboxRouter.loadAll).toHaveBeenCalled();
  });

  it('mergeAndSaveSettings: preserves existing fields', async () => {
    (get as ReturnType<typeof vi.fn>).mockReturnValue({
      enabled: true,
      policyName: 'awgm-router',
      snifferEnabled: false,
      wanAutoDetect: true,
    });
    await mergeAndSaveSettings({ snifferEnabled: true });
    expect(api.singboxRouterPutSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        enabled: true,
        policyName: 'awgm-router',
        snifferEnabled: true,
        wanAutoDetect: true,
      }),
    );
  });

  it('mergeAndSaveSettings: patch overrides existing field', async () => {
    (get as ReturnType<typeof vi.fn>).mockReturnValue({
      wanAutoDetect: true,
      wanInterface: '',
    });
    await mergeAndSaveSettings({ wanAutoDetect: false, wanInterface: 'ppp0' });
    expect(api.singboxRouterPutSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        wanAutoDetect: false,
        wanInterface: 'ppp0',
      }),
    );
  });

  it('mergeAndSaveSettings: bypass presets array', async () => {
    (get as ReturnType<typeof vi.fn>).mockReturnValue(null);
    await mergeAndSaveSettings({ bypassPresets: ['l2tp', 'ntp'] });
    expect(api.singboxRouterPutSettings).toHaveBeenCalledWith(
      expect.objectContaining({ bypassPresets: ['l2tp', 'ntp'] }),
    );
  });

  it('mergeAndSaveSettings: re-throws API error', async () => {
    (get as ReturnType<typeof vi.fn>).mockReturnValue(null);
    (api.singboxRouterPutSettings as ReturnType<typeof vi.fn>).mockRejectedValueOnce(new Error('boom'));
    await expect(mergeAndSaveSettings({ snifferEnabled: true })).rejects.toThrow('boom');
  });

  it('mergeAndSaveSettings: empty patch → put with current', async () => {
    (get as ReturnType<typeof vi.fn>).mockReturnValue({ enabled: true });
    await mergeAndSaveSettings({});
    expect(api.singboxRouterPutSettings).toHaveBeenCalledWith(
      expect.objectContaining({ enabled: true }),
    );
  });
});
