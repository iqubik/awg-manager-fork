<script lang="ts">
	import { onMount } from 'svelte';
	import { api } from '$lib/api/client';
	import { singboxRouter } from '$lib/stores/singboxRouter';
	import { singboxTunnels } from '$lib/stores/singbox';
	import type { AWGTagInfo, SingboxTunnel } from '$lib/types';
	import { buildOutboundOptions, PresetsGallery } from '$lib/components/routing/singboxRouter';

	const presetsStore = singboxRouter.presets;
	const outboundsStore = singboxRouter.outbounds;
	const phase1Store = singboxTunnels;

	const presets = $derived($presetsStore);
	const outbounds = $derived($outboundsStore);
	const phase1Tunnels = $derived(($phase1Store.data ?? []) as SingboxTunnel[]);

	let awgTags = $state<AWGTagInfo[]>([]);

	async function loadAWGTags(): Promise<void> {
		try {
			awgTags = await api.getAWGTags();
		} catch {
			awgTags = [];
		}
	}

	onMount(() => {
		loadAWGTags();
	});

	const outboundOptions = $derived(
		buildOutboundOptions(awgTags, phase1Tunnels, outbounds, true),
	);
</script>

<PresetsGallery
	{presets}
	{outboundOptions}
	onApplied={() => singboxRouter.loadAll()}
/>
