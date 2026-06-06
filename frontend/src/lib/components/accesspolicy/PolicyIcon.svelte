<script lang="ts">
	import NdmsIconTile from '$lib/components/ui/NdmsIconTile.svelte';
	import { DEFAULT_ICON_TILE_BG } from '$lib/utils/icon-tile-background';
	import {
		ndmsIconTileInnerSize,
		NDMS_ICON_TILE_SIZE,
	} from '$lib/utils/ndms-icon-tile';
	import {
		getPolicyInlineSvg,
		getPolicyIconComponent,
		resolvePolicyIcon,
	} from '$lib/utils/policy-icon';

	interface Props {
		label: string;
		policyName?: string;
		isHydraRoute?: boolean;
		size?: number;
		strokeWidth?: number;
	}

	let {
		label,
		policyName = '',
		isHydraRoute = false,
		size = NDMS_ICON_TILE_SIZE,
		strokeWidth = 1.75,
	}: Props = $props();

	const iconId = $derived(resolvePolicyIcon(label, { policyName, isHydraRoute }));
	const inlineSvg = $derived(getPolicyInlineSvg(iconId));
	const Icon = $derived(getPolicyIconComponent(iconId));
	const innerSize = $derived(ndmsIconTileInnerSize(size));
</script>

<NdmsIconTile background={DEFAULT_ICON_TILE_BG} {size}>
	{#if inlineSvg}
		<svg
			class="policy-inline-icon"
			viewBox={inlineSvg.viewBox}
			width={innerSize}
			height={innerSize}
			aria-hidden="true"
		>
			{#each inlineSvg.paths as path (path)}
				<path d={path} fill="currentColor" />
			{/each}
		</svg>
	{:else if Icon}
		<Icon size={innerSize} {strokeWidth} />
	{/if}
</NdmsIconTile>

<style>
	.policy-inline-icon {
		display: block;
		flex-shrink: 0;
	}
</style>
