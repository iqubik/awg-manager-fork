<script lang="ts">
    import { Button } from '$lib/components/ui';
    import CreateIcon from '$lib/components/ui/icons/CreateIcon.svelte';

    interface Props {
        disabled?: boolean;
        onclick: () => void;
        label?: string;
    }

    let { disabled = false, onclick, label = 'Создать' }: Props = $props();
</script>

{#snippet createIcon()}
    <CreateIcon />
{/snippet}

<Button variant="primary" size="sm" {disabled} {onclick} iconBefore={createIcon}>
    {label}
</Button>
