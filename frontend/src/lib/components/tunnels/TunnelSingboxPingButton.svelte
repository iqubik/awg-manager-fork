<script lang="ts">
	import { PingButton } from '$lib/components/ui';
	import type { SingboxDelayState } from '$lib/utils/singboxDelay';
	import type { SingboxLayoutMode } from '$lib/constants/singboxLayout';

	interface Props {
		layout?: SingboxLayoutMode | 'cards';
		label: string;
		state: SingboxDelayState;
		checking?: boolean;
		disabled?: boolean;
		onclick?: (e: MouseEvent) => void;
	}

	let {
		layout = 'compact',
		label,
		state,
		checking = false,
		disabled = false,
		onclick,
	}: Props = $props();

	const isList = $derived(layout === 'list');
	const isDense = $derived(layout === 'dense' || layout === 'cards');
</script>

<PingButton
	{label}
	{state}
	{checking}
	{disabled}
	size={isDense ? 'sm' : 'md'}
	forceBorder={isList}
	{onclick}
/>
