<!--
  Mobile adaptation reference: singbox-router/project/parts/MobileShell.jsx (MBottomBar)
  Sticky bottom CTA wrapper. Visible только на mobile (<768px), hidden на desktop.
-->

<script lang="ts">
  import type { Snippet } from 'svelte';

  interface Props {
    children: Snippet;
  }

  let { children }: Props = $props();
</script>

<div class="bar">
  {@render children()}
</div>

<style>
  .bar {
    display: none;
  }
  @media (max-width: 768px) {
    .bar {
      display: flex;
      position: sticky;
      bottom: 0;
      z-index: 5;
      padding: 10px 14px 14px;
      background: color-mix(in srgb, var(--bg-primary) 92%, transparent);
      border-top: 1px solid var(--border);
      backdrop-filter: blur(8px);
      gap: 8px;
      align-items: center;
    }
  }
</style>
