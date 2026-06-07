<script lang="ts">
	import { QURE_ICON_INNER_SCALE } from '$lib/utils/icon-tile-background';
	import {
		NDMS_ICON_TILE_RADIUS,
		NDMS_ICON_TILE_SIZE,
		ndmsIconTileInnerSize,
	} from '$lib/utils/ndms-icon-tile';

	interface Props {
		src: string;
		background: string;
		size?: number;
		innerScale?: number;
		alt?: string;
		onerror?: () => void;
	}

	let {
		src,
		background,
		size = NDMS_ICON_TILE_SIZE,
		innerScale,
		alt = '',
		onerror,
	}: Props = $props();

	const resolvedInnerScale = $derived(innerScale ?? QURE_ICON_INNER_SCALE);
	let innerSize = $derived(ndmsIconTileInnerSize(size, resolvedInnerScale));
</script>

<div
	class="icon-tile"
	style:width="{size}px"
	style:height="{size}px"
	style:background={background}
	style:border-radius="{NDMS_ICON_TILE_RADIUS}px"
>
	<img
		{src}
		{alt}
		width={innerSize}
		height={innerSize}
		loading="lazy"
		{onerror}
	/>
</div>

<style>
	.icon-tile {
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		overflow: hidden;
	}
	.icon-tile img {
		object-fit: contain;
	}
</style>
