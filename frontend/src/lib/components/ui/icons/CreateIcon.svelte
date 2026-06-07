<script lang="ts">
	import { Plus } from 'lucide-svelte';

	interface Props {
		size?: number;
	}

	let { size = 14 }: Props = $props();
</script>

<Plus {size} strokeWidth={2} aria-hidden="true" />
