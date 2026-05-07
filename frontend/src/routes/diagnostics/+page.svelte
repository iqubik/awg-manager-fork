<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import type { TunnelListItem, SingboxTunnel, Subscription } from '$lib/types';
	import type { DiagnosticsTargetSeed } from '$lib/stores/diagnostics';
	import { PageContainer, PageHeader } from '$lib/components/layout';
	import { Tabs } from '$lib/components/ui';
	import { LogsTerminal } from '$lib/components/diagnostics';
	import ConnectionsTab from './ConnectionsTab.svelte';
	import ChecksTab from './ChecksTab.svelte';

	type ActiveTab = 'logs' | 'connections' | 'checks';

	let activeTab = $state<ActiveTab>('logs');
	let tunnels = $state<DiagnosticsTargetSeed[]>([]);

	const diagnosticsTabs = [
		{ id: 'logs', label: 'Журнал' },
		{ id: 'connections', label: 'Соединения' },
		{ id: 'checks', label: 'Проверки' },
	];

	// Deep-link: ?tab=logs|connections|checks. Legacy `tests`/`dnscheck`
	// (which used to render the health rail inside the logs tab) now map
	// to `checks` since the rail moved into its own tab.
	$effect(() => {
		const tabParam = $page.url.searchParams.get('tab');
		if (tabParam === 'connections') {
			activeTab = 'connections';
		} else if (tabParam === 'checks' || tabParam === 'tests' || tabParam === 'dnscheck') {
			activeTab = 'checks';
		} else {
			activeTab = 'logs';
		}
	});

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

			const awg: DiagnosticsTargetSeed[] = (snap.tunnels ?? []).map((t: TunnelListItem) => ({
				id: t.id,
				name: t.name,
				status: t.status,
			}));

			const singbox: DiagnosticsTargetSeed[] = singboxTunnels.map((t) => ({
				id: `singbox:${t.tag}`,
				name: t.tag,
				status: t.running ? 'running' : 'stopped',
			}));

			const subscriptionMembers: DiagnosticsTargetSeed[] = [];
			for (const sub of subscriptions) {
				if (!sub.enabled) continue;
				for (const m of sub.members ?? []) {
					subscriptionMembers.push({
						id: `singbox:${m.tag}`,
						name: m.label || m.tag,
						// Members are checked through the sing-box process,
						// so default to 'running' for rail visibility.
						status: 'running',
					});
				}
			}

			const uniq = new Map<string, DiagnosticsTargetSeed>();
			for (const t of [...awg, ...singbox, ...subscriptionMembers]) {
				if (!uniq.has(t.id)) uniq.set(t.id, t);
			}
			tunnels = Array.from(uniq.values());
		} catch {
			tunnels = [];
		}
	});

	const pageTitle = $derived(
		activeTab === 'connections' ? 'Соединения · Диагностика' :
		activeTab === 'checks' ? 'Проверки · Диагностика' :
		'Журнал · Диагностика',
	);
</script>

<svelte:head>
	<title>{pageTitle} - AWG Manager</title>
</svelte:head>

<PageContainer width="full">
	<PageHeader title="Диагностика" />

	<Tabs
		tabs={diagnosticsTabs}
		active={activeTab}
		onchange={(id) => (activeTab = id as ActiveTab)}
	/>

	{#if activeTab === 'logs'}
		<LogsTerminal />
	{:else if activeTab === 'connections'}
		<ConnectionsTab />
	{:else if activeTab === 'checks'}
		<ChecksTab {tunnels} />
	{/if}
</PageContainer>
