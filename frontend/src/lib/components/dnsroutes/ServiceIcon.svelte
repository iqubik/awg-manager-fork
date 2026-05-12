<script lang="ts">
	import { getServiceIcon } from '$lib/utils/service-icons';
	import { qureIconUrl, qureMatchByName } from '$lib/generated/qureIcons';

	interface Props {
		name: string;
		size?: number;
		iconUrl?: string;
	}

	let { name, size = 36, iconUrl }: Props = $props();

	let imgFailed = $state(false);

	// Reset failure state when iconUrl changes (new prop value, retry)
	$effect(() => {
		void iconUrl;
		imgFailed = false;
	});

	// Fallback chain:
	//   1. iconUrl set (and not failed)               → <img src={iconUrl}>
	//   2. exact case-insensitive Qure name match     → <img src={qureUrl}>
	//   3. keyword-detection inline SVG               → current ServiceIcon behavior
	//   4. default globe                              → handled by getServiceIcon
	let qureName = $derived(qureMatchByName(name));
	let qureUrl = $derived(qureName ? qureIconUrl(qureName) : null);

	let renderUrl = $derived.by(() => {
		if (iconUrl && !imgFailed) return iconUrl;
		if (!iconUrl && qureUrl) return qureUrl;
		return null;
	});

	let inlineIcon = $derived(getServiceIcon(name));
	let innerSize = $derived(Math.round(size * (inlineIcon.scale ?? 0.56)));
</script>

{#if renderUrl}
	<div
		class="service-icon img-wrapper"
		style="width: {size}px; height: {size}px;"
	>
		<img
			src={renderUrl}
			alt={name}
			width={size}
			height={size}
			loading="lazy"
			onerror={() => (imgFailed = true)}
		/>
	</div>
{:else}
	<div
		class="service-icon"
		style="width: {size}px; height: {size}px; background: {inlineIcon.background};"
	>
		<svg
			viewBox={inlineIcon.viewBox ?? '0 0 24 24'}
			width={innerSize}
			height={innerSize}
		>
			{@html inlineIcon.svg}
		</svg>
	</div>
{/if}

<style>
	.service-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		border-radius: 8px;
		flex-shrink: 0;
	}
	.img-wrapper {
		background: transparent;
		overflow: hidden;
	}
	.img-wrapper img {
		width: 100%;
		height: 100%;
		object-fit: contain;
	}
</style>
