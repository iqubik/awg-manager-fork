<!--
  Выезжающий drawer настроек inbound (device proxy). Переиспользует SettingsCard
  + чистые хелперы из $lib/utils/deviceProxyInstance. Открывается из панели Inbounds.
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { SideDrawer } from '$lib/components/ui';
  import SettingsCard from '$lib/components/deviceproxy/SettingsCard.svelte';
  import { api } from '$lib/api/client';
  import { deviceProxyOutbounds } from '$lib/stores/deviceproxy';
  import { configFromInstance, mergeInstanceConfig } from '$lib/utils/deviceProxyInstance';
  import type { DeviceProxyInstance, DeviceProxyConfig } from '$lib/types';

  interface Props {
    /** Редактируемый inbound; для создания нового передавайте newDeviceProxyInstance(...). */
    instance: DeviceProxyInstance;
    open: boolean;
    onClose: () => void;
    onSaved?: () => void;
  }
  let { instance, open, onClose, onSaved }: Props = $props();

  const outboundsSnap = $derived($deviceProxyOutbounds);
  const outbounds = $derived(outboundsSnap.data ?? []);

  let bridgeInterfaces = $state<{ id: string; label: string }[]>([]);
  onMount(async () => {
    try {
      const choices = await api.getDeviceProxyListenChoices();
      bridgeInterfaces = choices.bridges.map((b) => ({ id: b.id, label: b.label }));
    } catch {
      bridgeInterfaces = [];
    }
  });

  const config = $derived<DeviceProxyConfig>(configFromInstance(instance));

  async function save(cfg: DeviceProxyConfig): Promise<DeviceProxyConfig> {
    const saved = await api.saveDeviceProxyInstance(mergeInstanceConfig(instance, cfg));
    onSaved?.();
    onClose();
    return configFromInstance(saved);
  }
</script>

<SideDrawer {open} onClose={onClose} title="Настройки inbound">
  <SettingsCard
    {config}
    {outbounds}
    {bridgeInterfaces}
    onSaveConfig={save}
    onSaved={() => {}}
    onCancel={onClose}
  />
</SideDrawer>
