<!--
  Мастер первичной настройки (простой режим): туннель → сервисы в туннель (final=direct) → включить.
-->
<script lang="ts">
  import { get } from 'svelte/store';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { Check } from 'lucide-svelte';
  import { singboxRouter as singboxRouterStore } from '$lib/stores/singboxRouter';
  import { notifications } from '$lib/stores/notifications';
  import { Button } from '$lib/components/ui';
  import EmptyHero from './EmptyHero.svelte';
  import StepPill from './StepPill.svelte';
  import WizardStep from './WizardStep.svelte';
  import SelectedTemplatesRow from './SelectedTemplatesRow.svelte';
  import CustomMatcherForm from './CustomMatcherForm.svelte';
  import TemplatesModal from './TemplatesModal.svelte';
  import { wizardCustom, resetWizardState } from './addWizardStore';
  import { templatesSelection, openTemplatesModal, clearSelection } from './templatesStore';
  import { buildTemplateList } from './templatesData';
  import { finishSetup } from './emptyStateActions';

  const outbounds = singboxRouterStore.outbounds;
  const presets = singboxRouterStore.presets;
  const ruleSets = singboxRouterStore.ruleSets;

  onMount(() => {
    void singboxRouterStore.loadAll();
  });

  let selectedTunnel = $state<string | null>(null);
  let finishing = $state(false);

  const tunnelOutbounds = $derived($outbounds.filter((o) => o.type !== 'direct'));
  const groups = $derived(buildTemplateList($presets, $ruleSets, ''));

  const hasCustom = $derived.by(() => {
    const c = $wizardCustom;
    return c.domainSuffix.trim() !== '' || c.ipCidr.trim() !== ''
      || c.sourceIpCidr.trim() !== '' || c.port.trim() !== '' || c.ruleSetTags.size > 0;
  });
  const hasServices = $derived($templatesSelection.size > 0 || hasCustom);

  const step1Done = $derived(selectedTunnel !== null);
  const step2Done = $derived(step1Done && hasServices);
  const canFinish = $derived(step1Done && step2Done && !finishing);

  async function handleFinish() {
    if (!canFinish || selectedTunnel === null) return;
    finishing = true;
    try {
      const result = await finishSetup({
        tunnelTag: selectedTunnel,
        selectedTemplates: Array.from(get(templatesSelection)),
        customFields: get(wizardCustom),
        groups,
      });
      if (result.failures.length === 0) {
        notifications.success('Готово — sing-box запущен');
      } else {
        const sN = result.successes.length;
        const fN = result.failures.length;
        notifications.error(
          sN > 0
            ? `Запущено, но часть правил с ошибкой: ${sN} из ${sN + fN}`
            : `Не удалось создать правила (${fN})`,
        );
      }
      clearSelection();
      resetWizardState();
      selectedTunnel = null;
      await singboxRouterStore.loadAll();
    } catch (e) {
      notifications.error(`Ошибка: ${e instanceof Error ? e.message : String(e)}`);
    } finally {
      finishing = false;
    }
  }
</script>

{#snippet iconCheck()}<Check size={14} />{/snippet}

<div class="wrap">
  <EmptyHero />

  <div class="stepper">
    <StepPill n={1} label="Туннель" active={!step1Done} done={step1Done} />
    <div class="connector"></div>
    <StepPill n={2} label="Что в туннель" active={step1Done && !step2Done} done={step2Done} />
    <div class="connector"></div>
    <StepPill n={3} label="Включить" active={step2Done} done={false} />
  </div>

  <WizardStep n={1} title="Выберите туннель" hint="весь трафик идёт напрямую, кроме выбранных сервисов" active={true}>
    {#if tunnelOutbounds.length === 0}
      <div class="empty-tunnels">
        Нет доступных туннелей.
        <button type="button" class="link" onclick={() => goto('/')}>Создайте туннель</button>
        и вернитесь сюда.
      </div>
    {:else}
      <div class="tunnel-chips">
        {#each tunnelOutbounds as ob (ob.tag)}
          {@const selected = selectedTunnel === ob.tag}
          <button type="button" class="t-chip" class:selected onclick={() => (selectedTunnel = ob.tag)}>
            <span class="tag">{ob.tag}</span>
            <span class="ttype">· {ob.type}</span>
          </button>
        {/each}
      </div>
    {/if}
  </WizardStep>

  <WizardStep n={2} title="Что направить в туннель" hint="остальное — напрямую" active={step1Done}>
    <button type="button" class="picker-btn" onclick={() => openTemplatesModal()}>
      <div class="picker-icon">+</div>
      <div class="picker-text">
        <div class="picker-title">Выбрать сервисы</div>
        <div class="picker-sub">{$presets.length} сервисов · {$ruleSets.length} наборов</div>
      </div>
      <div class="picker-chev">›</div>
    </button>
    <SelectedTemplatesRow />
    <CustomMatcherForm />
  </WizardStep>

  <WizardStep n={3} title="Включить" active={step2Done}>
    <p class="enable-hint">sing-box будет настроен на весь роутер. Дополнительные настройки движка — в режиме «Эксперт».</p>
    <Button variant="primary" size="md" onclick={handleFinish} disabled={!canFinish} iconBefore={iconCheck}>
      Включить sing-box
    </Button>
  </WizardStep>

  <TemplatesModal mode="collect" />
</div>

<style>
  .wrap { max-width: 720px; margin: 0 auto; padding: var(--sp-4); }
  .stepper {
    display: flex; align-items: center; gap: 8px; margin: 16px 0 24px; padding: 14px;
    background: var(--bg-secondary); border: 1px solid var(--border); border-radius: var(--radius); font-size: 12px;
  }
  .connector { flex: 1; height: 1px; background: var(--border); min-width: 16px; }
  .tunnel-chips { display: flex; flex-wrap: wrap; gap: 6px; }
  .t-chip {
    display: inline-flex; align-items: center; gap: 8px; padding: 6px 10px; border-radius: var(--radius-sm);
    background: var(--bg-tertiary); border: 1px solid var(--border); cursor: pointer; font-family: inherit; color: inherit;
  }
  .t-chip.selected { background: var(--accent-soft); border-color: var(--accent); }
  .t-chip .tag { font-family: var(--font-mono); font-size: 12px; font-weight: 500; }
  .t-chip .ttype { font-size: 11px; color: var(--text-muted); }
  .empty-tunnels { font-size: 13px; color: var(--text-muted); }
  .link { background: none; border: 0; padding: 0; color: var(--accent); cursor: pointer; font: inherit; text-decoration: underline; }
  .picker-btn {
    display: grid; grid-template-columns: 40px 1fr auto; align-items: center; gap: 12px; width: 100%;
    padding: 12px 14px; border-radius: var(--radius-sm); background: var(--bg-primary);
    border: 1px dashed var(--accent-line); color: var(--text-primary); cursor: pointer; font-family: inherit;
    text-align: left; margin-bottom: 14px;
  }
  .picker-icon {
    width: 40px; height: 40px; border-radius: 8px; background: var(--accent-soft); color: var(--accent);
    display: inline-flex; align-items: center; justify-content: center; font-size: 20px; font-weight: 600;
  }
  .picker-title { font-size: 13.5px; font-weight: 600; }
  .picker-sub { font-size: 11.5px; color: var(--text-muted); margin-top: 2px; }
  .picker-chev { color: var(--text-muted); font-size: 18px; }
  .enable-hint { margin: 0 0 12px; font-size: 13px; color: var(--text-secondary); }
</style>
