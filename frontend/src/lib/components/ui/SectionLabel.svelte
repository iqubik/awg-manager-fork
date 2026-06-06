<script lang="ts" module>
  import type { Snippet } from 'svelte';
</script>

<script lang="ts">
  interface Props {
    children: Snippet;
  }
  let { children }: Props = $props();
</script>

<div class="section-label">{@render children()}</div>

<style>
  .section-label {
    font-size: var(--fs-xs);
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
    line-height: var(--lh-tight);
  }
</style>
