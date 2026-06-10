<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { ArrowLeft } from 'lucide-svelte';
  import { LoadingSpinner } from '$lib/components/layout';
  import { singboxRouter as singboxRouterStore } from '$lib/stores/singboxRouter';
  import { StagingBanner, RouteInspector, JsonConfigDrawer } from '$lib/components/singbox-routing';
  import { ConnectionsSubTab } from '$lib/components/routing/singboxRouter';
  import {
    PageShell,
    RulesPanel,
    FlowGraph,
    TracePanel,
    traceOpen,
    AddWizardPanel,
    addWizardOpen,
    closeAddWizard,
    closeTrace,
    EmptyState,
    ExpertPanel,
    mode as sbMode,
  } from '$lib/components/sb-router';

  let activeSingboxSub = $derived($page.url.searchParams.get('sub'));
  let inspectorOpen = $state(false);
  let jsonOpen = $state(false);
  const singboxRulesStore = singboxRouterStore.rules;
  const singboxInitialized = singboxRouterStore.initialized;
  let singboxRulesCount = $derived($singboxRulesStore.length);

  const SUB_VIEWS = new Set(['connections']);
  const LEGACY_SUBS = new Set(['deviceproxy', 'rules', 'rulesets', 'outbounds', 'dns', 'engine']);

  function resetSingboxOverlayState() {
    closeAddWizard();
    closeTrace();
  }

  onMount(() => {
    // Не восстанавливаем визард (?add=1) и sub=connections после ухода на другие вкладки routing.
    resetSingboxOverlayState();
    const sub = $page.url.searchParams.get('sub');
    if (!sub) {
      void singboxRouterStore.loadAll();
      return;
    }

    const url = new URL(window.location.href);
    let shouldReplace = false;

    if (SUB_VIEWS.has(sub)) {
      url.searchParams.delete('sub');
      shouldReplace = true;
    } else if (LEGACY_SUBS.has(sub)) {
      url.searchParams.delete('sub');
      if (sub === 'deviceproxy') {
        url.searchParams.set('mode', 'expert');
      }
      shouldReplace = true;
    }

    if (shouldReplace) {
      const search = url.searchParams.toString();
      void goto(`${url.pathname}${search ? `?${search}` : ''}`, {
        replaceState: true,
        keepFocus: true,
        noScroll: true,
      });
    }

    void singboxRouterStore.loadAll();
  });

  // Явный переход в sub=connections — закрыть визард/trace, но sub оставить.
  $effect(() => {
    const sub = activeSingboxSub;
    if (sub === 'connections') {
      resetSingboxOverlayState();
    }
  });

  let inSubView = $derived(activeSingboxSub === 'connections');

  function clearSub() {
    const url = new URL(window.location.href);
    url.searchParams.delete('sub');
    void goto(`${url.pathname}${url.search}`, { keepFocus: true, noScroll: true });
  }
</script>

<PageShell onOpenInspector={() => (inspectorOpen = true)} onOpenJson={() => (jsonOpen = true)}>
  <StagingBanner />
  {#if inSubView}
    <button type="button" class="sub-back" onclick={clearSub}>
      <ArrowLeft size={14} /> Назад
    </button>
  {/if}
  {#if activeSingboxSub === 'connections'}
    <ConnectionsSubTab />
  {:else if $sbMode === 'beginner'}
    {#if $addWizardOpen}
      <AddWizardPanel />
    {:else if $traceOpen}
      <TracePanel />
    {:else if !$singboxInitialized}
      <div class="boot-loading"><LoadingSpinner size="sm" /></div>
    {:else if singboxRulesCount === 0}
      <EmptyState />
    {:else}
      <FlowGraph />
      <RulesPanel />
    {/if}
  {:else}
    <ExpertPanel />
  {/if}
</PageShell>

<RouteInspector open={inspectorOpen} onClose={() => (inspectorOpen = false)} />
<JsonConfigDrawer open={jsonOpen} onClose={() => (jsonOpen = false)} />

<style>
  .sub-back {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    margin-bottom: 12px;
    padding: 6px 12px;
    border-radius: var(--radius-sm);
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    color: var(--text-secondary);
    font-size: 13px;
    font-family: inherit;
    cursor: pointer;
  }
  .sub-back:hover {
    color: var(--text-primary);
    border-color: var(--border-hover, var(--accent-line));
  }

  .boot-loading {
    display: flex;
    justify-content: center;
    padding: 48px 0;
  }
</style>
