<script lang="ts">
	import { Gauge } from 'lucide-svelte';

	interface Props {
		size?: number;
	}

	let { size = 14 }: Props = $props();
</script>

<Gauge {size} strokeWidth={2} aria-hidden="true" />
