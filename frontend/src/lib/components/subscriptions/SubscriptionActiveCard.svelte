<script lang="ts">
    import { onMount, untrack } from 'svelte';
    import { goto } from '$app/navigation';
    import { browser } from '$app/environment';
    import { api } from '$lib/api/client';
    import { Badge, Button, Modal, TrafficChart, TrafficSparkline, PingButton } from '$lib/components/ui';
    import { singboxDelayFromHistory } from '$lib/utils/singboxDelay';
    import { getTrafficRates, subscribeTraffic, loadHistory } from '$lib/stores/traffic';
    import {
        singboxDelayHistory,
        singboxTraffic,
        triggerDelayCheck,
    } from '$lib/stores/singbox';
    import { subscriptionsStore } from '$lib/stores/subscriptions';
    import { notifications } from '$lib/stores/notifications';
    import type { Subscription, SubscriptionMember } from '$lib/types';
    import { formatRelativeTime } from '$lib/utils/format';
    import SubscriptionMemberPicker from './SubscriptionMemberPicker.svelte';
    import type { SingboxLayoutMode } from '$lib/constants/singboxLayout';
    import TunnelDiagnosticsModal from '$lib/components/testing/TunnelDiagnosticsModal.svelte';
    import TunnelTestIcon from '$lib/components/tunnels/TunnelTestIcon.svelte';

    interface Props {
        subscription: Subscription;
        activeMember: SubscriptionMember;
        autoDelayCheckNonce?: number;
        autoDelayCheckDelayMs?: number;
        layout?: SingboxLayoutMode;
        ondetail?: (tag: string) => void;
    }
    let {
        subscription,
        activeMember,
        autoDelayCheckNonce = 0,
        autoDelayCheckDelayMs = 0,
        layout = 'compact',
        ondetail,
    }: Props = $props();

    let pickerOpen = $state(false);
    let checking = $state(false);
    let showEndpoint = $state(false);
    let confirmDeleteOpen = $state(false);
    let deleting = $state(false);
    let diagnosticsOpen = $state(false);

    // NDMS Proxy interface name (Proxy<N>) and matching kernel TUN
    // (t2s<N>) — same naming convention sing-box tunnels use, just
    // keyed off the subscription's allocated proxyIndex. Empty when
    // proxyIndex < 0 (defensive; live subscriptions on the active list
    // always have a valid index after the EnsureProxy step).
    const proxyIface = $derived(
        subscription.proxyIndex >= 0 ? `Proxy${subscription.proxyIndex}` : '',
    );
    const kernelIface = $derived(
        subscription.proxyIndex >= 0 ? `t2s${subscription.proxyIndex}` : '',
    );
    const selectorTag = $derived(subscription.selectorTag ?? '');
    const diagnosticsUnavailableReason = $derived(
        !selectorTag || !kernelIface
            ? 'Для подписки не удалось определить интерфейс тестирования.'
            : undefined,
    );

    const history = $derived($singboxDelayHistory.get(activeMember.tag) ?? []);
    const delayPresentation = $derived(singboxDelayFromHistory(history));
    const latest = $derived(delayPresentation.latest ?? -1);
    const traffic = $derived($singboxTraffic.get(activeMember.tag));

    const trafficSparkSeries = $derived.by(() => {
        const n = Math.min(rxRates.length, txRates.length);
        if (n === 0) return { rx: [] as number[], tx: [] as number[] };
        const take = Math.min(36, n);
        const start = n - take;
        return {
            rx: rxRates.slice(start, n),
            tx: txRates.slice(start, n),
        };
    });

    let rxRates = $state<number[]>([]);
    let txRates = $state<number[]>([]);
    let trafficMemberTag = $derived(activeMember.tag);

    $effect(() => {
        const tag = trafficMemberTag;
        const update = () => {
            const t = getTrafficRates(tag);
            rxRates = t.rx;
            txRates = t.tx;
        };
        update();
        return subscribeTraffic(update);
    });

    $effect(() => {
        const tag = trafficMemberTag;
        untrack(() => loadHistory(tag));
    });
    const CHART_KEY_PREFIX = 'sub_chart_expanded_';
    let chartStorageKey = $derived(`${CHART_KEY_PREFIX}${subscription.id}`);
    let chartExpanded = $state(true);
    onMount(() => {
        chartExpanded = localStorage.getItem(chartStorageKey) !== 'false';
    });
    function toggleCharts() {
        chartExpanded = !chartExpanded;
        if (browser) {
            localStorage.setItem(chartStorageKey, String(chartExpanded));
        }
    }
    const endpointText = $derived(`${activeMember.server}:${activeMember.port}`);
    /** List row: title above IP — prefer remark, else outbound tag. */
    const listActiveServerName = $derived(
        activeMember.label?.trim() || activeMember.tag?.trim() || '',
    );
    const activeEndpointTitle = $derived(
        listActiveServerName ? `${listActiveServerName} · ${endpointText}` : endpointText,
    );
    const isURLTest = $derived(subscription.mode === 'urltest');
    /** URL feed vs inline server list (wizard: «Подписка» / «Группа серверов»). */
    const isInlineGroup = $derived(subscription.isInline || !subscription.url?.trim());
    const sourceKindLabel = $derived(isInlineGroup ? 'группа' : 'подписка');
    const lastFetchedHuman = $derived(
        subscription.lastFetched ? formatRelativeTime(subscription.lastFetched) : '—',
    );

    const cardState = $derived(delayPresentation.state);
    const latText = $derived(delayPresentation.label);
    const protocolLabel = $derived.by(() => {
        switch (activeMember.protocol) {
            case 'vless':         return 'VLESS';
            case 'trojan':        return 'Trojan';
            case 'shadowsocks':   return 'Shadowsocks';
            case 'hysteria2':     return 'Hysteria2';
            case 'naive':         return 'Naive';
            case 'mieru':         return 'Mieru';
            default:              return activeMember.protocol;
        }
    });

    async function triggerCheck(e?: MouseEvent | KeyboardEvent): Promise<void> {
        e?.stopPropagation();
        if (checking) return;
        checking = true;
        try {
            await triggerDelayCheck(activeMember.tag);
        } finally {
            checking = false;
        }
    }

    let lastAutoDelayCheckNonce = 0;
    $effect(() => {
        const nonce = autoDelayCheckNonce;
        const delay = autoDelayCheckDelayMs;
        const tag = activeMember.tag;

        if (nonce <= 0 || nonce === lastAutoDelayCheckNonce) return;
        lastAutoDelayCheckNonce = nonce;
        if (!tag || checking) return;

        const timer = setTimeout(() => {
            untrack(() => void triggerCheck());
        }, delay);
        return () => clearTimeout(timer);
    });

    async function pickMember(memberTag: string): Promise<void> {
        await api.setSubscriptionActiveMember(subscription.id, memberTag);
        await subscriptionsStore.refetch();
    }

    function openDetail(): void {
        goto(`/subscriptions/${subscription.id}`);
    }

    function openDiagnostics(e?: MouseEvent | PointerEvent | KeyboardEvent): void {
        e?.preventDefault();
        e?.stopPropagation();
        diagnosticsOpen = true;
    }

    function stopNestedAction(e: Event): void {
        e.preventDefault();
        e.stopPropagation();
    }

    async function removeSubscription(): Promise<void> {
        if (deleting) return;
        deleting = true;
        try {
            await api.deleteSubscription(subscription.id);
            await subscriptionsStore.refetch();
            confirmDeleteOpen = false;
        } catch (e) {
            notifications.error(e instanceof Error ? e.message : 'Не удалось удалить подписку');
        } finally {
            deleting = false;
        }
    }

    function formatBytes(n: number): string {
        if (n < 1024) return `${n} B`;
        if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
        if (n < 1024 * 1024 * 1024) return `${(n / (1024 * 1024)).toFixed(1)} MB`;
        return `${(n / (1024 * 1024 * 1024)).toFixed(1)} GB`;
    }


</script>

{#if layout === 'list'}
    <div
        class="sub-active-list-group"
        class:ok={cardState === 'ok'}
        class:slow={cardState === 'slow'}
        class:fail={cardState === 'fail'}
        class:unknown={cardState === 'unknown'}
    >
        <div
            class="sbx-sub-active-row"
            onclick={openDetail}
            onkeydown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    openDetail();
                }
            }}
            role="button"
            tabindex="0"
        >
            <div class="lc lc-delay" data-label="Delay">
                {#if subscription.lastError}
                    <span class="dot fail" aria-hidden="true"></span>
                    <span class="delay-inline-err mono" title={subscription.lastError}>
                        {subscription.lastError}
                    </span>
                {:else}
                    <span class="dot {cardState}" aria-hidden="true"></span>
                    <PingButton
                        label={latText}
                        state={cardState}
                        {checking}
                        onclick={(e) => {
                            e.stopPropagation();
                            void triggerCheck(e);
                        }}
                    />
                {/if}
            </div>
            <div class="lc lc-name" data-label="Подписка">
                <div class="name-title-row">
                    <div class="t1">{subscription.label}</div>
                </div>
                <div class="name-meta-row">
                    <Badge variant="accent" size="sm">{sourceKindLabel}</Badge>
                    <span>{subscription.memberTags.length} серверов</span>
                    <span>обновлено {lastFetchedHuman}</span>
                </div>
                <div class="t2 mono">{proxyIface}{#if kernelIface} · {kernelIface}{/if}</div>
            </div>
            <div class="lc lc-mode" data-label="Режим">
                {isURLTest ? 'URLTest' : 'Selector'}
            </div>
            <div class="lc lc-endpoint" data-label="Активный сервер" title={activeEndpointTitle}>
                <div class="lc-endpoint-stack">
                    {#if listActiveServerName}
                        <span class="lc-endpoint-name" title={listActiveServerName}>{listActiveServerName}</span>
                    {/if}
                    <span class="lc-endpoint-host mono">
                        {#if showEndpoint}{endpointText}{:else}••••••••{/if}
                    </span>
                </div>
                <button
                    type="button"
                    class="eye-mini"
                    onclick={(e) => {
                        e.stopPropagation();
                        showEndpoint = !showEndpoint;
                    }}
                    aria-label={showEndpoint ? 'Скрыть' : 'Показать'}
                >
                    {#if showEndpoint}
                        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
                    {:else}
                        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
                    {/if}
                </button>
            </div>
            <div class="lc lc-traffic" data-label="Трафик">
                {#if subscription.lastError}
                    <span class="delay-dash">—</span>
                {:else}
                    <div
                        role="button"
                        tabindex="0"
                        class="traffic-row-list traffic-row-list--stack mono"
                        onclick={(e) => {
                            e.stopPropagation();
                            ondetail?.(activeMember.tag);
                        }}
                        onkeydown={(e) => {
                            if (e.key === 'Enter' || e.key === ' ') {
                                e.preventDefault();
                                e.stopPropagation();
                                ondetail?.(activeMember.tag);
                            }
                        }}
                        title="Открыть детальный график"
                    >
                        <span class="traffic-rate rx">↓ {formatBytes(traffic?.download ?? 0)}</span>
                        <TrafficSparkline
                            rxData={trafficSparkSeries.rx}
                            txData={trafficSparkSeries.tx}
                            responsive
                            height={18}
                        />
                        <span class="traffic-rate tx">↑ {formatBytes(traffic?.upload ?? 0)}</span>
                    </div>
                {/if}
            </div>
            <div class="lc lc-ping-mini" data-label="Ping">
                {#if subscription.lastError}
                    <span class="delay-dash">—</span>
                {:else}
                    <div
                        class="spark-mini {cardState}"
                        title="Delay за последние проверки"
                    >
                        {#if history.length === 0}
                            {#each Array(10) as _, i (i)}
                                <div class="bar empty"></div>
                            {/each}
                        {:else}
                            {@const max = Math.max(...history.map((v) => (v <= 0 ? 100 : v)), 100)}
                            {#each history.slice(-14) as d, i (i)}
                                <div class="bar" style="height: {Math.max((d <= 0 ? max : d) / max, 0.08) * 100}%;"></div>
                            {/each}
                        {/if}
                    </div>
                {/if}
            </div>
            <div class="lc lc-actions" data-label="">
                <button
                    type="button"
                    class="action-btn"
                    title="Открыть подписку «{subscription.label}»"
                    aria-label="Открыть подписку «{subscription.label}»"
                    onclick={(e) => {
                        e.stopPropagation();
                        openDetail();
                    }}
                >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                        <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
                    </svg>                    
                </button>
                <button
                    type="button"
                    class="action-btn action-test"
                    title="Открыть диагностику подписки «{subscription.label}»"
                    aria-label="Открыть диагностику подписки «{subscription.label}»"
                    data-diagnostics-action="true"
                    onpointerdown={stopNestedAction}
                    onmousedown={stopNestedAction}
                    onclick={(e) => openDiagnostics(e)}
                    onkeydown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') openDiagnostics(e);
                        else e.stopPropagation();
                    }}
                >
                    <TunnelTestIcon />
                </button>
                <button
                    type="button"
                    class="action-btn action-danger"
                    title="Удалить подписку «{subscription.label}»"
                    aria-label="Удалить подписку «{subscription.label}»"
                    onclick={(e) => {
                        e.stopPropagation();
                        confirmDeleteOpen = true;
                    }}
                >
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <polyline points="3,6 5,6 21,6"/>
                        <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
                    </svg>                    
                </button>
            </div>
        </div>
    </div>
{:else if layout === 'dense'}
<div
    class="card view-dense"
    class:ok={cardState === 'ok'}
    class:slow={cardState === 'slow'}
    class:fail={cardState === 'fail'}
    class:unknown={cardState === 'unknown'}
>
    <div class="header header-dense">
        <div class="header-dense-body">
            <div class="title-row-dense">
                <span class="dot {cardState}" aria-hidden="true"></span>
                <button type="button" class="title title-dense" onclick={openDetail}>{subscription.label}</button>
                <Badge variant="accent" size="sm">{sourceKindLabel}</Badge>
            </div>
            <div class="meta-tags-dense">
                <span class="iface-dense">
                    {#if proxyIface}
                        <span>{proxyIface}</span>
                        {#if kernelIface}<span class="meta-dot" aria-hidden="true">·</span><span>{kernelIface}</span>{/if}
                        <span class="meta-dot" aria-hidden="true">·</span><span>{subscription.inboundTag}</span>
                    {:else}
                        <span>{subscription.inboundTag}</span>
                    {/if}
                </span>
                <span class="badge proto">{protocolLabel}</span>
                {#if activeMember.transport && activeMember.transport !== 'tcp'}
                    <span class="badge transport">{activeMember.transport.toUpperCase()}</span>
                {/if}
                {#if activeMember.security === 'reality'}
                    <span class="badge reality">Reality</span>
                {:else if activeMember.security === 'tls'}
                    <span class="badge tls">TLS</span>
                {/if}
                <span class="badge mode">{isURLTest ? 'URLTest' : 'Selector'}</span>
            </div>
        </div>
        <div class="dense-toolbar">
            <div class="dense-toolbar-bottom">
                <PingButton label={latText} state={cardState} {checking} size="sm" onclick={triggerCheck} />
            </div>
        </div>
    </div>

    <div class="details">
    {#if subscription.lastError}
        <div class="sub-error mono">{subscription.lastError}</div>
    {/if}

    <div class="details-dense-cols">
        <div class="details-dense-col">
            <div class="kv-stacked-stat">
                <span class="kv-stacked-label">{isURLTest ? 'Авто' : 'Сервер'}</span>
                <span class="kv-endpoint">
                    <span
                        class="kv-stacked-value"
                        title={activeMember.label ? `${activeMember.label} · ${endpointText}` : endpointText}
                    >
                        {#if showEndpoint}
                            {endpointText}
                        {:else if activeMember.label}
                            {activeMember.label}
                        {:else}
                            {endpointText}
                        {/if}
                    </span>
                    <button
                        type="button"
                        class="eye-btn"
                        onclick={(e) => {
                            e.stopPropagation();
                            showEndpoint = !showEndpoint;
                        }}
                        aria-label={showEndpoint ? 'Скрыть IP' : 'Показать IP'}
                    >
                        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            {#if showEndpoint}
                                <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/>
                            {:else}
                                <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/>
                            {/if}
                        </svg>
                    </button>
                </span>
            </div>
        </div>
    </div>
    <div class="dense-meta-line mono">
        <span>{subscription.memberTags.length} серверов</span>
        <span>обновлено: {lastFetchedHuman}</span>
    </div>
    </div>

    <div class="actions">
        <button type="button" class="action-btn" onclick={openDetail} title="Открыть подписку «{subscription.label}»" aria-label="Открыть подписку «{subscription.label}»">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
            </svg>
            Открыть
        </button>
        <button
            type="button"
            class="action-btn action-test"
            title="Тест"
            aria-label="Тест подписки «{subscription.label}»"
            data-diagnostics-action="true"
            onpointerdown={stopNestedAction}
            onmousedown={stopNestedAction}
            onclick={(e) => openDiagnostics(e)}
        >
            <TunnelTestIcon size={12} />
            Тест
        </button>
        <button type="button" class="action-btn action-danger" onclick={() => (confirmDeleteOpen = true)} title="Удалить подписку «{subscription.label}»" aria-label="Удалить подписку «{subscription.label}»">
            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="3,6 5,6 21,6"/>
                <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
            </svg>
            Удалить
        </button>
    </div>

    {#if !subscription.lastError}
        <div class="charts-dense">
            <button
                type="button"
                class="traffic-inline"
                onclick={() => ondetail?.(activeMember.tag)}
                title="Открыть график трафика"
            >
                <TrafficSparkline
                    rxData={trafficSparkSeries.rx}
                    txData={trafficSparkSeries.tx}
                    responsive
                    height={20}
                />
                <div class="traffic-inline-rates">
                    <span class="traffic-inline-rate rx">↓ {formatBytes(traffic?.download ?? 0)}</span>
                    <span class="traffic-inline-rate tx">↑ {formatBytes(traffic?.upload ?? 0)}</span>
                </div>
            </button>
            <div class="chart-inline delay-inline">
                <div class="chart-inline-head">
                    <span class="chart-inline-label">Delay</span>
                    <span class="chart-inline-stats">
                        {#if cardState === 'unknown'}
                            ещё не тестировали
                        {:else if cardState === 'fail'}
                            <span class="err">не отвечает</span>
                        {:else}
                            {latText}
                        {/if}
                    </span>
                </div>
                <button
                    type="button"
                    class="spark-mini spark {cardState}"
                    onclick={triggerCheck}
                    title="Клик — обновить delay"
                    aria-label="Обновить delay"
                >
                    {#if history.length === 0}
                        {#each Array(10) as _, i (i)}
                            <div class="bar empty"></div>
                        {/each}
                    {:else}
                        {@const max = Math.max(...history.map((v) => (v <= 0 ? 100 : v)), 100)}
                        {#each history.slice(-14) as d, i (i)}
                            <div class="bar" style="height: {Math.max((d <= 0 ? max : d) / max, 0.08) * 100}%;"></div>
                        {/each}
                    {/if}
                </button>
            </div>
        </div>
    {/if}
</div>
{:else}
<div
    class="card view-compact"
    class:ok={cardState === 'ok'}
    class:slow={cardState === 'slow'}
    class:fail={cardState === 'fail'}
    class:unknown={cardState === 'unknown'}
>
    <div class="led-wrap">
        <PingButton label={latText} state={cardState} {checking} onclick={triggerCheck} />
    </div>

    <div class="title-row">
        <span class="dot {cardState}" aria-hidden="true"></span>
        <h3 class="title">{subscription.label}</h3>
        <Badge variant="accent" size="sm">{sourceKindLabel}</Badge>
    </div>
    <div class="iface">
        {#if proxyIface}
            <span>{proxyIface}</span>
            {#if kernelIface}<span class="meta-dot" aria-hidden="true">·</span><span>{kernelIface}</span>{/if}
            <span class="meta-dot" aria-hidden="true">·</span><span>{subscription.inboundTag}</span>
        {:else}
            <span>{subscription.inboundTag}</span>
        {/if}
        <span class="meta-dot" aria-hidden="true">·</span><span>:{subscription.listenPort}</span>
    </div>

    <div class="badges">
        <span class="badge proto">{protocolLabel}</span>
        {#if activeMember.transport && activeMember.transport !== 'tcp'}
            <span class="badge transport">{activeMember.transport.toUpperCase()}</span>
        {/if}
        {#if activeMember.security === 'reality'}
            <span class="badge reality">Reality</span>
        {:else if activeMember.security === 'tls'}
            <span class="badge tls">TLS</span>
        {/if}
    </div>

    <div class="sub-meta">
        <div>
            <span>{subscription.memberTags.length} серверов</span>
            {#if subscription.activeMember}
                <span>· активен <span class="mono">{subscription.activeMember}</span></span>
            {/if}
        </div>
        <div>
            <span>обновлено {lastFetchedHuman}</span>
            {#if subscription.refreshHours > 0}
                <span>· auto {subscription.refreshHours}ч</span>
            {/if}
        </div>
    </div>

    {#if subscription.lastError}
        <div class="sub-error mono">{subscription.lastError}</div>
    {/if}

    <div class="divider divider-dashed"></div>

    <div class="server-row">
        <span class="label">{isURLTest ? 'Авто' : 'Сервер'}</span>
        <div class="picker-anchor">
            <div class="server-control">
                <button
                    class="server-btn"
                    class:server-btn-readonly={isURLTest}
                    onclick={(e) => {
                        e.stopPropagation();
                        if (isURLTest) {
                            notifications.info(
                                'Включён автовыбор (URLTest). Чтобы выбирать сервер вручную, откройте подписку → вкладка «Настройки» → режим «Вручную».',
                                { duration: 9000 },
                            );
                            return;
                        }
                        pickerOpen = !pickerOpen;
                    }}
                    aria-haspopup={isURLTest ? undefined : 'listbox'}
                    aria-expanded={isURLTest ? undefined : pickerOpen}
                    title={isURLTest ? 'Sing-box выбирает самый быстрый сервер автоматически' : ''}
                >
                    <span
                        class="server-text"
                        class:mono={showEndpoint || !activeMember.label}
                        title={activeMember.label ? `${activeMember.label} · ${endpointText}` : endpointText}
                    >
                        {#if showEndpoint}
                            {endpointText}
                        {:else if activeMember.label}
                            {activeMember.label}
                        {:else}
                            {endpointText}
                        {/if}
                    </span>
                    {#if !isURLTest}
                        <span class="caret" aria-hidden="true">▾</span>
                    {/if}
                </button>
                <button
                    type="button"
                    class="eye-btn"
                    onclick={(e) => {
                        e.stopPropagation();
                        showEndpoint = !showEndpoint;
                    }}
                    title={showEndpoint ? 'Скрыть IP' : 'Показать IP'}
                    aria-label={showEndpoint ? 'Скрыть IP сервера' : 'Показать IP сервера'}
                >
                    {#if showEndpoint}
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/></svg>
                    {:else}
                        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
                    {/if}
                </button>
            </div>
            {#if pickerOpen && !isURLTest}
                <SubscriptionMemberPicker
                    members={subscription.members ?? []}
                    activeMemberTag={subscription.activeMember}
                    onPick={pickMember}
                    onClose={() => (pickerOpen = false)}
                />
            {/if}
        </div>
    </div>

    <div class="actions">
        <button
            type="button"
            class="action-btn"
            onclick={openDetail}
            title="Открыть подписку «{subscription.label}»"
            aria-label="Открыть подписку «{subscription.label}»"
        >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
                <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
            </svg>
            Открыть
        </button>
        <button
            type="button"
            class="action-btn action-test"
            title="Открыть диагностику подписки «{subscription.label}»"
            aria-label="Открыть диагностику подписки «{subscription.label}»"
            data-diagnostics-action="true"
            onpointerdown={stopNestedAction}
            onmousedown={stopNestedAction}
            onclick={(e) => openDiagnostics(e)}
            onkeydown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') openDiagnostics(e);
                else e.stopPropagation();
            }}
        >
            <TunnelTestIcon />
            Тест
        </button>
        <button
            type="button"
            class="action-btn action-danger"
            onclick={() => (confirmDeleteOpen = true)}
            title="Удалить подписку «{subscription.label}»"
            aria-label="Удалить подписку «{subscription.label}»"
        >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="3,6 5,6 21,6"/>
                <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
            </svg>
            Удалить
        </button>
    </div>

    <div class="chart-section">
        <button type="button" class="chart-header" onclick={toggleCharts}>
            <span class="chart-label">Графики</span>
            <span class="chart-chevron" class:expanded={chartExpanded}>▾</span>
        </button>
        <div class="chart-body" class:expanded={chartExpanded}>
            <div class="chart-head">
                <span>Delay (5 мин)</span>
                <span class="stats">
                    {#if cardState === 'unknown'}ещё не тестировали
                    {:else if cardState === 'fail'}<span class="err">не отвечает</span>
                    {:else}{latText}{/if}
                </span>
            </div>
            <div
                class="spark {cardState}"
                title="Delay за последние проверки"
            >
                {#if history.length === 0}
                    {#each Array(6) as _, i (i)}<div class="bar empty"></div>{/each}
                {:else}
                    {@const max = Math.max(...history.map((v) => (v <= 0 ? 100 : v)), 100)}
                    {#each history as d, i (i)}
                        <div class="bar" style="height: {Math.max((d <= 0 ? max : d) / max, 0.1) * 100}%;"></div>
                    {/each}
                {/if}
            </div>
            <div class="chart-head traffic-head">
                <span>Трафик</span>
                <span class="stats">
                    ↓ {formatBytes(traffic?.download ?? 0)} · ↑ {formatBytes(traffic?.upload ?? 0)}
                </span>
            </div>
            <TrafficChart
                {rxRates}
                {txRates}
                rxTotal={traffic?.download ?? 0}
                txTotal={traffic?.upload ?? 0}
                height={56}
                onclick={() => ondetail?.(activeMember.tag)}
            />
        </div>
    </div>
</div>
{/if}

<TunnelDiagnosticsModal
    open={diagnosticsOpen}
    kind="subscription"
    targetId={selectorTag}
    displayName={subscription.label || selectorTag || subscription.id}
    subjectLabel="подписку"
    iface={kernelIface}
    loading={false}
    unavailableReason={diagnosticsUnavailableReason}
    onclose={() => (diagnosticsOpen = false)}
/>

<Modal
    open={confirmDeleteOpen}
    title="Удалить подписку?"
    size="md"
    onclose={() => {
        if (deleting) return;
        confirmDeleteOpen = false;
    }}
>
    <p>
        Подписка <strong>{subscription.label || subscription.url}</strong> будет
        удалена вместе с её sing-box outbound'ами и NDMS Proxy
        <code class="mono">Proxy{subscription.proxyIndex}</code>.
    </p>
    {#snippet actions()}
        <Button variant="ghost" disabled={deleting} onclick={() => (confirmDeleteOpen = false)}>
            Отмена
        </Button>
        <Button variant="danger" disabled={deleting} loading={deleting} onclick={removeSubscription}>
            {deleting ? 'Удаляем...' : 'Удалить'}
        </Button>
    {/snippet}
</Modal>

<style>
    .card {
        position: relative;
        display: flex;
        flex-direction: column;
        gap: 10px;
        padding: 12px 14px;
        border: 1px solid var(--color-border);
        border-radius: var(--radius);
        background: var(--color-bg-secondary);
        color: var(--color-text-primary);
        transition: border-color var(--t-fast) ease;
    }
    .card.ok { border-color: var(--color-success-border); }
    .card.slow { border-color: var(--color-warning-border); }
    .card.fail { border-color: var(--color-error-border); }
    .card.unknown { border-color: var(--color-border); }

    .card.view-dense {
        gap: 8px;
        padding: 10px 12px;
    }

    .card.view-dense .header.header-dense {
        display: grid;
        grid-template-columns: minmax(0, 1fr) auto;
        align-items: flex-start;
        gap: 6px;
    }

    .header-dense-body {
        display: flex;
        flex-direction: column;
        gap: 1px;
        min-width: 0;
    }

    .title-row-dense {
        display: flex;
        align-items: center;
        gap: 5px;
        min-width: 0;
        overflow: hidden;
    }

    .title-dense {
        margin: 0;
        padding: 0;
        border: none;
        background: none;
        font: inherit;
        font-size: 13px;
        font-weight: 600;
        color: inherit;
        cursor: pointer;
        text-align: left;
        flex: 0 1 auto;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .title-dense:hover { color: var(--color-accent); }

    .title-row-dense :global(.badge) {
        flex-shrink: 0;
        font-size: 9px;
        padding: 1px 5px;
    }

    .meta-tags-dense {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        margin-top: 3px;
        gap: 3px;
        min-width: 0;
    }

    .iface-dense {
        display: inline-flex;
        flex-wrap: wrap;
        align-items: center;
        gap: 0;
        font-size: 9px;
        font-family: var(--font-mono, monospace);
        color: var(--color-text-muted);
        min-width: 0;
    }

    .meta-dot {
        margin: 0 0.35em;
        opacity: 0.75;
    }

    .card.view-dense .meta-tags-dense .badge {
        font-size: 9px;
        padding: 1px 5px;
        line-height: 1.3;
    }

    .card.view-dense .badge.mode {
        background: rgba(100, 100, 100, 0.3);
        color: var(--color-text-muted);
    }

    .dense-toolbar {
        display: flex;
        flex-direction: column;
        align-items: flex-end;
        flex-shrink: 0;
    }

    .dense-toolbar-bottom { display: flex; align-items: center; }

    .card.view-dense .title-row-dense .dot {
        width: var(--sbx-status-dot-dense);
        height: var(--sbx-status-dot-dense);
        border-radius: 50%;
        flex-shrink: 0;
    }

    .details-dense-cols {
        display: grid;
        grid-template-columns: minmax(0, 1fr);
        gap: 10px 10px;
        align-items: start;
    }

    .details-dense-col {
        display: flex;
        flex-direction: column;
        gap: 6px;
        min-width: 0;
    }

    .dense-meta-line {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        justify-content: flex-start;
        gap: 0.35rem 0.75rem;
        min-width: 0;
        font-size: 9px;
        line-height: 1.25;
        color: var(--color-text-muted);
    }

    .dense-meta-line span {
        white-space: nowrap;
    }

    .kv-stacked-stat {
        display: flex;
        flex-direction: column;
        gap: 1px;
        min-width: 0;
    }

    .card.view-dense .details-dense-cols .kv-stacked-stat {
        flex-direction: row;
        align-items: center;
        gap: 0.35rem;
    }

    .card.view-dense .details-dense-cols .kv-stacked-label {
        flex: 0 0 auto;
    }

    .card.view-dense .details-dense-cols .kv-endpoint {
        flex: 1 1 auto;
        min-width: 0;
    }

    .card.view-dense .details-dense-cols .kv-stacked-value {
        min-width: 0;
    }

    .card.view-dense .kv-endpoint {
        display: flex;
        align-items: center;
        gap: 2px;
        min-width: 0;
    }

    .kv-stacked-label {
        font-size: 9px;
        text-transform: uppercase;
        letter-spacing: 0.04em;
        color: var(--color-text-muted);
    }

    .kv-stacked-value {
        font-size: 10px;
        font-family: var(--font-mono, monospace);
        color: var(--color-text-secondary);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .card.view-dense .details {
        display: flex;
        flex-direction: column;
        gap: 6px;
        padding: 4px 0;
        border-top: 1px solid var(--color-border);
        border-bottom: 1px solid var(--color-border);
    }

    .charts-dense {
        display: flex;
        flex-direction: row;
        align-items: stretch;
        gap: 4px;
        width: 100%;
        min-width: 0;
    }

    .charts-dense > .delay-inline,
    .charts-dense > .traffic-inline {
        flex: 1 1 0;
        min-width: 0;
        width: auto;
    }

    .chart-inline {
        display: flex;
        flex-direction: column;
        gap: 3px;
        min-width: 0;
        padding: 5px 6px;
        border: 1px solid var(--color-border);
        border-radius: var(--radius-sm);
        background: var(--color-bg-secondary);
        font: inherit;
        color: inherit;
        text-align: left;
    }

    .chart-inline.delay-inline {
        gap: 0;
        padding: 0;
        overflow: hidden;
    }

    .chart-inline.delay-inline .chart-inline-head {
        padding: 5px 6px 3px;
    }

    .charts-dense .traffic-inline {
        display: flex;
        align-items: center;
        gap: 0.3rem;
        padding: 5px 4px 5px 5px;
        border: 1px solid var(--color-border);
        border-radius: var(--radius-sm);
        background: var(--color-bg-secondary);
        cursor: pointer;
        font: inherit;
        color: inherit;
        text-align: left;
    }

    .charts-dense .traffic-inline :global(svg.responsive) {
        flex: 1 1 auto;
        width: 100%;
        min-width: 0;
    }

    .traffic-inline:hover {
        background: var(--color-bg-hover);
    }

    .traffic-inline:focus-visible,
    .card.view-dense .delay-inline .spark-mini:focus-visible {
        outline: 2px solid var(--color-accent);
        outline-offset: 2px;
    }

    .charts-dense .traffic-inline-rates {
        display: flex;
        flex-direction: column;
        gap: 0.06rem;
        min-width: 0;
        flex: 0 0 auto;
        font-size: 9px;
        line-height: 1.1;
        font-family: var(--font-mono, monospace);
    }

    .charts-dense .traffic-inline-rate {
        max-width: 100%;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .charts-dense .traffic-inline-rate.rx { color: var(--color-accent); }
    .charts-dense .traffic-inline-rate.tx { color: var(--color-success); }

    .chart-inline-head {
        display: flex;
        justify-content: space-between;
        align-items: baseline;
        gap: 6px;
        font-size: 9px;
        line-height: 1.2;
    }

    .chart-inline-label {
        color: var(--color-text-muted);
        text-transform: uppercase;
        letter-spacing: 0.04em;
        font-weight: 500;
    }

    .chart-inline-stats {
        color: var(--color-text-muted);
        font-family: var(--font-mono, monospace);
        font-variant-numeric: tabular-nums;
        white-space: nowrap;
    }

    .chart-inline-stats .err {
        color: var(--color-error);
    }

    .charts-dense .chart-inline-head {
        gap: 4px;
    }

    .charts-dense .chart-inline-stats {
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .card.view-dense .delay-inline .spark-mini {
        display: flex;
        align-items: flex-end;
        align-self: stretch;
        gap: 1px;
        width: 100%;
        max-width: none;
        height: 20px;
        padding: 0;
        border: none;
        border-radius: 0 0 var(--radius-sm) var(--radius-sm);
        background: none;
        cursor: pointer;
        overflow: hidden;
    }

    .card.view-dense .delay-inline .spark-mini .bar {
        flex: 1;
        min-width: 0;
        min-height: 2px;
        border-radius: 1px;
        background: linear-gradient(to top, rgba(59, 130, 246, 0.6), rgba(96, 165, 250, 0.9));
    }

    .card.view-dense .delay-inline .spark-mini.fail .bar {
        background: var(--latency-bar-fail);
        height: 100% !important;
    }

    .card.view-dense .delay-inline .spark-mini.unknown .bar,
    .card.view-dense .delay-inline .spark-mini .bar.empty {
        background: var(--color-border);
        height: 30% !important;
    }

    .card.view-dense .dot.ok { background: var(--latency-color-ok); }
    .card.view-dense .dot.slow { background: var(--latency-color-slow); }
    .card.view-dense .dot.fail { background: var(--latency-color-fail); }
    .card.view-dense .dot.unknown { background: var(--color-text-muted); }
    .led-wrap {
        position: absolute;
        top: 12px;
        right: 12px;
        display: flex;
        align-items: center;
        gap: 0.4rem;
    }
    .dot {
        width: var(--sbx-status-dot);
        height: var(--sbx-status-dot);
        border-radius: 999px;
        background: var(--color-bg-tertiary);
    }
    .dot.ok      { background: var(--latency-color-ok); box-shadow: 0 0 6px var(--latency-dot-ok-shadow); }
    .dot.slow    { background: var(--latency-color-slow); box-shadow: 0 0 6px var(--latency-dot-slow-shadow); }
    .dot.fail    { background: var(--latency-color-fail); box-shadow: 0 0 6px var(--latency-dot-fail-shadow); }
    .title-row {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        min-width: 0;
        margin-right: 90px; /* room for led-wrap */
    }
    .title-row > .dot {
        flex-shrink: 0;
    }
    .title {
        font-size: var(--sbx-card-title);
        line-height: var(--sbx-card-title-line-height);
        font-weight: 600;
        margin: 0;
        flex: 0 1 auto;
        min-width: 0;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .title-row :global(.badge) {
        flex-shrink: 0;
    }
    .iface {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        font-size: var(--sbx-card-meta);
        color: var(--color-text-muted);
        font-family: var(--font-mono, ui-monospace, monospace);
    }
    .badges { display: flex; gap: 0.4rem; flex-wrap: wrap; }
    .badge {
        font-size: var(--sbx-card-badge);
        padding: 2px 8px;
        border-radius: 10px;
        font-weight: 500;
    }
    .badge.proto    { background: rgba(88,166,255,0.15); color: var(--color-accent); }
    .badge.transport{ background: var(--color-bg-tertiary); color: var(--color-text-muted); }
    .badge.tls      { background: rgba(63,185,80,0.15); color: #3fb950; }
    .badge.reality  { background: rgba(210,153,34,0.15); color: #d29922; }
    .server-row {
        display: grid;
        grid-template-columns: max-content minmax(0, 1fr);
        gap: 0.45rem;
        align-items: center;
        margin: 0;
    }
    .label {
        color: var(--color-text-muted);
        font-size: var(--sbx-card-label);
        text-transform: uppercase;
        letter-spacing: 0.04em;
    }
    .picker-anchor { position: relative; min-width: 0; }
    .server-control {
        display: flex;
        align-items: center;
        gap: 0.25rem;
        min-width: 0;
    }
    .server-btn {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 0.5rem;
        width: 100%;
        min-width: 0;
        padding: 0.4rem 0.55rem;
        background: var(--color-bg-primary);
        border: 1px solid var(--color-border);
        border-radius: 4px;
        font: inherit;
        font-size: var(--sbx-card-value);
        color: var(--color-text-primary);
        cursor: pointer;
        min-width: 0;
    }
    .server-btn:hover { border-color: var(--color-accent); }
    .server-btn-readonly { cursor: default; }
    .server-btn-readonly:hover { border-color: var(--color-border); }
    .server-text {
        font-size: var(--sbx-card-value);
        overflow: hidden;
        display: block;
        text-overflow: ellipsis;
        white-space: nowrap;
        word-break: normal;
        overflow-wrap: normal;
        min-width: 0;
    }
    .server-text.mono {
        font-family: var(--font-mono, ui-monospace, monospace);
        font-size: var(--sbx-card-value);
    }
    .caret { color: var(--color-text-muted); font-size: var(--sbx-card-note); }
    .eye-btn {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        flex: 0 0 auto;
        padding: 0.35rem;
        border: none;
        background: none;
        color: var(--color-text-muted);
        cursor: pointer;
        transition: color var(--t-fast) ease;
    }
    .eye-btn:hover { color: var(--color-text-secondary); }
    .divider {
        height: 0;
        border: none;
        margin: 4px 0;
        background: none;
    }
    .divider-dashed {
        border-top: 1px dashed var(--color-border);
    }
    .chart-block { display: flex; flex-direction: column; gap: 0.3rem; }
    .traffic-block { margin-bottom: 0.25rem; }
    .chart-head {
        display: flex;
        justify-content: space-between;
        font-size: var(--sbx-card-label);
        color: var(--color-text-muted);
        text-transform: uppercase;
        letter-spacing: 0.04em;
    }
    .chart-head .stats {
        font-size: var(--sbx-card-value);
    }
    .stats { font-family: var(--font-mono, ui-monospace, monospace); }
    .err { color: #f85149; text-transform: none; }
    .spark {
        display: flex;
        gap: 1px;
        align-items: flex-end;
        height: 28px;
    }
    .bar { flex: 1; background: var(--color-bg-tertiary); border-radius: 1px; }
    .spark.ok .bar    { background: var(--latency-bar-ok); }
    .spark.slow .bar  { background: var(--latency-bar-slow); }
    .spark.fail .bar  { background: var(--latency-bar-fail); }
    .bar.empty        { opacity: 0.3; }
    .actions {
        display: flex;
        gap: 6px;
        justify-content: flex-end;
        align-items: center;
        margin-top: 12px;
        padding: 10px 0;
        border-top: 1px solid var(--color-border);
        border-bottom: 1px solid var(--color-border);
    }
    .card.view-compact .actions {
        justify-content: center;
    }
    .card.view-dense .actions {
        gap: 2px;
        justify-content: center;
        margin-top: 0;
        padding: 0;
        border: none;
    }
    .action-btn {
        display: inline-flex;
        align-items: center;
        gap: 4px;
        padding: 5px 9px;
        font-size: var(--sbx-card-action);
        font-weight: 500;
        border: none;
        background: transparent;
        color: var(--color-text-secondary);
        cursor: pointer;
        border-radius: var(--radius-sm);
        text-decoration: none;
        font-family: inherit;
        transition: background var(--t-fast) ease, color var(--t-fast) ease;
    }
    .card.view-dense .action-btn {
        padding: 3px 6px;
        font-size: var(--sbx-card-action-dense);
        gap: 3px;
    }
    .action-btn:hover:not(:disabled) {
        background: var(--color-bg-hover);
        color: var(--color-text-primary);
    }
    .action-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }
    .action-btn.action-danger:hover:not(:disabled) {
        color: var(--color-error);
        background: var(--color-error-tint);
    }
    .action-btn.action-test:hover:not(:disabled) {
        color: var(--color-success);
        background: var(--color-success-tint);
    }
    .chart-section {
        margin: 0 -14px -12px;
        border-radius: 0 0 var(--radius) var(--radius);
        background: var(--color-bg-secondary);
        overflow: hidden;
    }
    .chart-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        width: 100%;
        padding: 7px 12px;
        border: none;
        border-bottom: 1px solid color-mix(in srgb, var(--color-border) 70%, transparent);
        background: color-mix(in srgb, var(--color-bg-tertiary) 78%, transparent);
        color: var(--color-text-secondary);
        cursor: pointer;
        user-select: none;
        font: inherit;
        transition: background var(--t-fast) ease, border-color var(--t-fast) ease;
    }
    .chart-header:hover {
        background: color-mix(in srgb, var(--color-bg-hover) 82%, transparent);
        border-bottom-color: var(--color-border-hover);
    }
    .chart-label {
        font-size: var(--sbx-card-note);
        font-weight: 600;
        color: var(--color-text-secondary);
        text-transform: uppercase;
        letter-spacing: 0.04em;
    }
    .chart-chevron {
        font-size: 14px;
        color: var(--color-text-secondary);
        opacity: 0.85;
        transition: transform var(--t-fast) ease;
        transform: rotate(-90deg);
    }
    .chart-chevron.expanded {
        transform: rotate(0deg);
    }
    .chart-body {
        max-height: 0;
        overflow: hidden;
        transition: max-height var(--t-med) ease;
        padding: 0 12px;
    }
    .chart-body.expanded {
        max-height: 300px;
        padding: 8px 12px 10px;
    }
    .traffic-head { margin-top: 8px; }

    .sub-meta {
        font-size: var(--sbx-card-meta);
        color: var(--color-text-muted);
        line-height: 1.35;
        display: flex;
        flex-direction: column;
        gap: 0.25rem;
    }
    .sub-error {
        font-size: var(--sbx-card-meta);
        color: #f85149;
    }
    .mono {
        font-family: var(--font-mono, ui-monospace, monospace);
    }

    /* Dense active subscription cards: let the delay bars use the whole cardlet.
       The header keeps its own padding, but the bars start exactly from the
       cardlet edge instead of from the old inner 6px gutter. */
    .card.view-dense .chart-inline.delay-inline {
        gap: 0;
        padding: 0;
        overflow: hidden;
    }

    .card.view-dense .chart-inline.delay-inline .chart-inline-head {
        padding: 5px 6px 3px;
    }

    .card.view-dense .chart-inline.delay-inline .spark-mini {
        align-self: stretch;
        width: 100%;
        max-width: none;
        height: 20px;
        padding: 0;
        border-radius: 0 0 var(--radius-sm) var(--radius-sm);
        overflow: hidden;
    }

    .sub-active-list-group {
        border-bottom: 1px solid var(--color-border);
    }
    .sub-active-list-group:last-child {
        border-bottom: none;
    }
    .sbx-sub-active-row {
        display: grid;
        grid-template-columns: var(
            --sbx-sub-list-columns,
            minmax(80px, 80px)
            minmax(190px, 1.35fr)
            minmax(58px, 0.55fr)
            minmax(190px, 1.2fr)
            minmax(160px, 0.95fr)
            minmax(96px, 96px)
            minmax(76px, 76px)
        );
        gap: 0.75rem 1rem;
        align-items: center;
        padding: 0.75rem 1rem;
        cursor: pointer;
        min-width: max(100%, max(var(--awg-list-min-width, 0px), max-content));
    }
    .sbx-sub-active-row:focus-visible {
        outline: 2px solid var(--color-accent);
        outline-offset: -2px;
    }
    .lc {
        display: flex;
        align-items: center;
        min-width: 0;
        font-size: var(--sbx-card-value);
        color: var(--color-text-secondary);
    }
    .lc-delay {
        gap: 0.35rem;
        min-width: 0;
    }
    .delay-inline-err {
        font-size: var(--sbx-card-badge);
        line-height: 1.25;
        color: #f85149;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        min-width: 0;
        flex: 1;
    }
    .delay-dash {
        font-size: var(--sbx-card-value);
        color: var(--color-text-muted);
    }
    .lc-ping-mini {
        justify-content: center;
        align-items: center;
        width: 100%;
        min-width: 0;
    }
    .spark-mini {
        width: 100%;
        height: 22px;
        max-width: 96px;
        display: flex;
        align-items: flex-end;
        gap: 1px;
        padding: 1px 0;
    }
    .spark-mini .bar {
        flex: 1;
        min-width: 0;
        min-height: 2px;
        border-radius: 1px;
        background: var(--color-bg-tertiary);
    }
    .spark-mini.ok .bar {
        background: var(--latency-bar-ok);
    }
    .spark-mini.slow .bar {
        background: var(--latency-bar-slow);
    }
    .spark-mini.fail .bar {
        background: var(--latency-bar-fail);
    }
    .spark-mini.unknown .bar,
    .spark-mini .bar.empty {
        opacity: 0.35;
        height: 30% !important;
    }
    .traffic-row-list {
        display: flex;
        min-width: 0;
        width: 100%;
    }
    .traffic-row-list--stack {
        flex-direction: column;
        align-items: stretch;
        gap: 0.05rem;
        border-radius: 4px;
        cursor: pointer;
        font-size: var(--sbx-card-note);
        line-height: 1.1;
        transition: background var(--t-fast) ease;
    }
    .traffic-row-list--stack :global(svg.responsive) {
        width: 100%;
        min-width: 0;
        max-width: 100%;
        flex: 1 1 auto;
    }
    .traffic-row-list--stack:hover {
        background: rgba(96, 165, 250, 0.06);
    }
    .traffic-row-list--stack:focus-visible {
        outline: 1px solid var(--color-accent, #58a6ff);
        outline-offset: 1px;
    }
    .lc-name {
        flex-direction: column;
        align-items: flex-start !important;
        gap: 0.15rem;
    }
    .name-title-row {
        display: flex;
        align-items: center;
        gap: 5px;
        min-width: 0;
        max-width: 100%;
    }
    .name-title-row .t1 {
        flex: 0 1 auto;
        min-width: 0;
        overflow: hidden;
        display: -webkit-box;
        -webkit-box-orient: vertical;
        -webkit-line-clamp: 2;
        line-clamp: 2;
        white-space: normal;
        overflow-wrap: anywhere;
        font-weight: 600;
        font-size: var(--sbx-card-title);
        color: var(--color-text-primary);
    }
    .name-meta-row {
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        gap: 0.2rem 0.45rem;
        min-width: 0;
        font-size: var(--sbx-card-meta);
        color: var(--color-text-muted);
        line-height: 1.25;
    }
    .name-meta-row :global(.badge) {
        flex-shrink: 0;
        font-size: 10px;
        padding: 1px 5px;
    }
    .name-meta-row span {
        white-space: nowrap;
    }
    .lc-name .t2 {
        font-size: var(--sbx-card-meta);
        color: var(--color-text-muted);
    }
    .lc-endpoint {
        display: flex;
        align-items: flex-start;
        gap: 0.25rem;
        min-width: 0;
        overflow: hidden;
    }
    .lc-endpoint-stack {
        display: flex;
        flex-direction: column;
        align-items: flex-start;
        gap: 0.12rem;
        min-width: 0;
        flex: 1;
    }
    .lc-endpoint-name {
        width: 100%;
        font-size: var(--sbx-card-value);
        font-weight: 500;
        color: var(--color-text-primary);
        line-height: 1.2;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .lc-endpoint-host {
        width: 100%;
        font-size: var(--sbx-card-meta);
        line-height: 1.2;
        color: var(--color-text-muted);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
    .eye-mini {
        display: inline-flex;
        padding: 0.15rem;
        border: none;
        background: none;
        color: var(--color-text-muted);
        cursor: pointer;
        flex-shrink: 0;
        align-self: center;
    }
    .lc-actions {
        flex-wrap: nowrap;
        gap: 0.375rem;
        justify-content: flex-end;
        align-items: center;
        white-space: nowrap;
    }
    .lc-actions .action-btn {
        justify-content: center;
        padding: 0.375rem;
    }
</style>
