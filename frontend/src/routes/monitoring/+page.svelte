<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';

	onMount(() => {
		const url = new URL(window.location.href);
		url.pathname = '/diagnostics';
		url.searchParams.set('tab', 'monitoring');
		void goto(url.pathname + url.search + url.hash, { replaceState: true });
	});
</script>
