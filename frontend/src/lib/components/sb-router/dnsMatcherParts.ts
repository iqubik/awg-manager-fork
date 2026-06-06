import type { SingboxRouterDNSRule } from '$lib/types';

export interface DnsMatcherPart {
	key: 'rule_set' | 'suffix' | 'domain' | 'keyword' | 'regex' | 'query_type';
	value: string;
}

function headWithExtra(items: string[], stripLeadingDot = false): string {
	let head = items[0] ?? '';
	if (stripLeadingDot) head = head.replace(/^\./, '');
	const rest = items.length > 1 ? ` +${items.length - 1}` : '';
	return `${head}${rest}`;
}

/** Matcher fragments for DNS rule compact rows (order matches edit modal). */
export function dnsMatcherParts(r: SingboxRouterDNSRule): DnsMatcherPart[] {
	const parts: DnsMatcherPart[] = [];
	if (r.rule_set?.length) parts.push({ key: 'rule_set', value: r.rule_set.join(', ') });
	if (r.domain_suffix?.length) {
		parts.push({ key: 'suffix', value: headWithExtra(r.domain_suffix, true) });
	}
	if (r.domain?.length) parts.push({ key: 'domain', value: headWithExtra(r.domain) });
	if (r.domain_keyword?.length) parts.push({ key: 'keyword', value: headWithExtra(r.domain_keyword) });
	if (r.domain_regex?.length) parts.push({ key: 'regex', value: headWithExtra(r.domain_regex) });
	if (r.query_type?.length) parts.push({ key: 'query_type', value: r.query_type.join(', ') });
	return parts;
}

export function dnsMatcherSummary(r: SingboxRouterDNSRule): string {
	const parts = dnsMatcherParts(r);
	if (parts.length === 0) return '—';
	return parts
		.map((p) => (p.key === 'query_type' ? `query_type=${p.value}` : `${p.key}: ${p.value}`))
		.join(' · ');
}
