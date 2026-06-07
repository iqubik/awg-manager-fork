<script lang="ts">
	import NdmsIconTile from '$lib/components/ui/NdmsIconTile.svelte';
	import { settingsSectionIconMode } from '$lib/stores/settingsSectionIconMode';
	import { resolveNdmsCardIconStyle } from '$lib/utils/ndms-card-icon-style';
	import {
		ndmsIconTileInnerSize,
		NDMS_ICON_TILE_SIZE,
	} from '$lib/utils/ndms-icon-tile';
	import {
		getPolicyInlineSvg,
		getPolicyIconColor,
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
	const tileStyle = $derived(
		resolveNdmsCardIconStyle($settingsSectionIconMode, getPolicyIconColor(iconId)),
	);
</script>

{#key $settingsSectionIconMode}
	<NdmsIconTile background={tileStyle.background} foreground={tileStyle.foreground} {size}>
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
			<Icon size={innerSize} {strokeWidth} color="currentColor" />
		{/if}
	</NdmsIconTile>
{/key}

<style>
	.policy-inline-icon {
		display: block;
		flex-shrink: 0;
	}
</style>
