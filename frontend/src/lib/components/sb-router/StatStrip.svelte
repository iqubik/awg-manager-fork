<!--
  Источник дизайна: singbox-router/project/screens/MainExpert.jsx (StatCell + status strip)
-->

<script lang="ts" module>
  export interface StatCellData {
    label: string;
    value: string;
    tone?: 'success' | 'error' | 'muted' | 'default';
    helpTitle?: string;
    helpText?: string;
    helpItems?: string[];
  }
</script>

<script lang="ts">
  type TooltipPlacement = 'top' | 'bottom';

  interface Props {
    cells: StatCellData[];
  }
  let { cells }: Props = $props();

  const VIEWPORT_PAD = 8;
  const TOOLTIP_GAP = 10;
  const TOOLTIP_WIDTH = 288;

  let activeIndex = $state<number | null>(null);
  let tooltipWidth = $state(TOOLTIP_WIDTH);
  let tooltipX = $state(0);
  let tooltipY = $state(0);
  let tooltipPlacement = $state<TooltipPlacement>('top');

  function colorFor(tone?: StatCellData['tone']): string {
    switch (tone) {
      case 'success': return 'var(--color-success, #22c55e)';
      case 'error': return 'var(--color-error, #dc2626)';
      case 'muted': return 'var(--text-muted)';
      default: return 'var(--text-primary)';
    }
  }

  function clamp(value: number, min: number, max: number): number {
    return Math.min(Math.max(value, min), max);
  }

  function closeTooltip(): void {
    activeIndex = null;
  }

  function hideTooltip(index?: number): void {
    if (index === undefined || activeIndex === index) {
      activeIndex = null;
    }
  }

  function showTooltip(index: number, event: MouseEvent | FocusEvent): void {
    const target = event.currentTarget;
    if (!(target instanceof HTMLElement)) return;

    const rect = target.getBoundingClientRect();
    const width = Math.max(
      200,
      Math.min(TOOLTIP_WIDTH, window.innerWidth - VIEWPORT_PAD * 2),
    );
    tooltipWidth = width;

    const x = clamp(
      rect.left + rect.width / 2,
      VIEWPORT_PAD + width / 2,
      window.innerWidth - VIEWPORT_PAD - width / 2,
    );

    const placeBottom = rect.top < 150;

    tooltipPlacement = placeBottom ? 'bottom' : 'top';
    tooltipX = x;
    tooltipY = placeBottom
      ? rect.bottom + TOOLTIP_GAP
      : rect.top - TOOLTIP_GAP;

    activeIndex = index;
  }

  function handleKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') closeTooltip();
  }
</script>

<svelte:window
  onscroll={closeTooltip}
  onresize={closeTooltip}
  onkeydown={handleKeydown}
/>

<div class="strip" style:--cols={cells.length}>
  {#each cells as cell, i (i)}
    {@const tipId = `stat-tip-${i}`}
    <div class="cell-shell">
      <div class="cell">
        <div class="label">{cell.label}</div>
        <div class="value" style:color={colorFor(cell.tone)}>{cell.value}</div>

        {#if cell.helpText}
          <button
            type="button"
            class="help-btn"
            aria-label={`Подсказка: ${cell.label}`}
            aria-describedby={activeIndex === i ? tipId : undefined}
            onmouseenter={(event) => showTooltip(i, event)}
            onmouseleave={() => hideTooltip(i)}
            onfocus={(event) => showTooltip(i, event)}
            onblur={() => hideTooltip(i)}
          >
            ?
          </button>
        {/if}
      </div>

      {#if activeIndex === i && cell.helpText}
        <div
          id={tipId}
          class="stat-tooltip"
          class:bottom={tooltipPlacement === 'bottom'}
          role="tooltip"
          style:width={`${tooltipWidth}px`}
          style:left={`${tooltipX}px`}
          style:top={`${tooltipY}px`}
        >
          <div class="tooltip-title">{cell.helpTitle ?? cell.label}</div>
          <p>{cell.helpText}</p>
          {#if cell.helpItems?.length}
            <ul>
              {#each cell.helpItems as item}
                <li>{item}</li>
            {/each}
          </ul>
          {/if}
        </div>
      {/if}
    </div>
  {/each}
</div>

<style>
  .strip {
    display: grid;
    grid-template-columns: repeat(var(--cols, 7), minmax(0, 1fr));
    gap: 0.5rem;
    margin: 0.875rem 0 1rem;
    position: relative;
    overflow: visible;
  }
  .cell {
    min-width: 0;
    width: 100%;
    min-height: 5.25rem;
    padding: 0.75rem 0.8rem;
    border: 1px solid var(--border);
    border-radius: 10px;
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.035), rgba(255, 255, 255, 0)),
      var(--bg-secondary);
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    position: relative;
    overflow: visible;
    box-sizing: border-box;
    cursor: default;
    transition:
      background-color 0.15s ease,
      border-color 0.15s ease,
      transform 0.15s ease,
      box-shadow 0.15s ease;
  }
  .cell-shell {
    min-width: 0;
    width: 100%;
    overflow: visible;
  }
  .label {
    min-width: 0;
    font-size: 10px;
    line-height: 1.15;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .value {
    min-width: 0;
    margin-top: 0.35rem;
    font-size: 18px;
    line-height: 1;
    font-weight: 700;
    font-family: var(--font-mono);
    white-space: nowrap;
  }
  .help-btn {
    position: absolute;
    top: 0.45rem;
    right: 0.45rem;
    width: 1.25rem;
    height: 1.25rem;
    border-radius: 999px;
    border: 1px solid var(--border);
    background: color-mix(in srgb, var(--bg-tertiary) 80%, transparent);
    color: var(--text-muted);
    font-size: 0.72rem;
    line-height: 1;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    padding: 0;
  }
  .help-btn:hover,
  .help-btn:focus-visible {
    color: var(--text-primary);
    border-color: color-mix(in srgb, var(--accent) 45%, var(--border));
    outline: none;
  }
  .stat-tooltip {
    position: fixed;
    z-index: 1000;
    width: max-content;
    max-width: min(18rem, calc(100vw - 16px));
    transform: translate(-50%, -100%);
    pointer-events: auto;
    padding: 0.75rem 0.85rem;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    background: var(--bg-secondary);
    box-shadow: 0 14px 40px rgba(0, 0, 0, 0.35);
    color: var(--text-secondary);
    font-size: 0.75rem;
    line-height: 1.4;
    text-transform: none;
    letter-spacing: normal;
    overflow: visible;
  }
  @media (hover: hover) and (pointer: fine) {
    .cell:hover {
    background:
      linear-gradient(180deg, rgba(255, 255, 255, 0.05), rgba(255, 255, 255, 0)),
      color-mix(in srgb, var(--bg-hover) 70%, transparent);
    border-color: color-mix(in srgb, var(--accent) 35%, var(--border));
    transform: translateY(-1px);
    }
  }
  .stat-tooltip::after {
    content: '';
    position: absolute;
    left: 50%;
    top: 100%;
    width: 0.55rem;
    height: 0.55rem;
    transform: translate(-50%, -50%) rotate(45deg);
    border-right: 1px solid var(--border);
    border-bottom: 1px solid var(--border);
    background: var(--bg-secondary);
  }
  .stat-tooltip.bottom {
    transform: translate(-50%, 0);
  }
  .stat-tooltip.bottom::after {
    top: 0;
    transform: translate(-50%, -50%) rotate(45deg);
    border-right: 0;
    border-bottom: 0;
    border-left: 1px solid var(--border);
    border-top: 1px solid var(--border);
  }
  .tooltip-title {
    margin-bottom: 0.35rem;
    font-size: 0.72rem;
    font-weight: 700;
    color: var(--text-primary);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .stat-tooltip p {
    margin: 0;
  }
  .stat-tooltip ul {
    margin: 0.45rem 0 0;
    padding-left: 1rem;
  }
  .stat-tooltip li + li {
    margin-top: 0.2rem;
  }
  @media (max-width: 768px) {
    .strip {
      grid-template-columns: repeat(12, minmax(0, 1fr));
      gap: 0.5rem;
      margin: 0.75rem 0 0.875rem;
    }
    .cell {
      min-height: 4.5rem;
      padding: 0.65rem 0.7rem;
      border-radius: 9px;
    }
    .cell-shell:nth-child(-n + 3) {
      grid-column: span 4;
    }
    .cell-shell:nth-child(n + 4) {
      grid-column: span 3;
    }
    .stat-tooltip::after {
      left: 1.25rem;
    }
    .stat-tooltip.bottom::after {
      left: 1.25rem;
    }
  }
  @media (hover: none), (pointer: coarse) {
    .help-btn {
      display: none;
    }
  }
  @media (max-width: 360px) {
    .strip {
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 0.375rem;
    }
    .cell {
      padding: 0.45rem;
    }
    .cell-shell,
    .cell-shell:nth-child(-n + 3),
    .cell-shell:nth-child(n + 4) {
      grid-column: auto;
    }
    .stat-tooltip {
      max-width: min(17rem, 86vw);
    }
  }
</style>
