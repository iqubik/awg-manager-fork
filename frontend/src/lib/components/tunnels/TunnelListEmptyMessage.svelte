<script lang="ts">
	interface Props {
		show?: boolean;
	}

	let { show = false }: Props = $props();
</script>

{#if show}
	<p class="tunnel-list-empty">Ничего не найдено</p>
{/if}
