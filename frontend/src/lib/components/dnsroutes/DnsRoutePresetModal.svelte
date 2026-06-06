<script lang="ts">
    import ServiceCatalogModal from './ServiceCatalogModal.svelte';
    import type { RoutingTunnel, CatalogPreset } from '$lib/types';

    interface Props {
        open: boolean;
        existingNames: string[];
        tunnels: RoutingTunnel[];
        isOS5?: boolean;
        hydrarouteInstalled?: boolean;
        onclose: () => void;
        oncreate: (presets: CatalogPreset[], tunnelId: string, backend: 'ndms' | 'hydraroute') => void;
    }

    let {
        open = $bindable(false),
        existingNames,
        tunnels,
        isOS5 = false,
        hydrarouteInstalled = false,
        onclose,
        oncreate,
    }: Props = $props();

    let creating = $state(false);

    $effect(() => {
        if (!open) creating = false;
    });

    function handleCreate(
        presets: CatalogPreset[],
        tunnelId: string,
        backend: 'ndms' | 'hydraroute',
    ) {
        creating = true;
        oncreate(presets, tunnelId, backend);
    }
</script>

<ServiceCatalogModal
    bind:open
    markExisting
    {existingNames}
    footer="tunnel"
    {tunnels}
    {isOS5}
    {hydrarouteInstalled}
    submitting={creating}
    {onclose}
    oncreate={handleCreate}
/>
