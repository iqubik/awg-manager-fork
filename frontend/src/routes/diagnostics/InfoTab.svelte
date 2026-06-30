<script lang="ts">
	import { onMount } from 'svelte';
	import { usageLevel } from '$lib/stores/settings';
	import AboutDeviceTab from './AboutDeviceTab.svelte';
	import DnsInfoTab from './DnsInfoTab.svelte';
	import InfoSpoiler from './InfoSpoiler.svelte';

	type InfoSectionId = 'about' | 'dns';

	const STORAGE_KEY = 'diagnostics.info.sections.open.v1';

	let openSections = $state<Record<InfoSectionId, boolean>>({
		about: true,
		dns: false,
	});
	let sectionsHydrated = $state(false);

	function readBool(value: unknown, fallback: boolean): boolean {
		return typeof value === 'boolean' ? value : fallback;
	}

	function normalizeInfoOpenSections(
		value: Partial<Record<InfoSectionId, unknown>> | null | undefined,
	): Record<InfoSectionId, boolean> {
		return {
			about: readBool(value?.about, true),
			dns: readBool(value?.dns, false),
		};
	}

	function toggleSection(id: InfoSectionId): void {
		openSections = {
			...openSections,
			[id]: !openSections[id],
		};
	}

	onMount(() => {
		try {
			const raw = localStorage.getItem(STORAGE_KEY);
			openSections = raw
				? normalizeInfoOpenSections(JSON.parse(raw) as Partial<Record<InfoSectionId, unknown>>)
				: normalizeInfoOpenSections(null);
		} catch {
			openSections = normalizeInfoOpenSections(null);
		} finally {
			sectionsHydrated = true;
		}
	});

	$effect(() => {
		if (typeof window === 'undefined' || !sectionsHydrated) return;
		try {
			localStorage.setItem(STORAGE_KEY, JSON.stringify(openSections));
		} catch {
			// ignore storage failures
		}
	});
</script>

<div class="info-tab">
	<InfoSpoiler
		title="Окружение"
		summary="Роутер, клиент, браузер, AWGM"
		open={openSections.about}
		onToggle={() => toggleSection('about')}
	>
		{#snippet body()}
			<AboutDeviceTab />
		{/snippet}
	</InfoSpoiler>

	{#if $usageLevel === 'expert'}
		<InfoSpoiler
			title="Сведения о DNS"
			summary="Апстримы, политики, статические записи, rebind"
			open={openSections.dns}
			onToggle={() => toggleSection('dns')}
		>
			{#snippet body()}
				<DnsInfoTab />
			{/snippet}
		</InfoSpoiler>
	{/if}
</div>

<style>
	.info-tab {
		display: flex;
		flex-direction: column;
		gap: 0.875rem;
		padding-top: 0.35rem;
	}
</style>
