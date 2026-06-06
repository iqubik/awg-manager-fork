import { get } from 'svelte/store';
import type { SingboxRouterSettings } from '$lib/types';
import { api } from '$lib/api/client';
import { singboxRouter } from '$lib/stores/singboxRouter';

export interface BypassPresetMeta {
  id: string;
  label: string;
  desc: string;
}

export const BYPASS_PRESETS: readonly BypassPresetMeta[] = [
  { id: 'l2tp', label: 'L2TP / IPsec VPN', desc: 'UDP 500, 1701, 4500' },
  { id: 'ntp', label: 'NTP (синхронизация времени)', desc: 'UDP 123' },
  { id: 'netbios-smb', label: 'NetBIOS / SMB', desc: 'UDP 137/138, TCP 139/445' },
];

export async function mergeAndSaveSettings(
  patch: Partial<SingboxRouterSettings>,
): Promise<void> {
  const current = get(singboxRouter.settings);
  const merged: SingboxRouterSettings = {
    ...(current ?? ({} as SingboxRouterSettings)),
    ...patch,
  };
  await api.singboxRouterPutSettings(merged);
  await singboxRouter.loadAll();
}
