<!--
  Модалка настроек inbound (device proxy). Переиспользует SettingsCard
  + чистые хелперы из $lib/utils/deviceProxyInstance. Открывается из панели Inbounds.
-->
<script lang="ts">
  import { onMount } from 'svelte';
  import { Button } from '$lib/components/ui';
  import SingboxSettingsModal from '$lib/components/routing/singboxRouter/SingboxSettingsModal.svelte';
	import SettingsCard from '$lib/components/deviceproxy/SettingsCard.svelte';
  import { api } from '$lib/api/client';
  import { deviceProxyOutbounds } from '$lib/stores/deviceproxy';
  import { configFromInstance, mergeInstanceConfig } from '$lib/utils/deviceProxyInstance';
  import type { DeviceProxyInstance, DeviceProxyConfig } from '$lib/types';

  interface Props {
    instance: DeviceProxyInstance;
    open: boolean;
    onClose: () => void;
    onSaved?: () => void;
  }
  let { instance, open, onClose, onSaved }: Props = $props();

  const outboundsSnap = $derived($deviceProxyOutbounds);
  const outbounds = $derived(outboundsSnap.data ?? []);

  let bridgeInterfaces = $state<{ id: string; label: string }[]>([]);
  let settingsCard = $state<SettingsCard | null>(null);
  let saving = $state(false);

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

<SingboxSettingsModal
  title="Настройки inbound"
  {open}
  onClose={onClose}
  size="md"
>
  {#key instance.id}
    <SettingsCard
      bind:this={settingsCard}
      bind:saving
      embedded
      hideFooter
      {config}
      {outbounds}
      {bridgeInterfaces}
      onSaveConfig={save}
      onSaved={() => {}}
      onCancel={onClose}
    />
  {/key}

  {#snippet actions()}
    <Button variant="ghost" size="md" onclick={onClose} disabled={saving} type="button">Отмена</Button>
    <Button
      variant="primary"
      size="md"
      onclick={() => void settingsCard?.save()}
      disabled={saving}
      loading={saving}
      type="button"
    >
      Сохранить
    </Button>
  {/snippet}
</SingboxSettingsModal>
