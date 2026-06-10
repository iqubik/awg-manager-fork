<!--
  Источник дизайна: singbox-router/project/parts/RuleCard.jsx (ActionTile)
-->

<script lang="ts" module>
  export type OutboundTileSize = 'default' | 'compact';
</script>

<script lang="ts">
  import type { OutboundDisplay } from './types';
  import { displayTone, toneClass } from './outboundTileTone';
  import OutboundToneIcon from './OutboundToneIcon.svelte';
  import OutboundChipLabel from './OutboundChipLabel.svelte';
  import { outboundDisplayTitle } from './outboundLabelFormat';

  interface Props {
    outbound: OutboundDisplay;
    /** default — RuleCard; compact — expert tables (как Badge sm: remote, inline). */
    size?: OutboundTileSize;
  }
  let { outbound, size = 'default' }: Props = $props();

  let tone = $derived(displayTone(outbound));
  let cls = $derived(
    `tone-chip ${toneClass(tone)}${size === 'compact' ? ' tone-chip-compact' : ''}`,
  );
  let iconSize = $derived(size === 'compact' ? 10 : 14);
</script>

{#if outbound.kind === 'block'}
  <div class={cls}>
    <OutboundToneIcon {tone} kind={outbound.kind} size={iconSize} />
    <span>Заблокировать</span>
  </div>
{:else if outbound.kind === 'direct'}
  <div class={cls}>
    <OutboundToneIcon {tone} kind={outbound.kind} size={iconSize} />
    <span>Напрямую</span>
  </div>
{:else}
  <div class={cls} title={outbound.kind === 'unknown' ? 'Outbound не найден в конфиге' : undefined}>
    <OutboundToneIcon {tone} kind={outbound.kind} size={iconSize} />
    <OutboundChipLabel
      label={outbound.label}
      metaSuffix={outbound.metaSuffix}
      title={outboundDisplayTitle(outbound)}
    />
  </div>
{/if}
