import { describe, expect, it } from 'vitest';
import { formatDnsInfoReport, dnsInfoReportFilename } from './dns-report';
import type { DnsProxyInfo } from '$lib/types';

describe('dns-report', () => {
	it('formats a readable text report', () => {
		const info: DnsProxyInfo = {
			proxies: [
				{
					name: 'dns-main',
					displayName: 'Main DNS',
					tcpPort: 53,
					udpPort: 53,
					stat: {
						totalRequests: 123,
						proxyRequestsSent: 45,
						cacheHitRatio: 66.7,
						cacheHits: 82,
						memory: '12 MiB',
					},
					upstreams: [
						{
							address: '8.8.8.8',
							port: 853,
							encryption: 'DoT',
							sni: 'dns.google',
							scope: 'all',
							rSent: 10,
							aRcvd: 9,
							nxRcvd: 1,
							medResp: '12ms',
							avgResp: '15ms',
							rank: 1,
						},
					],
					staticRecords: [
						{ host: 'example.com', type: 'A', value: '1.2.3.4', flag: 1 },
					],
					rebind: {
						enabled: true,
						nets: ['192.168.0.0/16'],
						excludes: ['10.0.0.0/8'],
					},
				},
			],
		};

		const report = formatDnsInfoReport(info, '2026-06-03T12:00:00Z', 180);

		expect(report).toContain('AWG Manager DNS diagnostics');
		expect(report).toContain('Generated: 2026-06-03 15:00:00+03:00');
		expect(report).toContain('Proxy 1: Main DNS (dns-main)');
		expect(report).toContain('Upstreams:');
		expect(report).toContain('8.8.8.8:853');
		expect(report).toContain('Static records:');
		expect(report).toContain('example.com A 1.2.3.4');
		expect(report).toContain('Rebind:');
		expect(report).toContain('enabled: yes');
	});

	it('builds a safe filename', () => {
		expect(dnsInfoReportFilename('2026-06-03T12:00:00Z', 180)).toBe(
			'awg-manager-dns-info-2026-06-03-15-00-00+03-00.txt',
		);
	});
});
