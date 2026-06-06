<script lang="ts">
    import { onMount, onDestroy } from 'svelte';
    import { Button } from '$lib/components/ui';
    import CreateIcon from '$lib/components/ui/icons/CreateIcon.svelte';

    interface Props {
        label?: string;
        disabled?: boolean;
        oncatalog?: () => void;
        onmanual: () => void;
        importEnabled?: boolean;
        importLabel?: string;
        onimport?: () => void;
    }

    let {
        label = 'Добавить',
        disabled = false,
        oncatalog,
        onmanual,
        importEnabled = false,
        importLabel = 'Загрузить конфигурацию',
        onimport,
    }: Props = $props();

    let menuOpen = $state(false);

    function handleClickOutside() {
        menuOpen = false;
    }

    onMount(() => document.addEventListener('click', handleClickOutside));
    onDestroy(() => document.removeEventListener('click', handleClickOutside));
</script>

{#snippet createIcon()}
    <CreateIcon />
{/snippet}

<div class="dropdown-wrapper">
    <Button
        variant="primary"
        size="sm"
        {disabled}
        onclick={(e) => {
            e.stopPropagation();
            menuOpen = !menuOpen;
        }}
        iconBefore={createIcon}
    >
        {label}
        {#snippet iconAfter()}
            <svg width="10" height="10" viewBox="0 0 10 10" fill="currentColor">
                <path d="M2 4l3 3 3-3" />
            </svg>
        {/snippet}
    </Button>
    {#if menuOpen}
        <div class="dropdown-menu">
            {#if oncatalog}
                <button
                    type="button"
                    class="dropdown-item"
                    onclick={() => {
                        menuOpen = false;
                        oncatalog();
                    }}
                >
                    <svg
                        class="dropdown-icon"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                    >
                        <rect x="3" y="3" width="7" height="7" />
                        <rect x="14" y="3" width="7" height="7" />
                        <rect x="3" y="14" width="7" height="7" />
                        <rect x="14" y="14" width="7" height="7" />
                    </svg>
                    Из каталога
                </button>
            {/if}
            <button
                type="button"
                class="dropdown-item"
                onclick={() => {
                    menuOpen = false;
                    onmanual();
                }}
            >
                <svg
                    class="dropdown-icon"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    stroke-width="2"
                >
                    <line x1="12" y1="5" x2="12" y2="19" />
                    <line x1="5" y1="12" x2="19" y2="12" />
                </svg>
                Создать вручную
            </button>
            {#if importEnabled && onimport}
                <div class="dropdown-sep"></div>
                <button
                    type="button"
                    class="dropdown-item"
                    onclick={() => {
                        menuOpen = false;
                        onimport();
                    }}
                >
                    <svg
                        class="dropdown-icon"
                        viewBox="0 0 24 24"
                        fill="none"
                        stroke="currentColor"
                        stroke-width="2"
                    >
                        <path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4" />
                        <polyline points="17 8 12 3 7 8" />
                        <line x1="12" y1="3" x2="12" y2="15" />
                    </svg>
                    {importLabel}
                </button>
            {/if}
        </div>
    {/if}
</div>

<style>
    .dropdown-wrapper {
        position: relative;
        display: inline-block;
    }

    .dropdown-menu {
        position: absolute;
        top: calc(100% + 4px);
        right: 0;
        z-index: 10;
        background: var(--bg-secondary, var(--bg-card, #1a1b2e));
        border: 1px solid var(--border);
        border-radius: 8px;
        box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
        min-width: 210px;
        padding: 4px;
    }

    .dropdown-item {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 0.5rem 0.75rem;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.8125rem;
        color: var(--text-secondary);
        border: none;
        background: none;
        width: 100%;
        text-align: left;
        font-family: inherit;
        transition: background 0.1s;
    }

    .dropdown-item:hover {
        background: var(--bg-hover);
        color: var(--text-primary);
    }

    :global(.dropdown-icon) {
        width: 16px;
        height: 16px;
        flex-shrink: 0;
        color: var(--text-muted);
    }

    .dropdown-item:hover :global(.dropdown-icon) {
        color: var(--accent);
    }

    .dropdown-sep {
        height: 1px;
        background: var(--border);
        margin: 4px 8px;
    }

    @media (max-width: 640px) {
        .dropdown-wrapper {
            display: block;
            width: 100%;
        }

        .dropdown-wrapper :global(.btn) {
            width: 100%;
            justify-content: center;
        }
    }
</style>
