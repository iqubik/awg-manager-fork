<!--
  Действие правила в простом режиме: обычный outbound-tile или раскрытый
  composite со списком участников (как RoutingTargetBadges в DNS/политиках).
-->

<script lang="ts">
  import type { OutboundDisplay } from './types';
  import OutboundTile from './OutboundTile.svelte';
  import RoutingTargetBadges from '$lib/components/routing/RoutingTargetBadges.svelte';

  interface Props {
    outbound: OutboundDisplay;
    /** Ширина для fit composite-бейджей (px), см. RuleCard.trail. */
    badgeBudget?: number;
  }

  let { outbound, badgeBudget }: Props = $props();

  const showComposite = $derived(
    outbound.kind === 'composite' && (outbound.memberLabels?.length ?? 0) > 0,
  );
</script>

<div class="wrap" class:composite={showComposite}>
  {#if showComposite}
    <RoutingTargetBadges
      variant="tunnel"
      labels={outbound.memberLabels!}
      titles={outbound.memberTitles ?? outbound.memberLabels!}
      overflowNoun="туннелей"
      budgetWidth={badgeBudget}
    />
  {:else}
    <svg class="arrow" viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
      <line x1="5" y1="12" x2="19" y2="12" />
      <polyline points="12 5 19 12 12 19" />
    </svg>
    <OutboundTile {outbound} />
  {/if}
</div>

<style>
  .wrap {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-shrink: 0;
    min-width: 0;
    max-width: 100%;
  }

  .wrap.composite {
    flex: 0 1 auto;
    min-width: 0;
    max-width: 100%;
  }

  .arrow {
    color: var(--text-muted);
    flex-shrink: 0;
  }

  .wrap :global(.fitting-badges) {
    min-width: 0;
    max-width: 100%;
  }
</style>
