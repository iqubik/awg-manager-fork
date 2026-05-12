<script lang="ts">
	interface Props {
		data: number[];
		width?: number;
		height?: number;
		color?: string;
	}

	let {
		data = [],
		width = 92,
		height = 28,
		color = 'var(--color-accent)',
	}: Props = $props();

	const padding = 2;

	const points = $derived.by(() => {
		if (data.length === 0) return '';

		const maxValue = Math.max(...data, 1);
		const innerWidth = Math.max(width - padding * 2, 1);
		const innerHeight = Math.max(height - padding * 2, 1);

		return data
			.map((value, index) => {
				const x =
					data.length === 1
						? width / 2
						: padding + (innerWidth * index) / (data.length - 1);
				const y = padding + innerHeight * (1 - value / maxValue);
				return `${x},${y}`;
			})
			.join(' ');
	});
</script>

<svg {width} {height} viewBox={`0 0 ${width} ${height}`} role="img" aria-label="Traffic sparkline">
	<line
		x1={padding}
		x2={width - padding}
		y1={height - padding}
		y2={height - padding}
		class="baseline"
	/>
	{#if points}
		<polyline points={points} fill="none" stroke={color} stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" />
	{/if}
</svg>

<style>
	svg {
		display: block;
		flex-shrink: 0;
	}

	.baseline {
		stroke: var(--color-border-hover);
		stroke-width: 1;
		opacity: 0.7;
	}
</style>
