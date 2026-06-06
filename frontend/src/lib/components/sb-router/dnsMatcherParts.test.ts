import { describe, it, expect } from 'vitest';
import { dnsMatcherParts, dnsMatcherSummary } from './dnsMatcherParts';

describe('dnsMatcherParts', () => {
	it('collects all matcher kinds in stable order', () => {
		expect(
			dnsMatcherParts({
				query_type: ['A', 'AAAA'],
				domain_regex: ['^ads\\.'],
				domain_keyword: ['tracker'],
				domain: ['example.com', 'foo.com'],
				domain_suffix: ['.youtube.com'],
				rule_set: ['geosite-netflix', 'ads'],
			}).map((p) => p.key),
		).toEqual(['rule_set', 'suffix', 'domain', 'keyword', 'regex', 'query_type']);
	});

	it('summary uses query_type= and colon for other keys', () => {
		expect(
			dnsMatcherSummary({
				rule_set: ['geosite-netflix'],
				domain_suffix: ['.yt.com', '.google.com'],
				query_type: ['HTTPS'],
			}),
		).toBe('rule_set: geosite-netflix · suffix: yt.com +1 · query_type=HTTPS');
	});

	it('empty rule → dash summary', () => {
		expect(dnsMatcherSummary({})).toBe('—');
		expect(dnsMatcherParts({})).toEqual([]);
	});
});
