<script lang="ts" module>
  import type { LogBucket } from '$lib/stores/logs';

  export interface LogsFilter {
    search: string;
    group: string;
    subgroup: string;
    levels: string[];
  }

  export const ALL_LEVELS = ['error', 'warn', 'info', 'full', 'debug'] as const;

  export const APP_GROUPS = ['tunnel', 'routing', 'server', 'system'] as const;
  export const SINGBOX_GROUPS = ['inbound', 'outbound', 'dns', 'router', 'runtime', 'process'] as const;

  // App-bucket entries land under group=tunnel/routing/server/system; sing-box
  // entries land under group="singbox" with subgroup=inbound/outbound/etc.
  // The toolbar shows the user a flat list of "groups" that abstracts this:
  // when bucket=singbox, "groups" actually drive the SUBGROUP filter and the
  // group filter is forced to "singbox" by LogsTerminal.
  export const GROUP_LABELS: Record<string, string> = {
    tunnel: 'Туннели',
    routing: 'Маршрутизация',
    server: 'Серверы',
    system: 'Система',
    inbound: 'Входящие',
    outbound: 'Исходящие',
    dns: 'DNS',
    router: 'Маршрутизация',
    runtime: 'Runtime',
    process: 'Процесс',
  };

  export const SUBGROUP_LABELS: Record<string, string> = {
    // tunnel
    lifecycle: 'Жизненный цикл',
    ops: 'Операции',
    state: 'Состояние',
    firewall: 'Firewall',
    pingcheck: 'Ping-check',
    connectivity: 'Connectivity',
    test: 'Тестирование',
    signature: 'Подпись',
    // routing
    'dns-route': 'DNS-маршруты',
    'static-route': 'Статические маршруты',
    'access-policy': 'Access policies',
    'client-route': 'Per-client routes',
    'singbox-router': 'Sing-box router',
    deviceproxy: 'Device proxy',
    hrneo: 'HrNeo',
    catalog: 'Каталог',
    'awg-outbounds': 'AWG outbounds',
    // server
    managed: 'Managed',
    // system
    boot: 'Загрузка',
    auth: 'Авторизация',
    settings: 'Настройки',
    update: 'Обновления',
    wan: 'WAN',
    'system-tunnels': 'Системные туннели',
    cleanup: 'Cleanup',
    dnscheck: 'DNS-проверки',
    connections: 'Соединения',
    traffic: 'Трафик',
    diagnostics: 'Диагностика',
    rci: 'RCI',
    ndms: 'NDMS',
  };

  export interface BufferBadge {
    size: number;
    capacity: number;
    oldest?: string;
  }
</script>

<script lang="ts">
  import { Badge, StatusDot, Modal, Button } from '$lib/components/ui';
  import { formatRelativeTime } from '$lib/utils/format';

  interface Props {
    filter: LogsFilter;
    onFilterChange: (filter: LogsFilter) => void;
    bucket: LogBucket;
    onBucketChange: (bucket: LogBucket) => void;
    paused: boolean;
    bufferCount: number;
    onTogglePause: () => void;
    onResume?: () => void;
    onCopy: () => void;
    onDownload: () => void;
    onClear: () => void;
    totalEntries: number;
    visibleEntries: number;
    bufferStats: BufferBadge;
    availableSubgroups: string[];
    downloading?: boolean;
    clearing?: boolean;
    searchInputRef?: (el: HTMLInputElement) => void;
  }

  let {
    filter = $bindable(),
    onFilterChange,
    bucket,
    onBucketChange,
    paused,
    bufferCount,
    onTogglePause,
    onResume,
    onCopy,
    onDownload,
    onClear,
    totalEntries,
    visibleEntries,
    bufferStats,
    availableSubgroups,
    downloading = false,
    clearing = false,
    searchInputRef,
  }: Props = $props();

  const levelLabel: Record<string, string> = {
    error: 'ERROR',
    warn: 'WARN',
    info: 'INFO',
    full: 'FULL',
    debug: 'DEBUG',
  };

  const groupOptions = $derived(bucket === 'singbox' ? SINGBOX_GROUPS : APP_GROUPS);

  function toggleLevel(lvl: string) {
    const set = new Set(filter.levels);
    if (set.has(lvl)) set.delete(lvl);
    else set.add(lvl);
    filter.levels = Array.from(set);
    onFilterChange({ ...filter });
  }

  function selectGroup(g: string) {
    if (filter.group === g) return;
    filter.group = g;
    filter.subgroup = '';
    onFilterChange({ ...filter });
  }

  function selectSubgroup(s: string) {
    if (filter.subgroup === s) return;
    filter.subgroup = s;
    onFilterChange({ ...filter });
  }

  let searchTimeout: ReturnType<typeof setTimeout> | null = null;
  function handleSearchInput(v: string) {
    filter.search = v;
    if (searchTimeout) clearTimeout(searchTimeout);
    searchTimeout = setTimeout(() => onFilterChange({ ...filter }), 300);
  }

  let confirmClearOpen = $state(false);

  function handleClear() {
    confirmClearOpen = true;
  }

  function confirmClear() {
    confirmClearOpen = false;
    onClear();
  }

  let searchEl = $state<HTMLInputElement | null>(null);
  $effect(() => {
    if (searchEl && searchInputRef) searchInputRef(searchEl);
  });

  const oldestRel = $derived(
    bufferStats.oldest ? formatRelativeTime(bufferStats.oldest) : '',
  );

  const bucketTitle = $derived(
    bucket === 'singbox' ? 'журнал sing-box' : 'журнал приложения',
  );
</script>

<div class="toolbar">
  <div class="row row-bucket">
    <span class="bucket-label">Источник</span>
    <span class="chip-row" role="group" aria-label="Источник логов">
      <button
        type="button"
        class="chip"
        class:chip-active={bucket === 'app'}
        aria-pressed={bucket === 'app'}
        onclick={() => onBucketChange('app')}
      >
        Приложение
      </button>
      <button
        type="button"
        class="chip"
        class:chip-active={bucket === 'singbox'}
        aria-pressed={bucket === 'singbox'}
        onclick={() => onBucketChange('singbox')}
      >
        Sing-box
      </button>
    </span>

    <span class="divider" aria-hidden="true"></span>

    <span class="live-cell">
      {#if paused}
        <Badge variant="warning" size="sm">PAUSED</Badge>
        {#if bufferCount > 0}
          <button type="button" class="buffer-chip" onclick={onResume}>
            +{bufferCount} ↑
          </button>
        {/if}
      {:else}
        <StatusDot variant="success" pulse size="sm" />
        <span class="live-label">LIVE</span>
      {/if}
    </span>

    <span class="buffer-meta" title="Размер настраивается в Настройках">
      <a class="buffer-link" href="/settings#logging">
        {bufferStats.size}/{bufferStats.capacity}
      </a>
      {#if oldestRel}
        <span class="buffer-oldest">· старейшая {oldestRel}</span>
      {/if}
    </span>
  </div>

  <div class="row row-chips">
    <span class="chip-row" role="group" aria-label="Фильтр по уровню">
      {#each ALL_LEVELS as lvl (lvl)}
        {@const active = filter.levels.includes(lvl)}
        <button
          type="button"
          class="chip chip-level-{lvl}"
          class:chip-active={active}
          aria-pressed={active}
          onclick={() => toggleLevel(lvl)}
        >
          {levelLabel[lvl]}
        </button>
      {/each}
    </span>

    <span class="divider" aria-hidden="true"></span>

    <span class="chip-row" role="group" aria-label="Фильтр по группе">
      <button
        type="button"
        class="chip chip-group-pill"
        class:chip-active={!filter.group}
        aria-pressed={!filter.group}
        onclick={() => selectGroup('')}
      >
        ALL
      </button>
      {#each groupOptions as g (g)}
        {@const active = filter.group === g}
        <button
          type="button"
          class="chip chip-group-pill chip-group-{g}"
          class:chip-active={active}
          aria-pressed={active}
          onclick={() => selectGroup(g)}
        >
          {GROUP_LABELS[g] ?? g}
        </button>
      {/each}
    </span>
  </div>

  {#if availableSubgroups.length > 0 && (filter.group || bucket === 'singbox')}
    <div class="row row-subgroups">
      <span class="sub-label">Подгруппа</span>
      <span class="chip-row" role="group" aria-label="Фильтр по подгруппе">
        <button
          type="button"
          class="chip chip-sub"
          class:chip-active={!filter.subgroup}
          aria-pressed={!filter.subgroup}
          onclick={() => selectSubgroup('')}
        >
          ALL
        </button>
        {#each availableSubgroups as s (s)}
          {@const active = filter.subgroup === s}
          <button
            type="button"
            class="chip chip-sub"
            class:chip-active={active}
            aria-pressed={active}
            onclick={() => selectSubgroup(s)}
          >
            {SUBGROUP_LABELS[s] ?? s}
          </button>
        {/each}
      </span>
    </div>
  {/if}

  <div class="row row-actions">
    <input
      bind:this={searchEl}
      type="search"
      placeholder="Поиск..."
      bind:value={filter.search}
      oninput={(e) => handleSearchInput((e.currentTarget as HTMLInputElement).value)}
      class="search"
    />

    <span class="counter">{visibleEntries}/{totalEntries}</span>

    <span class="actions">
      <button type="button" class="chip" onclick={onTogglePause}>
        {paused ? 'Resume' : 'Pause'}
      </button>
      <button type="button" class="chip" onclick={onCopy} disabled={visibleEntries === 0}>
        Copy
      </button>
      <button type="button" class="chip" onclick={onDownload} disabled={totalEntries === 0 || downloading}>
        {downloading ? 'Downloading…' : 'Download'}
      </button>
      <button type="button" class="chip chip-danger" onclick={handleClear} disabled={totalEntries === 0 || clearing}>
        {clearing ? 'Clearing…' : 'Clear'}
      </button>
    </span>
  </div>
</div>

<Modal
  open={confirmClearOpen}
  title="Очистить {bucketTitle}"
  size="sm"
  onclose={() => (confirmClearOpen = false)}
>
  <p class="confirm-text">
    Удалить <strong>{totalEntries}</strong> {totalEntries === 1 ? 'запись' : (totalEntries < 5 ? 'записи' : 'записей')} из {bucketTitle === 'журнал sing-box' ? 'журнала sing-box' : 'журнала приложения'}? Это действие нельзя отменить.
  </p>
  <p class="confirm-hint">
    Логирование продолжится: новые события появятся по мере работы.
  </p>
  {#snippet actions()}
    <Button variant="ghost" size="md" onclick={() => (confirmClearOpen = false)}>Отмена</Button>
    <Button variant="danger" size="md" onclick={confirmClear}>Очистить</Button>
  {/snippet}
</Modal>

<style>
  /* Layout-only. Chip colors / states are design-system primitives in app.css. */
  .confirm-text {
    margin: 0 0 0.5rem;
    font-size: 13px;
    color: var(--color-text-primary);
  }
  .confirm-hint {
    margin: 0;
    font-size: 12px;
    color: var(--color-text-muted);
  }

  .toolbar {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    padding: 0.5rem 0.75rem;
    background: var(--color-bg-secondary);
    border-bottom: 1px solid var(--color-border);
  }

  .row {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    flex-wrap: wrap;
  }

  .row-subgroups .chip-sub {
    font-size: 11px;
    padding: 0.125rem 0.5rem;
  }

  .bucket-label,
  .sub-label {
    color: var(--color-text-muted);
    font-weight: 600;
    font-size: 11px;
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }

  .live-cell {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    font-size: 12px;
  }

  .live-label {
    color: var(--color-text-muted);
    font-weight: 600;
    font-size: 11px;
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }

  .divider {
    width: 1px;
    align-self: stretch;
    background: var(--color-border);
    margin: 0 0.125rem;
  }

  .chip-row {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    flex-wrap: wrap;
  }

  .buffer-chip {
    background: var(--color-accent);
    color: white;
    border: none;
    border-radius: var(--radius-pill);
    padding: 0.125rem 0.5rem;
    font: inherit;
    font-size: 11px;
    font-weight: 600;
    cursor: pointer;
  }
  .buffer-chip:hover { filter: brightness(1.1); }

  .buffer-meta {
    display: inline-flex;
    align-items: center;
    gap: 0.375rem;
    margin-left: auto;
    font-size: 11px;
    color: var(--color-text-muted);
    font-variant-numeric: tabular-nums;
  }
  .buffer-link {
    color: var(--color-text-muted);
    text-decoration: none;
    border-bottom: 1px dotted var(--color-border);
  }
  .buffer-link:hover {
    color: var(--color-text-primary);
  }
  .buffer-oldest {
    color: var(--color-text-muted);
  }

  .counter {
    font-size: 11px;
    color: var(--color-text-muted);
    font-variant-numeric: tabular-nums;
    margin-right: 0.25rem;
  }

  .search {
    flex: 1;
    min-width: 200px;
    background: var(--color-bg-primary);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-pill);
    color: var(--color-text-primary);
    font: inherit;
    font-size: 12px;
    padding: 0.3125rem 0.75rem;
    line-height: 1.4;
  }
  .search:focus {
    outline: none;
    border-color: var(--color-accent);
  }
  .search::placeholder { color: var(--color-text-muted); }

  .actions {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    margin-left: auto;
    flex-wrap: wrap;
  }

  @media (max-width: 640px) {
    .search {
      flex-basis: 100%;
      min-width: 0;
    }
    .buffer-meta {
      margin-left: 0;
    }
  }
</style>
