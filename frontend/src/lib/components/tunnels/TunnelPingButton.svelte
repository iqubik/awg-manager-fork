<script lang="ts">
	import { PingButton } from '$lib/components/ui';
	import type { SingboxLayoutMode } from '$lib/constants/singboxLayout';

	type ConnState = 'idle' | 'connected' | 'disconnected' | 'checking';
	type AwgLayout = SingboxLayoutMode | 'cards';

	interface Props {
		layout?: AwgLayout;
		connectivity?: ConnState;
		latencyMs?: number | null;
		statusNote?: string;
		statusNoteTone?: 'recovering' | 'transitional';
		checking?: boolean;
		disabled?: boolean;
		onclick?: (e: MouseEvent) => void;
	}

	let {
		layout = 'compact',
		connectivity,
		latencyMs = null,
		statusNote,
		statusNoteTone = 'transitional',
		checking = false,
		disabled = false,
		onclick,
	}: Props = $props();

	const isList = $derived(layout === 'list');
	const isDense = $derived(layout === 'dense' || layout === 'cards');
</script>

<PingButton
	{connectivity}
	{latencyMs}
	{statusNote}
	{statusNoteTone}
	{checking}
	{disabled}
	size={isDense ? 'sm' : 'md'}
	forceBorder={isList}
	{onclick}
/>
