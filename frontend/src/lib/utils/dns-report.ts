import type { DnsProxy, DnsProxyInfo, DnsRebind, DnsStaticRecord, DnsUpstream } from '$lib/types';
import { formatDateTimeWithOffset } from '$lib/utils/format';

function fmt(v: number | string | null | undefined): string {
	return v === null || v === undefined || v === '' ? '—' : String(v);
}

function fmtList(items: string[] | undefined | null): string {
	return items?.length ? items.join(', ') : '—';
}

function formatUpstream(u: DnsUpstream): string {
	const parts = [`${u.address}:${u.port}`, `[${u.encryption}]`];
	if (u.sni) parts.push(`SNI: ${u.sni}`);
	if (u.scope) parts.push(`scope: ${u.scope}`);
	const stats = [
		`sent ${fmt(u.rSent)}`,
		`rcvd ${fmt(u.aRcvd)}`,
		`nx ${fmt(u.nxRcvd)}`,
		`med ${fmt(u.medResp)}`,
		`avg ${fmt(u.avgResp)}`,
		`rank ${fmt(u.rank)}`,
	].join(', ');
	return `- ${parts.join(' · ')} | ${stats}`;
}

function formatStaticRecord(r: DnsStaticRecord): string {
	return `- ${r.host} ${r.type} ${r.value} (flag ${r.flag})`;
}

function formatRebind(rebind: DnsRebind): string {
	const lines = [
		`- enabled: ${rebind.enabled ? 'yes' : 'no'}`,
		`- nets: ${fmtList(rebind.nets)}`,
		`- excludes: ${fmtList(rebind.excludes)}`,
	];
	return lines.join('\n');
}

function formatProxy(proxy: DnsProxy, index: number): string {
	return [
		`Proxy ${index + 1}: ${proxy.displayName} (${proxy.name})`,
		`Ports: TCP ${proxy.tcpPort}, UDP ${proxy.udpPort}`,
		'Statistics:',
		`- Total requests: ${fmt(proxy.stat.totalRequests)}`,
		`- Requests sent upstream: ${fmt(proxy.stat.proxyRequestsSent)}`,
		`- Cache hit ratio: ${fmt(proxy.stat.cacheHitRatio)}%`,
		`- Cache hits: ${fmt(proxy.stat.cacheHits)}`,
		`- Memory: ${fmt(proxy.stat.memory)}`,
		proxy.upstreams.length > 0
			? ['Upstreams:', ...proxy.upstreams.map((u) => formatUpstream(u))]
			: ['Upstreams: —'],
		proxy.staticRecords.length > 0
			? ['Static records:', ...proxy.staticRecords.map((r) => formatStaticRecord(r))]
			: ['Static records: —'],
		['Rebind:', formatRebind(proxy.rebind)],
	].flat().join('\n');
}

export function formatDnsInfoReport(
	info: DnsProxyInfo,
	generatedAt: string,
	routerOffset?: number,
): string {
	const lines = [
		'AWG Manager DNS diagnostics',
		`Generated: ${formatDateTimeWithOffset(generatedAt, routerOffset)}`,
		`Proxies: ${info.proxies.length}`,
		'',
	];

	for (const [index, proxy] of info.proxies.entries()) {
		lines.push(formatProxy(proxy, index), '');
	}

	return lines.join('\n').trimEnd() + '\n';
}

export function dnsInfoReportFilename(generatedAt: string, routerOffset?: number): string {
	return `awg-manager-dns-info-${formatDateTimeWithOffset(generatedAt, routerOffset)
		.replace(' ', '-')
		.replace(/:/g, '-')}.txt`;
}
