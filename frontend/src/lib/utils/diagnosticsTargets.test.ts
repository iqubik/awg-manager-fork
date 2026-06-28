import { describe, expect, it } from 'vitest';
import { buildDiagnosticsTargets } from './diagnosticsTargets';
import type { SingboxTunnel, Subscription, TunnelListItem } from '$lib/types';

describe('buildDiagnosticsTargets', () => {
	it('prefers subscription selector target over raw sing-box tunnel for diagnostics', () => {
		const awg: TunnelListItem[] = [];
		const singbox: SingboxTunnel[] = [
			{
				tag: 'iq0',
				running: true,
				protocol: 'vless',
				security: 'reality',
				server: '',
				port: 0,
				transport: 'tcp',
				listenPort: 0,
				proxyInterface: '',
				connectivity: {
					connected: false,
					latency: null,
				},
				kernelInterface: '',
			},
		];
		const subscriptions: Subscription[] = [
			{
				id: 'sub-1',
				label: 'IQ subscription',
				url: '',
				isInline: false,
				headers: [],
				refreshHours: 24,
				lastFetched: '',
				selectorTag: 'iq0',
				inboundTag: 'sub-1-in',
				listenPort: 11000,
				proxyIndex: 0,
				memberTags: ['member-1'],
				members: [
					{
						tag: 'member-1',
						label: 'DE node',
						protocol: 'vless',
						server: 'example.com',
						port: 443,
						security: 'reality',
					},
				],
				orphanTags: [],
				activeMember: 'member-1',
				enabled: true,
				mode: 'selector',
			},
		];

		const targets = buildDiagnosticsTargets(awg, singbox, subscriptions);

		expect(targets).toHaveLength(1);
		expect(targets[0]).toMatchObject({
			id: 'singbox:iq0',
			name: 'IQ subscription',
			kind: 'xray',
			status: 'running',
		});
	});
});
