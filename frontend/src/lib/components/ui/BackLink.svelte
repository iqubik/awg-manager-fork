<script lang="ts">
	import { ArrowLeft } from 'lucide-svelte';

	interface Props {
		href: string;
		label?: string;
	}

	let { href, label = 'Назад' }: Props = $props();
</script>

<a {href} class="back-link">
	<ArrowLeft size={20} strokeWidth={2} aria-hidden="true" />
	{label}
</a>
