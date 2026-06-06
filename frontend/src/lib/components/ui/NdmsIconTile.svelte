<script lang="ts">
	import type { Snippet } from 'svelte';
	import { NDMS_ICON_TILE_CLIP_PATH, NDMS_ICON_TILE_RADIUS, NDMS_ICON_TILE_SIZE } from '$lib/utils/ndms-icon-tile';

	interface Props {
		background: string;
		size?: number;
		children?: Snippet;
	}

	let { background, size = NDMS_ICON_TILE_SIZE, children }: Props = $props();
</script>

<div
	class="ndms-icon-tile"
	style:width="{size}px"
	style:height="{size}px"
	style:background
	style:border-radius="{NDMS_ICON_TILE_RADIUS}px"
	style:clip-path={NDMS_ICON_TILE_CLIP_PATH}
>
	{@render children?.()}
</div>

<style>
	.ndms-icon-tile {
		display: flex;
		align-items: center;
		justify-content: center;
		flex-shrink: 0;
		overflow: hidden;
		color: var(--text-primary);
	}
</style>
