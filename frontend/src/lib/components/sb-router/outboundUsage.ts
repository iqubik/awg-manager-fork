import type {
	SingboxRouterDNSServer,
	SingboxRouterOutbound,
	SingboxRouterRule,
	SingboxRouterRuleSet,
} from '$lib/types';

export type OutboundUsageInput = {
	tag: string;
	rules: readonly SingboxRouterRule[];
	routeFinal: string;
	outbounds: readonly SingboxRouterOutbound[];
	dnsServers: readonly SingboxRouterDNSServer[];
	ruleSets: readonly SingboxRouterRuleSet[];
	deviceProxyOutbounds?: readonly string[];
};

function ruleReferencesOutbound(rule: SingboxRouterRule, tag: string): boolean {
	if (rule.outbound === tag) return true;
	for (const nested of rule.rules ?? []) {
		if (ruleReferencesOutbound(nested, tag)) return true;
	}
	return false;
}

/** Best-effort UI delete guards; API remains source of truth. Also blocks device-proxy usage (frontend-only). */
export function collectOutboundReferences(input: OutboundUsageInput): string[] {
	const { tag, rules, routeFinal, outbounds, dnsServers, ruleSets, deviceProxyOutbounds } = input;
	const refs: string[] = [];

	rules.forEach((r, i) => {
		if (ruleReferencesOutbound(r, tag)) refs.push(`route.rules[${i}]`);
	});

	if (routeFinal === tag) refs.push('route.final');

	for (const o of outbounds) {
		o.outbounds?.forEach((member, j) => {
			if (member === tag) refs.push(`outbounds[${o.tag}].members[${j}]`);
		});
		if (o.default === tag) refs.push(`outbounds[${o.tag}].default`);
	}

	for (const s of dnsServers) {
		if (s.detour === tag) refs.push(`dns.servers[${s.tag}].detour`);
	}

	for (const rs of ruleSets) {
		if (rs.download_detour === tag) refs.push(`route.rule_set[${rs.tag}].download_detour`);
	}

	if (deviceProxyOutbounds?.some((selected) => selected === tag)) {
		refs.push('device-proxy');
	}

	return refs;
}

export function outboundDeleteBlockReason(
	outbound: SingboxRouterOutbound,
	input: Omit<OutboundUsageInput, 'tag'>,
): string | null {
	if (outbound.source === 'subscription') {
		return 'Подписку можно удалить только в разделе «Подписки»';
	}

	const refs = collectOutboundReferences({ ...input, tag: outbound.tag });
	if (refs.length === 0) return null;

	const preview = refs.slice(0, 3).join(', ');
	return refs.length > 3
		? `Outbound используется (${preview}…)`
		: `Outbound используется (${preview})`;
}
