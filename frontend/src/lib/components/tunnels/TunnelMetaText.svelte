<script lang="ts">
	interface Props {
		mono?: boolean;
		error?: boolean;
		class?: string;
		children?: import('svelte').Snippet;
	}

	let { mono = false, error = false, class: className = '', children }: Props = $props();
</script>

<div
	class="tunnel-meta-text {className}"
	class:tunnel-meta-text--mono={mono}
	class:tunnel-meta-text--error={error}
>
	{#if children}
		{@render children()}
	{/if}
</div>
