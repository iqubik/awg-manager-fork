<script lang="ts">
	import { Modal } from '$lib/components/ui';
	import TunnelDiagnosticsPanel from './TunnelDiagnosticsPanel.svelte';

	type DiagnosticsKind = 'awg' | 'singbox' | 'subscription';
	type DiagnosticsSubjectLabel = 'туннель' | 'подписку';

	interface Props {
		open: boolean;
		kind: DiagnosticsKind;
		targetId: string;
		displayName: string;
		subjectLabel: DiagnosticsSubjectLabel;
		iface?: string;
		loading?: boolean;
		unavailableReason?: string;
		onclose: () => void;
	}

	let {
		open,
		kind,
		targetId,
		displayName,
		subjectLabel,
		iface,
		loading = false,
		unavailableReason,
		onclose,
	}: Props = $props();

	let diagnosticsTitlePrefix = $derived.by(() => {
		if (kind === 'awg') return 'AWG';
		if (kind === 'singbox') return 'Sing-box';
		return 'Subscription';
	});
	let modalTitle = $derived(`${diagnosticsTitlePrefix} тестирование: ${displayName}`);
</script>

<Modal
	{open}
	{onclose}
	title={modalTitle}
	size="xl"
>
	<TunnelDiagnosticsPanel
		{kind}
		{targetId}
		{displayName}
		backHref=""
		backLabel=""
		{subjectLabel}
		{iface}
		{loading}
		{unavailableReason}
		mode="modal"
	/>
</Modal>
