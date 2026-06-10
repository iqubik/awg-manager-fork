import type { SingboxRouterDNSServer, SingboxRouterDNSRule } from '$lib/types';

export type DnsServerUsageInput = {
	tag: string;
	rules: readonly SingboxRouterDNSRule[];
	servers: readonly SingboxRouterDNSServer[];
	dnsFinal: string;
};

/** Mirrors backend dnsServerReferences for UI delete guards. */
export function collectDnsServerReferences(input: DnsServerUsageInput): string[] {
	const { tag, rules, servers, dnsFinal } = input;
	const refs: string[] = [];

	rules.forEach((r, i) => {
		if (r.server === tag) refs.push(`rule[${i}]`);
	});

	for (const s of servers) {
		if (s.tag === tag) continue;
		if (s.domain_resolver?.server === tag) {
			refs.push(`server[${s.tag}].domain_resolver`);
		}
	}

	if (dnsFinal === tag) refs.push('final');

	return refs;
}

export function dnsServerDeleteBlockReason(
	server: SingboxRouterDNSServer,
	input: Omit<DnsServerUsageInput, 'tag'>,
): string | null {
	const refs = collectDnsServerReferences({ ...input, tag: server.tag });
	if (refs.length === 0) return null;

	const preview = refs.slice(0, 3).join(', ');
	return refs.length > 3
		? `DNS-сервер используется (${preview}…)`
		: `DNS-сервер используется (${preview})`;
}
