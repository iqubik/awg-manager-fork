<script lang="ts">
  import { page } from '$app/stores';
  import { singboxRouter as singboxRouterStore } from '$lib/stores/singboxRouter';
  import { DeviceProxySubTab, StagingBanner, EngineSubTab, PresetsSubTab, RouteInspector, JsonConfigDrawer } from '$lib/components/singbox-routing';
  import SettingsDrawer from './SettingsDrawer.svelte';
  import { ConnectionsSubTab } from '$lib/components/routing/singboxRouter';
  import {
    PageShell,
    RulesPanel,
    FlowGraph,
    TracePanel,
    traceOpen,
    AddWizardPanel,
    addWizardOpen,
    EmptyState,
    ExpertPanel,
    mode as sbMode,
    type EngineStatus,
  } from '$lib/components/sb-router';

  let activeSingboxSub = $derived($page.url.searchParams.get('sub'));
  let inspectorOpen = $state(false);
  let jsonOpen = $state(false);
  const singboxRouterStatus = singboxRouterStore.status;
  const singboxRulesStore = singboxRouterStore.rules;
  let singboxRulesCount = $derived($singboxRulesStore.length);

  let sbEngineStatus: EngineStatus = $derived.by(() => {
    const s = $singboxRouterStatus;
    if (!s || !s.installed) return 'unknown';
    return s.enabled ? 'ok' : 'down';
  });
</script>

<PageShell engineStatus={sbEngineStatus} onOpenInspector={() => (inspectorOpen = true)} onOpenJson={() => (jsonOpen = true)}>
  <StagingBanner />
  {#if activeSingboxSub === 'engine'}
    <EngineSubTab />
  {:else if activeSingboxSub === 'presets'}
    <PresetsSubTab />
  {:else if activeSingboxSub === 'deviceproxy'}
    <DeviceProxySubTab />
  {:else if activeSingboxSub === 'connections'}
    <ConnectionsSubTab />
  {:else if $sbMode === 'beginner'}
    {#if $addWizardOpen}
      <AddWizardPanel />
    {:else if $traceOpen}
      <TracePanel />
    {:else if !($singboxRouterStatus?.enabled ?? false) || singboxRulesCount === 0}
      <EmptyState />
    {:else}
      <FlowGraph />
      <RulesPanel />
    {/if}
  {:else}
    <ExpertPanel />
  {/if}
</PageShell>

<SettingsDrawer />

<RouteInspector open={inspectorOpen} onClose={() => (inspectorOpen = false)} />
<JsonConfigDrawer open={jsonOpen} onClose={() => (jsonOpen = false)} />
