<script lang="ts">
	import { Badge, type BadgeSize } from '$lib/components/ui';

	interface Props {
		defaultRoute?: boolean;
		size?: BadgeSize;
	}

	let { defaultRoute = false, size = 'sm' }: Props = $props();
</script>

{#if !defaultRoute}
	<Badge
		variant="error"
		{size}
		title="Интерфейс может не работать в политиках доступа"
	>
		not default
	</Badge>
{/if}
