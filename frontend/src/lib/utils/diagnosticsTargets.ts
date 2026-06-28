import type { DiagnosticsTargetSeed } from '$lib/stores/diagnostics';
import type { SingboxTunnel, Subscription, TunnelListItem } from '$lib/types';

function singboxKind(protocol: string, security?: string): string {
	if (protocol === 'vless' && security === 'reality') return 'xray';
	if (protocol === 'vless') return 'vless';
	if (protocol === 'hysteria2') return 'hy2';
	if (protocol === 'naive') return 'ss';
	return protocol;
}

export function buildDiagnosticsTargets(
	awgTunnels: TunnelListItem[],
	singboxTunnels: SingboxTunnel[],
	subscriptions: Subscription[],
): DiagnosticsTargetSeed[] {
	const awg: DiagnosticsTargetSeed[] = awgTunnels.map((t) => ({
		id: t.id,
		name: t.name,
		status: t.status,
		kind: t.awgVersion ?? 'awg',
	}));

	const singbox: DiagnosticsTargetSeed[] = singboxTunnels.map((t) => ({
		id: `singbox:${t.tag}`,
		name: t.tag,
		status: t.running ? 'running' : 'stopped',
		kind: singboxKind(t.protocol, t.security),
	}));

	const subscriptionTargets: DiagnosticsTargetSeed[] = [];
	for (const sub of subscriptions) {
		if (!sub.enabled) continue;

		const memberTags = sub.memberTags ?? [];
		const activeTag =
			(sub.activeMember && memberTags.includes(sub.activeMember)
				? sub.activeMember
				: memberTags[0]) ?? '';

		const groupTag = sub.selectorTag?.trim() || '';
		const targetTag = groupTag || activeTag;
		if (!targetTag) continue;

		const activeMember = (sub.members ?? []).find((member) => member.tag === activeTag);
		const protocol = activeMember?.protocol;

		subscriptionTargets.push({
			id: `singbox:${targetTag}`,
			name: sub.label || groupTag || activeMember?.label || activeTag || targetTag,
			kind: protocol ? singboxKind(protocol, activeMember?.security) : undefined,
			status: 'running',
		});
	}

	const uniq = new Map<string, DiagnosticsTargetSeed>();
	for (const target of [...awg, ...subscriptionTargets, ...singbox]) {
		if (!uniq.has(target.id)) uniq.set(target.id, target);
	}

	return Array.from(uniq.values());
}
