<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import type { SingboxTunnel, Subscription, TunnelListItem } from '$lib/types';
	import type { DiagnosticsTargetSeed } from '$lib/stores/diagnostics';
	import { PageContainer, PageHeader } from '$lib/components/layout';
	import { Tabs, MobileTabRail } from '$lib/components/ui';
	import { LogsTerminal } from '$lib/components/diagnostics';
	import { settings, usageLevel } from '$lib/stores/settings';
	import ConnectionsTab from './ConnectionsTab.svelte';
	import ChecksTab from './ChecksTab.svelte';
	import AwgConfigAnalyzerTab from './AwgConfigAnalyzerTab.svelte';
	import AboutDeviceTab from './AboutDeviceTab.svelte';
	import DnsInfoTab from './DnsInfoTab.svelte';
	import { MonitoringTab } from '$lib/components/pingcheck';
	import { readTunnelMobileLayout, subscribeTunnelMobileLayout } from '$lib/constants/singboxLayout';
	import { buildDiagnosticsTargets } from '$lib/utils/diagnosticsTargets';

	type ActiveTab = 'logs' | 'monitoring' | 'connections' | 'checks' | 'about' | 'awgConfig' | 'dns';

	function initialDiagnosticsTab(): ActiveTab {
		const tab = $page.url.searchParams.get('tab');

		if (tab === 'monitoring') return 'monitoring';
		if (tab === 'connections') return 'connections';
		if (tab === 'checks') return 'checks';
		if (tab === 'about') return 'about';
		if (tab === 'awgConfig') return 'awgConfig';
		if (tab === 'dns') return 'dns';

		// legacy aliases, чтобы первый render тоже сразу попадал в checks
		if (tab === 'tests' || tab === 'dnscheck') return 'checks';

		return 'logs';
	}

	let activeTab = $state<ActiveTab>(initialDiagnosticsTab());
	let tunnels = $state<DiagnosticsTargetSeed[]>([]);
	let isMobileTabs = $state(readTunnelMobileLayout());

	const diagnosticsTabs = $derived.by((): { id: ActiveTab; label: string }[] => {
		const base: { id: ActiveTab; label: string }[] = [
			{ id: 'logs', label: 'Журнал' },
			{ id: 'monitoring', label: 'Мониторинг' },
			{ id: 'connections', label: 'Соединения' },
			{ id: 'checks', label: 'Проверки' },
			{ id: 'about', label: 'Окружение' },
		];
		if ($usageLevel === 'expert') {
			base.push({ id: 'awgConfig', label: 'Конфиг AWG' });
		}
		if ($usageLevel === 'expert') {
			base.push({ id: 'dns', label: 'Сведения о DNS' });
		}
		return base;
	});

	$effect(() => {
		// Пока настройки не загружены usageLevel имеет fallback 'advanced'
		// и guard может преждевременно сбросить awgConfig на logs + вычистить URL.
		// Ждём загрузки settings — Tabs сам восстановит вкладку из URL.
		if ($settings === null) return;
		if ($usageLevel === 'expert') return;
		if (activeTab === 'awgConfig' || activeTab === 'dns') {
			activeTab = 'logs';
		}
		const tab = $page.url.searchParams.get('tab');
		if (tab === 'awgConfig' || tab === 'dns') {
			const url = new URL($page.url);
			url.searchParams.delete('tab');
			const q = url.searchParams.toString();
			const target = url.pathname + (q ? `?${q}` : '') + url.hash;
			void goto(target, { replaceState: true, keepFocus: true, noScroll: true });
		}
	});

	// Legacy URL sanitizer — rewrite ?tab=tests / ?tab=dnscheck (which used
	// to render the health rail inside the logs tab) to ?tab=checks BEFORE
	// the Tabs primitive reads the URL. Runs synchronously at init.
	{
		const sp = new URLSearchParams($page.url.search);
		const t = sp.get('tab');
		if (t === 'tests' || t === 'dnscheck') {
			sp.set('tab', 'checks');
			const url = $page.url.pathname + (sp.toString() ? `?${sp}` : '') + $page.url.hash;
			void goto(url, { replaceState: true, keepFocus: true, noScroll: true });
		}
	}

	onMount(async () => {
		// Combine three target sources for the diagnostics rail:
		//   1. AWG/managed tunnels (snap.tunnels) — system NativeWG and external
		//      adopted tunnels are excluded; diagnostics must not run against them.
		//   2. Sing-box tunnels (one row per outbound).
		//   3. Active+enabled subscription members (sing-box prefixed).
		// Failures in optional sources degrade silently to empty list.
		try {
			const [snap, singboxTunnels, subscriptions] = await Promise.all([
				api.getTunnelsAll(),
				api.singboxListTunnels().catch(() => [] as SingboxTunnel[]),
				api.listSubscriptions().catch(() => [] as Subscription[]),
			]);

			tunnels = buildDiagnosticsTargets(
				(snap.tunnels ?? []) as TunnelListItem[],
				singboxTunnels,
				subscriptions,
			);
		} catch {
			tunnels = [];
		}
	});

	onMount(() => subscribeTunnelMobileLayout((mobile) => {
		isMobileTabs = mobile;
	}));

	const pageTitle = $derived(
		activeTab === 'connections' ? 'Соединения · Инструменты' :
		activeTab === 'checks' ? 'Проверки · Инструменты' :
		activeTab === 'about' ? 'Окружение · Инструменты' :
		activeTab === 'awgConfig' ? 'Конфиг AWG · Инструменты' :
		activeTab === 'dns' ? 'Сведения о DNS · Инструменты' :
		activeTab === 'monitoring' ? 'Мониторинг · Инструменты' :
		'Журнал · Инструменты',
	);

</script>

<svelte:head>
	<title>{pageTitle} - AWG Manager</title>
</svelte:head>

<PageContainer width="full">
	<PageHeader title="Инструменты" />

	{#if isMobileTabs}
		<MobileTabRail
			tabs={diagnosticsTabs}
			active={activeTab}
			onchange={(id) => (activeTab = id as ActiveTab)}
			urlParam="tab"
			defaultTab="logs"
			ariaLabel="Diagnostics sections"
		/>
	{:else}
		<Tabs
			tabs={diagnosticsTabs}
			active={activeTab}
			onchange={(id) => (activeTab = id as ActiveTab)}
			urlParam="tab"
			defaultTab="logs"
		/>
	{/if}

	{#if activeTab === 'logs'}
		<LogsTerminal />
	{:else if activeTab === 'monitoring'}
		<MonitoringTab />
	{:else if activeTab === 'connections'}
		<ConnectionsTab />
	{:else if activeTab === 'checks'}
		<ChecksTab {tunnels} />
	{:else if activeTab === 'about'}
		<AboutDeviceTab />
	{:else if activeTab === 'awgConfig'}
		<AwgConfigAnalyzerTab />
	{:else if activeTab === 'dns'}
		<DnsInfoTab />
	{/if}
</PageContainer>
