<!--
  Источник дизайна: singbox-router/project/screens/MainExpert.jsx (MainExpertScreen)
  Главная композиция Expert вида (полный набор: правила, rule-sets, outbounds, DNS, движок, прокси).

  Адаптации от шаблона:
  - onSaved → onSave (реальный prop у всех 5 модалов)
  - Все модалы требуют outboundOptions: OutboundGroup[] — берём из store.options
  - RuleEditModal требует availableRuleSets + ruleSetUsage (excludeIndex для edit)
  - DNSServerEditModal требует servers: SingboxRouterDNSServer[]
  - DNSRuleEditModal требует servers + availableRuleSets + ruleSetUsage
  - DNS данные берём из store (dnsServers/dnsRules), не грузим отдельно
  - RuleSetAddModal поддерживает edit-mode через prop ruleSet (необязательный)
  - CompositeOutboundEditModal edit-mode через prop outbound (необязательный)
-->

<script lang="ts">
  import { onMount } from 'svelte';
  import { singboxRouter as singboxRouterStore } from '$lib/stores/singboxRouter';
  import { subscriptionsStore } from '$lib/stores/subscriptions';
  import { notifications } from '$lib/stores/notifications';
  import { api } from '$lib/api/client';
  import { computeRuleSetUsage } from '$lib/components/routing/singboxRouter';
  import type { OutboundGroup } from '$lib/components/routing/singboxRouter/outboundOptions';
  import type {
    SingboxRouterRule,
    SingboxRouterRuleSet,
    SingboxRouterOutbound,
    SingboxRouterDNSServer,
    SingboxRouterDNSRule,
    SingboxRouterDNSStrategy,
    DeviceProxyInstance,
  } from '$lib/types';
  import { newDeviceProxyInstance } from '$lib/utils/deviceProxyInstance';

  import StatStrip, { type StatCellData } from './StatStrip.svelte';
  import SidePanel from './SidePanel.svelte';
  import RoutingTable from './RoutingTable.svelte';
  import RuleSetsTable from './RuleSetsTable.svelte';
  import OutboundsCompact from './OutboundsCompact.svelte';
  import DnsServersCompact from './DnsServersCompact.svelte';
  import DeviceProxyCompact from './DeviceProxyCompact.svelte';
  import InboundSettingsDrawer from './InboundSettingsDrawer.svelte';

  import RuleEditModal from '$lib/components/routing/singboxRouter/RuleEditModal.svelte';
  import RuleSetAddModal from '$lib/components/routing/singboxRouter/RuleSetAddModal.svelte';
  import CompositeOutboundEditModal from '$lib/components/routing/singboxRouter/CompositeOutboundEditModal.svelte';
  import DNSServerEditModal from '$lib/components/routing/singboxRouter/DNSServerEditModal.svelte';
  import DNSRuleEditModal from '$lib/components/routing/singboxRouter/DNSRuleEditModal.svelte';
  import { DNSRewritesList } from '$lib/components/routing/singboxRouter';
  import { ConfirmModal, Dropdown, SideDrawer, Button, type DropdownOption } from '$lib/components/ui';

  // Store subscriptions
  const storeStatus = singboxRouterStore.status;
  const storeRules = singboxRouterStore.rules;
  const storeRuleSets = singboxRouterStore.ruleSets;
  const storeOutbounds = singboxRouterStore.outbounds;
  const storeDnsServers = singboxRouterStore.dnsServers;
  const storeDnsRules = singboxRouterStore.dnsRules;
  const storeDnsRewrites = singboxRouterStore.dnsRewrites;
  const storeDnsGlobals = singboxRouterStore.dnsGlobals;
  const storeOptions = singboxRouterStore.options;

  // ── Globals (route-final + DNS final/strategy) ──────────────────────
  const STRATEGY_OPTIONS: DropdownOption<SingboxRouterDNSStrategy>[] = [
    { value: '', label: '— default —' },
    { value: 'ipv4_only', label: 'ipv4_only' },
    { value: 'ipv6_only', label: 'ipv6_only' },
    { value: 'prefer_ipv4', label: 'prefer_ipv4' },
    { value: 'prefer_ipv6', label: 'prefer_ipv6' },
  ];

  // route-final: direct + все outbounds, кроме группы «Специальные»
  const routeFinalOptions = $derived<DropdownOption[]>([
    { value: 'direct', label: 'direct (мимо VPN)' },
    ...$storeOptions
      .filter((g) => g.group !== 'Специальные')
      .flatMap((g) => g.items.map((i) => ({ value: i.value, label: i.label, group: g.group }))),
  ]);

  // DNS-final: серверы из стора
  const dnsFinalOptions = $derived<DropdownOption[]>([
    { value: '', label: '— не задан —' },
    ...$storeDnsServers.map((s) => ({ value: s.tag, label: s.tag })),
  ]);

  let draftRouteFinal = $state('direct');
  let draftDnsFinal = $state('');
  let draftDnsStrategy = $state<SingboxRouterDNSStrategy>('');
  let routeFinalBusy = $state(false);
  let dnsGlobalsBusy = $state(false);

  // draft синхронизируется со стором
  $effect(() => {
    draftRouteFinal = $storeStatus?.final || 'direct';
  });
  $effect(() => {
    draftDnsFinal = $storeDnsGlobals.final;
    draftDnsStrategy = $storeDnsGlobals.strategy;
  });

  const routeFinalDirty = $derived(draftRouteFinal !== ($storeStatus?.final || 'direct'));
  const dnsGlobalsDirty = $derived(
    draftDnsFinal !== $storeDnsGlobals.final || draftDnsStrategy !== $storeDnsGlobals.strategy,
  );

  async function saveRouteFinal() {
    if (!routeFinalDirty || routeFinalBusy) return;
    routeFinalBusy = true;
    try {
      await api.singboxRouterPutRouteFinal(draftRouteFinal);
      await singboxRouterStore.loadAll();
    } catch (e) {
      notifications.error(e instanceof Error ? e.message : String(e));
    } finally {
      routeFinalBusy = false;
    }
  }

  async function saveDnsGlobals() {
    if (!dnsGlobalsDirty || dnsGlobalsBusy) return;
    dnsGlobalsBusy = true;
    try {
      await api.singboxRouterPutDNSGlobals({ final: draftDnsFinal, strategy: draftDnsStrategy });
      await singboxRouterStore.loadAll();
    } catch (e) {
      notifications.error(e instanceof Error ? e.message : String(e));
    } finally {
      dnsGlobalsBusy = false;
    }
  }

  function resetDnsGlobalsDraft() {
    draftDnsFinal = $storeDnsGlobals.final;
    draftDnsStrategy = $storeDnsGlobals.strategy;
  }

  function closeDnsGlobalsDrawer() {
    resetDnsGlobalsDraft();
    dnsGlobalsDrawerOpen = false;
  }

  function openDnsGlobalsDrawer() {
    resetDnsGlobalsDraft();
    dnsGlobalsDrawerOpen = true;
  }

  let activeProxyCount = $state<number | null>(null);
  let totalProxyCount = $state<number | null>(null);

  async function loadActiveProxyCount() {
    try {
      const proxyInstances = await api.listDeviceProxyInstances();
      totalProxyCount = proxyInstances.length;

        const runtimeEntries = await Promise.all(
          proxyInstances.map(async (in_) => {
            const runtime = await api.getDeviceProxyInstanceRuntime(in_.id).catch(() => null);
            return { instance: in_, runtime };
        }),
      );

        activeProxyCount = runtimeEntries.filter(({ instance, runtime }) => {
          return instance.enabled && runtime?.alive === true;
        }).length;
      } catch {
        activeProxyCount = null;
      totalProxyCount = null;
    }
  }

  const activeProxyCountLabel = $derived(
    activeProxyCount === null || totalProxyCount === null ? '—' : `${activeProxyCount}/${totalProxyCount}`,
  );

  // Modal state
  let ruleEditIdx = $state<number | null>(null);
  let ruleAddOpen = $state(false);
  let rewriteAddMode = $state(false);
  let rsEditTag = $state<string | null>(null);
  let rsAddOpen = $state(false);
  let outboundEditTag = $state<string | null>(null);
  let outboundAddOpen = $state(false);
  let dnsServerEditTag = $state<string | null>(null);
  let dnsServerAddOpen = $state(false);
  let dnsRuleEditIdx = $state<number | null>(null);
  let dnsRuleAddOpen = $state(false);
  let dnsGlobalsDrawerOpen = $state(false);

  let inboundDrawerInstance = $state<DeviceProxyInstance | null>(null);
  let inboundDrawerOpen = $state(false);
  let dpReloadKey = $state(0);

  // Унифицированное подтверждение удаления (rule / rule-set / inbound)
  let pendingConfirm = $state<{ title: string; message: string; run: () => Promise<void> } | null>(null);
  let confirmBusy = $state(false);

  async function runConfirm() {
    if (!pendingConfirm) return;
    confirmBusy = true;
    try {
      await pendingConfirm.run();
      pendingConfirm = null;
    } finally {
      confirmBusy = false;
    }
  }

  function openInbound(in_: DeviceProxyInstance) {
    inboundDrawerInstance = in_;
    inboundDrawerOpen = true;
  }
  async function addInbound() {
    let existing: DeviceProxyInstance[] = [];
    try {
      existing = await api.listDeviceProxyInstances();
    } catch {
      existing = [];
    }
    inboundDrawerInstance = newDeviceProxyInstance(existing);
    inboundDrawerOpen = true;
  }
  function onInboundSaved() {
    inboundDrawerOpen = false;
    dpReloadKey += 1;
    void loadActiveProxyCount();
  }
  function deleteInbound(in_: DeviceProxyInstance) {
    if (in_.id === 'default') return;
    pendingConfirm = {
      title: 'Удалить inbound',
      message: `Удалить inbound «${in_.name || in_.id}»?`,
      run: async () => {
        try {
          await api.deleteDeviceProxyInstance(in_.id);
          notifications.success('Inbound удалён');
          dpReloadKey += 1;
          await loadActiveProxyCount();
        } catch (e) {
          notifications.error(`Не удалось удалить: ${e instanceof Error ? e.message : String(e)}`);
        }
      },
    };
  }

  onMount(() => {
    void singboxRouterStore.loadAll();
    void loadActiveProxyCount();
  });

  // Derived modal targets
  const ruleEditTarget = $derived<SingboxRouterRule | undefined>(
    ruleEditIdx !== null ? $storeRules[ruleEditIdx] : undefined
  );
  const rsEditTarget = $derived<SingboxRouterRuleSet | undefined>(
    rsEditTag !== null ? $storeRuleSets.find((rs) => rs.tag === rsEditTag) : undefined
  );
  const outboundEditTarget = $derived<SingboxRouterOutbound | undefined>(
    outboundEditTag !== null ? $storeOutbounds.find((o) => o.tag === outboundEditTag) : undefined
  );
  const dnsServerEditTarget = $derived<SingboxRouterDNSServer | undefined>(
    dnsServerEditTag !== null ? $storeDnsServers.find((s) => s.tag === dnsServerEditTag) : undefined
  );
  const dnsRuleEditTarget = $derived<SingboxRouterDNSRule | undefined>(
    dnsRuleEditIdx !== null ? $storeDnsRules[dnsRuleEditIdx] : undefined
  );

  // ruleSetUsage for RuleEditModal: exclude currently edited index
  const ruleSetUsageForRuleAdd = $derived(computeRuleSetUsage($storeRules));
  const ruleSetUsageForRuleEdit = $derived(
    ruleEditIdx === null
      ? new Map<string, number>()
      : computeRuleSetUsage($storeRules, ruleEditIdx)
  );
  // ruleSetUsage for DNSRuleEditModal: exclude currently edited index
  const ruleSetUsageForDnsAdd = $derived(computeRuleSetUsage($storeDnsRules));
  const ruleSetUsageForDnsEdit = $derived(
    dnsRuleEditIdx === null
      ? new Map<string, number>()
      : computeRuleSetUsage($storeDnsRules, dnsRuleEditIdx)
  );

  // Engine badge keys on the live interception state, not the persisted
  // toggle: enabled+active → работает (ON); enabled but jumps gone → СБОЙ;
  // disabled → OFF.
  const engineStat = $derived.by<{ value: string; tone: StatCellData['tone'] }>(() => {
    if (!$storeStatus?.enabled) return { value: 'OFF', tone: 'muted' };
    return $storeStatus.active
      ? { value: 'ON', tone: 'success' }
      : { value: 'СБОЙ', tone: 'error' };
  });

  const statCells: StatCellData[] = $derived([
    {
      label: 'Движок',
      value: engineStat.value,
      tone: engineStat.tone,
    },
    {
      label: 'Правил',
      value: String($storeRules.length),
      helpTitle: 'Правила маршрутизации',
      helpText: 'Количество route rules. Они проверяются сверху вниз: первое совпадение выбирает outbound.',
      helpItems: [
        'Порядок важен.',
        'Если ничего не подошло — используется default outbound в панели правил.',
      ],
    },
    {
      label: 'Rule-sets',
      value: String($storeRuleSets.length),
      helpTitle: 'Наборы правил',
      helpText: 'Списки доменов и IP, на которые ссылаются route rules и DNS rules.',
      helpItems: [
        'Remote — скачивается и обновляется.',
        'Local — файл на роутере.',
        'Inline — правила хранятся прямо в конфиге.',
      ],
    },
    {
      label: 'OUTBOUNDS',
      value: String($storeOutbounds.length),
      helpTitle: 'Outbounds',
      helpText: 'Доступные направления трафика: direct, reject, VPN/selector/composite и подписочные группы.',
      helpItems: [
        'Route rule выбирает outbound.',
        'DNS server тоже может ходить через outbound.',
      ],
    },
    {
      label: 'DNS',
      value: String($storeDnsRules.length),
      helpTitle: 'DNS-правила',
      helpText: 'Правила выбора DNS-сервера по доменам, rule-set, типам запросов и другим условиям.',
      helpItems: [
        'Работают отдельно от route rules.',
        'Могут направлять конкретные домены на нужный DNS-сервер.',
      ],
    },
    {
      label: 'Rewrite',
      value: String($storeDnsRewrites.length),
      helpTitle: 'DNS-перезаписи',
      helpText: 'Статические DNS-ответы: домен или шаблон получает заданный IP.',
      helpItems: [
        'Полезно для локальных override.',
        'Срабатывает до обычного DNS-резолва.',
      ],
    },
      {
        label: 'Прокси',
        value: activeProxyCountLabel,
        helpTitle: 'Device Proxy / Inbounds',
        helpText: 'Количество активных локальных inbound-прокси для устройств.',
        helpItems: [
          'active — inbound запущен и принимает подключения.',
          'выкл — запись есть, но runtime не активен.',
      ],
    },
  ]);

  // Rule handlers
  function handleDeleteRule(idx: number) {
    pendingConfirm = {
      title: 'Удалить правило',
      message: `Удалить правило #${idx}?`,
      run: async () => {
        try {
          await api.singboxRouterDeleteRule(idx);
          await singboxRouterStore.loadAll();
          notifications.success('Правило удалено');
        } catch (e) {
          notifications.error(`Ошибка: ${e instanceof Error ? e.message : String(e)}`);
        }
      },
    };
  }

  function handleDeleteDNSRule(idx: number) {
    pendingConfirm = {
      title: 'Удалить DNS-правило',
      message: `Удалить DNS-правило #${idx + 1}?`,
      run: async () => {
        try {
          await api.singboxRouterDeleteDNSRule(idx);
          await singboxRouterStore.loadAll();
          notifications.success('DNS-правило удалено');
        } catch (e) {
          notifications.error(`Ошибка: ${e instanceof Error ? e.message : String(e)}`);
        }
      },
    };
  }

  async function handleMoveRule(idx: number, dir: 'up' | 'down') {
    const to = dir === 'up' ? idx - 1 : idx + 1;
    if (to < 0 || to >= $storeRules.length) return;
    try {
      await api.singboxRouterMoveRule(idx, to);
      await singboxRouterStore.loadAll();
    } catch (e) {
      notifications.error(`Ошибка: ${e instanceof Error ? e.message : String(e)}`);
    }
  }

  // Rule save handlers (called by modals)
  async function handleRuleSave(rule: SingboxRouterRule) {
    if (ruleEditIdx !== null) {
      await api.singboxRouterUpdateRule(ruleEditIdx, rule);
    } else {
      await api.singboxRouterAddRule(rule);
    }
    ruleEditIdx = null;
    ruleAddOpen = false;
    await singboxRouterStore.loadAll();
  }

  // RuleSet handlers
  function handleDeleteRs(tag: string) {
    pendingConfirm = {
      title: 'Удалить набор',
      message: `Удалить набор «${tag}»?`,
      run: async () => {
        try {
          await api.singboxRouterDeleteRuleSet(tag);
          await singboxRouterStore.loadAll();
          notifications.success('Набор удалён');
        } catch (e) {
          notifications.error(`Ошибка: ${e instanceof Error ? e.message : String(e)}`);
        }
      },
    };
  }

  async function handleRsAddSave(rs: SingboxRouterRuleSet) {
    await api.singboxRouterAddRuleSet(rs);
    rsAddOpen = false;
    await singboxRouterStore.loadAll();
  }

  async function handleRsEditSave(rs: SingboxRouterRuleSet) {
    if (rsEditTag !== null) {
      await api.singboxRouterUpdateRuleSet(rsEditTag, rs);
    }
    rsEditTag = null;
    await singboxRouterStore.loadAll();
  }

  // Outbound handlers
  async function handleOutboundAddSave(o: SingboxRouterOutbound) {
    await api.singboxRouterAddOutbound(o);
    outboundAddOpen = false;
    await singboxRouterStore.loadAll();
  }

  async function handleOutboundEditSave(o: SingboxRouterOutbound) {
    if (outboundEditTag !== null) {
      await api.singboxRouterUpdateOutbound(outboundEditTag, o);
    }
    outboundEditTag = null;
    await singboxRouterStore.loadAll();
  }

  // DNS server handlers
  async function handleDnsServerAddSave(server: SingboxRouterDNSServer) {
    await api.singboxRouterAddDNSServer(server);
    dnsServerAddOpen = false;
    await singboxRouterStore.loadAll();
  }

  async function handleDnsServerEditSave(server: SingboxRouterDNSServer) {
    if (dnsServerEditTag !== null) {
      await api.singboxRouterUpdateDNSServer(dnsServerEditTag, server);
    }
    dnsServerEditTag = null;
    await singboxRouterStore.loadAll();
  }

  // DNS rule handlers
  async function handleDnsRuleAddSave(rule: SingboxRouterDNSRule) {
    await api.singboxRouterAddDNSRule(rule);
    dnsRuleAddOpen = false;
    await singboxRouterStore.loadAll();
  }

  async function handleDnsRuleEditSave(rule: SingboxRouterDNSRule) {
    if (dnsRuleEditIdx !== null) {
      await api.singboxRouterUpdateDNSRule(dnsRuleEditIdx, rule);
    }
    dnsRuleEditIdx = null;
    await singboxRouterStore.loadAll();
  }

  async function saveDnsGlobalsAndClose() {
    if (!dnsGlobalsDirty || dnsGlobalsBusy) return;

    dnsGlobalsBusy = true;
    try {
      await api.singboxRouterPutDNSGlobals({
        final: draftDnsFinal,
        strategy: draftDnsStrategy,
      });
      await singboxRouterStore.loadAll();
      dnsGlobalsDrawerOpen = false;
    } catch (e) {
      notifications.error(e instanceof Error ? e.message : String(e));
    } finally {
      dnsGlobalsBusy = false;
    }
  }
</script>


<div class="wrap">
  <StatStrip cells={statCells} />

  <div class="main-grid">
    <div class="col-main">
      <SidePanel
        title="Правила маршрутизации"
        count={String($storeRules.length)}
        actionLabel="+ Правило"
        actionVariant="filled"
        onAction={() => (ruleAddOpen = true)}
      >
        <div class="globals-bar">
          <span class="gb-label gb-label-full">first-match-wins · если ничего не подошло →</span>
          <span class="gb-label gb-label-mobile">если не подошло →</span>
          <div class="route-final-select">
            <Dropdown bind:value={draftRouteFinal} options={routeFinalOptions} fullWidth />
          </div>
          {#if routeFinalDirty}
            <button class="gb-save" onclick={saveRouteFinal} disabled={routeFinalBusy} type="button">
              Сохранить
            </button>
          {/if}
        </div>
        <RoutingTable
          bare
          rules={$storeRules}
          outbounds={$storeOutbounds}
          outboundOptions={$storeOptions}
          onEdit={(idx) => (ruleEditIdx = idx)}
          onDelete={handleDeleteRule}
          onMove={handleMoveRule}
        />
      </SidePanel>

      <SidePanel
        title="Rule-sets"
        count={String($storeRuleSets.length)}
        actionLabel="+ Набор"
        actionVariant="filled"
        onAction={() => (rsAddOpen = true)}
      >
        <div class="panel-cap">наборы доменов и IP, на которые ссылаются правила</div>
        <RuleSetsTable
          bare
          ruleSets={$storeRuleSets}
          onEdit={(tag) => (rsEditTag = tag)}
          onDelete={handleDeleteRs}
        />
      </SidePanel>
    </div>

    <div class="col-sidebar">
      <SidePanel
        title="Outbounds"
        count={String($storeOutbounds.length)}
        actionLabel="+ Outbound"
        actionVariant="filled"
        onAction={() => (outboundAddOpen = true)}
      >
        <OutboundsCompact
          outbounds={$storeOutbounds}
          subscriptions={$subscriptionsStore.data ?? []}
          onEdit={(tag) => (outboundEditTag = tag)}
        />
      </SidePanel>

      <SidePanel
        title="DNS-серверы"
        count={String($storeDnsServers.length)}
        actionLabel="+ Сервер"
        actionVariant="filled"
        onAction={() => (dnsServerAddOpen = true)}
      >
        <button
          type="button"
          class="globals-summary"
          onclick={openDnsGlobalsDrawer}
        >
          <div>
            <span class="gb-label">DNS по умолчанию</span>
            <div class="globals-summary-values">
              <span>Final: <strong>{$storeDnsGlobals.final || '—'}</strong></span>
              <span>Strategy: <strong>{$storeDnsGlobals.strategy || 'default'}</strong></span>
            </div>
          </div>
          <span class="globals-summary-action">Настроить</span>
        </button>
        <DnsServersCompact
          servers={$storeDnsServers}
          rules={$storeDnsRules}
          outboundOptions={$storeOptions}
          onEditServer={(tag) => (dnsServerEditTag = tag)}
          onEditRule={(idx) => (dnsRuleEditIdx = idx)}
          onDeleteRule={handleDeleteDNSRule}
          onAddRule={() => (dnsRuleAddOpen = true)}
        />
      </SidePanel>

      <SidePanel
        title="DNS Rewrite"
        count={String($storeDnsRewrites.length)}
        actionLabel="+ Добавить"
        actionVariant="filled"
        onAction={() => (rewriteAddMode = true)}
      >
        <DNSRewritesList
          rewrites={$storeDnsRewrites}
          onChange={() => singboxRouterStore.loadAll()}
          showHeader={false}
          hideColumnHeader={true}
          bind:addMode={rewriteAddMode}
        />
      </SidePanel>

        <SidePanel
          title="Inbounds"
          count={activeProxyCountLabel}
          actionLabel="+ Добавить"
          actionVariant="filled"
          onAction={addInbound}
        >
        {#key dpReloadKey}
          <DeviceProxyCompact bare onSelect={openInbound} onDelete={deleteInbound} />
        {/key}
      </SidePanel>
    </div>
  </div>
</div>

<!-- RuleEditModal: add -->
{#if ruleAddOpen}
  <RuleEditModal
    outboundOptions={$storeOptions}
    availableRuleSets={$storeRuleSets}
    ruleSetUsage={ruleSetUsageForRuleAdd}
    onClose={() => (ruleAddOpen = false)}
    onSave={handleRuleSave}
  />
{/if}

<!-- RuleEditModal: edit -->
{#if ruleEditIdx !== null && ruleEditTarget !== undefined}
  <RuleEditModal
    rule={ruleEditTarget}
    outboundOptions={$storeOptions}
    availableRuleSets={$storeRuleSets}
    ruleSetUsage={ruleSetUsageForRuleEdit}
    onClose={() => (ruleEditIdx = null)}
    onSave={handleRuleSave}
  />
{/if}

<!-- RuleSetAddModal: add -->
{#if rsAddOpen}
  <RuleSetAddModal
    outboundOptions={$storeOptions}
    onClose={() => (rsAddOpen = false)}
    onSave={handleRsAddSave}
  />
{/if}

<!-- RuleSetAddModal: edit (ruleSet prop activates edit-mode) -->
{#if rsEditTag !== null && rsEditTarget !== undefined}
  <RuleSetAddModal
    ruleSet={rsEditTarget}
    outboundOptions={$storeOptions}
    onClose={() => (rsEditTag = null)}
    onSave={handleRsEditSave}
  />
{/if}

<!-- CompositeOutboundEditModal: add -->
{#if outboundAddOpen}
  <CompositeOutboundEditModal
    outboundOptions={$storeOptions}
    onClose={() => (outboundAddOpen = false)}
    onSave={handleOutboundAddSave}
  />
{/if}

<!-- CompositeOutboundEditModal: edit -->
{#if outboundEditTag !== null && outboundEditTarget !== undefined}
  <CompositeOutboundEditModal
    outbound={outboundEditTarget}
    outboundOptions={$storeOptions}
    onClose={() => (outboundEditTag = null)}
    onSave={handleOutboundEditSave}
  />
{/if}

<!-- DNSServerEditModal: add -->
{#if dnsServerAddOpen}
  <DNSServerEditModal
    servers={$storeDnsServers}
    outboundOptions={$storeOptions}
    onClose={() => (dnsServerAddOpen = false)}
    onSave={handleDnsServerAddSave}
  />
{/if}

<!-- DNSServerEditModal: edit -->
{#if dnsServerEditTag !== null && dnsServerEditTarget !== undefined}
  <DNSServerEditModal
    server={dnsServerEditTarget}
    servers={$storeDnsServers}
    outboundOptions={$storeOptions}
    onClose={() => (dnsServerEditTag = null)}
    onSave={handleDnsServerEditSave}
  />
{/if}

<!-- DNSRuleEditModal: add -->
{#if dnsRuleAddOpen}
  <DNSRuleEditModal
    servers={$storeDnsServers}
    availableRuleSets={$storeRuleSets}
    ruleSetUsage={ruleSetUsageForDnsAdd}
    onClose={() => (dnsRuleAddOpen = false)}
    onSave={handleDnsRuleAddSave}
  />
{/if}

<!-- DNSRuleEditModal: edit -->
{#if dnsRuleEditIdx !== null && dnsRuleEditTarget !== undefined}
  <DNSRuleEditModal
    rule={dnsRuleEditTarget}
    servers={$storeDnsServers}
    availableRuleSets={$storeRuleSets}
    ruleSetUsage={ruleSetUsageForDnsEdit}
    onClose={() => (dnsRuleEditIdx = null)}
    onSave={handleDnsRuleEditSave}
  />
{/if}

{#if dnsGlobalsDrawerOpen}
  <SideDrawer
    open
    onClose={closeDnsGlobalsDrawer}
    title="DNS по умолчанию"
    width={520}
    footer={dnsGlobalsFooter}
  >
    <div class="dns-globals-drawer">
      <label class="gb-field">
        <span class="gb-flabel">Final-сервер</span>
        <Dropdown
          bind:value={draftDnsFinal}
          options={dnsFinalOptions}
          disabled={$storeDnsServers.length === 0}
          fullWidth
        />
        <span class="gb-hint">Сервер по умолчанию для запросов, не попавших ни под одно правило.</span>
      </label>

      <label class="gb-field">
        <span class="gb-flabel">Стратегия</span>
        <Dropdown bind:value={draftDnsStrategy} options={STRATEGY_OPTIONS} fullWidth />
        <span class="gb-hint">Для роутера без IPv6 обычно prefer_ipv4 или ipv4_only.</span>
      </label>
    </div>
  </SideDrawer>
{/if}

{#snippet dnsGlobalsFooter()}
  <Button variant="ghost" size="md" onclick={closeDnsGlobalsDrawer} type="button">
    Отмена
  </Button>
  <Button
    variant="primary"
    size="md"
    onclick={saveDnsGlobalsAndClose}
    disabled={dnsGlobalsBusy || !dnsGlobalsDirty}
    loading={dnsGlobalsBusy}
    type="button"
  >
    Сохранить
  </Button>
{/snippet}

{#if inboundDrawerInstance}
  <InboundSettingsDrawer
    instance={inboundDrawerInstance}
    open={inboundDrawerOpen}
    onClose={() => (inboundDrawerOpen = false)}
    onSaved={onInboundSaved}
  />
{/if}

<ConfirmModal
  open={pendingConfirm !== null}
  title={pendingConfirm?.title ?? ''}
  message={pendingConfirm?.message ?? ''}
  busy={confirmBusy}
  onConfirm={runConfirm}
  onClose={() => { if (!confirmBusy) pendingConfirm = null; }}
/>

<style>
  .wrap {
    max-width: none;
    margin: 0 auto;
  }
  /* Caption внутри SidePanel body — sub-title строкой над контентом */
  .panel-cap {
    padding: 8px 14px;
    background: var(--bg-tertiary);
    border-bottom: 1px solid var(--border);
    font-size: 11px;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  /* Globals-бар route-final (шапка панели «Правила») */
  .globals-bar {
    display: grid;
    grid-template-columns: auto minmax(0, 1fr) auto;
    align-items: center;
    gap: 8px;
    padding: 8px 14px;
    background: var(--bg-tertiary);
    border-bottom: 1px solid var(--border);
  }
  .gb-label {
    font-size: 11px;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .gb-label-mobile {
    display: none;
  }
  .route-final-select {
    min-width: 0;
    width: 100%;
  }
  /* Globals-секция DNS (шапка панели «DNS-серверы») */
  .globals-summary {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    padding: 0.75rem 0.875rem;
    background: var(--bg-tertiary);
    border: 0;
    border-bottom: 1px solid var(--border);
    color: inherit;
    text-align: left;
    cursor: pointer;
  }
  .globals-summary-values {
    display: flex;
    flex-wrap: wrap;
    gap: 0.375rem 0.75rem;
    margin-top: 0.25rem;
    font-size: 0.75rem;
    color: var(--text-muted);
  }
  .globals-summary-action {
    flex: 0 0 auto;
    font-size: 0.72rem;
    font-weight: 700;
    color: var(--accent);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .gb-field {
    display: grid;
    grid-template-columns: 84px 1fr;
    align-items: center;
    gap: 8px;
  }
  .gb-flabel {
    font-size: 11px;
    color: var(--text-muted);
  }
  .dns-globals-drawer {
    display: grid;
    gap: 0.875rem;
    min-width: 0;
  }
  .dns-globals-drawer .gb-field {
    display: grid;
    grid-template-columns: 1fr;
    gap: 0.35rem;
    align-items: start;
  }
  .gb-hint {
    font-size: 0.75rem;
    line-height: 1.35;
    color: var(--text-muted);
  }
  .main-grid {
    display: grid;
    grid-template-columns: minmax(0, 8fr) minmax(0, 4fr);
    gap: 14px;
  }
  .col-main {
    display: flex;
    flex-direction: column;
    gap: 14px;
    min-width: 0;
  }
  .col-sidebar {
    display: flex;
    flex-direction: column;
    gap: 14px;
    min-width: 0;
  }
  @media (max-width: 1023px) {
    .main-grid {
      grid-template-columns: 1fr;
    }
  }
  @media (max-width: 768px) {
    .globals-bar {
      grid-template-columns: auto minmax(0, 1fr);
      align-items: center;
      gap: 0.5rem;
      padding: 0.625rem 0.875rem;
    }
    .gb-label-full {
      display: none;
    }
    .gb-label-mobile {
      display: flex;
      align-items: center;
      min-width: 0;
      font-size: 10px;
      line-height: 1.2;
      letter-spacing: 0.04em;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .route-final-select {
      width: 100%;
      min-width: 0;
    }
    .gb-save {
      grid-column: 1 / -1;
      width: 100%;
    }
    .globals-summary {
      align-items: flex-start;
    }
    .globals-summary-values {
      display: grid;
      gap: 0.25rem;
    }
    .globals-summary-action {
      padding-top: 0.1rem;
    }
  }
</style>
