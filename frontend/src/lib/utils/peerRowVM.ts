import type { ManagedPeer, ManagedPeerStats } from '$lib/types';
import { formatBytes, formatRelativeTime } from '$lib/utils/format';

export type PeerStatus = 'online' | 'offline' | 'disabled';

export interface PeerRowVM {
	publicKey: string;
	name: string;
	enabled: boolean;
	status: PeerStatus;
	ip: string;
	endpoint: string;
	endpointHost: string;
	endpointPort?: string;
	rx: string;
	tx: string;
	handshake: { main: string; suffix?: string } | null;
}

/** Общий контракт пропсов для desktop-строки и mobile-карточки клиента. */
export interface PeerRowProps {
	peer: ManagedPeer;
	vm: PeerRowVM;
	showToggle: boolean;
	showDownload: boolean;
	showActions: boolean;
	toggling: boolean;
	onToggle: (peer: ManagedPeer) => void;
	onConf: (peer: ManagedPeer) => void;
	onEdit: (peer: ManagedPeer) => void;
	onDelete: (peer: ManagedPeer) => void;
	onCopy: (value: string, label: string) => void;
}

export const STATUS_LABEL: Record<PeerStatus, string> = {
	online: 'ONLINE',
	offline: 'OFFLINE',
	disabled: 'OFF',
};

/** Срезает только host-маску /32. Прочие маски и голый ip не трогает. */
export function stripHostMask(ip: string): string {
	return ip.endsWith('/32') ? ip.slice(0, -'/32'.length) : ip;
}

export function splitEndpoint(endpoint: string | undefined): {
	value: string;
	host: string;
	port?: string;
} {
	const trimmed = (endpoint ?? '').trim();
	if (!trimmed || trimmed === '-') return { value: '—', host: '—' };

	const bracketMatch = /^(\[[^\]]+\]):(\d+)$/.exec(trimmed);
	if (bracketMatch) return { value: trimmed, host: bracketMatch[1], port: `:${bracketMatch[2]}` };

	const lastColon = trimmed.lastIndexOf(':');
	if (lastColon <= 0) return { value: trimmed, host: trimmed };

	const host = trimmed.slice(0, lastColon);
	const port = trimmed.slice(lastColon + 1);

	if (!/^\d+$/.test(port) || host.includes(':')) {
		return { value: trimmed, host: trimmed };
	}

	return { value: trimmed, host, port: `:${port}` };
}

export function endpointHost(endpoint: string | undefined): string {
	return splitEndpoint(endpoint).host;
}

export function peerStatus(enabled: boolean, online: boolean | null | undefined): PeerStatus {
	if (!enabled) return 'disabled';
	return online ? 'online' : 'offline';
}

export function splitHandshake(value: string): { main: string; suffix?: string } {
	const t = value.trim();
	if (t.endsWith(' назад')) return { main: t.slice(0, -' назад'.length), suffix: 'назад' };
	return { main: t };
}

export function buildPeerRowVM(peer: ManagedPeer, stats: ManagedPeerStats | undefined): PeerRowVM {
	const endpoint = splitEndpoint(stats?.endpoint);

	return {
		publicKey: peer.publicKey,
		name: peer.description || `${peer.publicKey.slice(0, 8)}...`,
		enabled: peer.enabled,
		status: peerStatus(peer.enabled, stats?.online),
		ip: stripHostMask(peer.tunnelIP),
		endpoint: endpoint.value,
		endpointHost: endpoint.host,
		endpointPort: endpoint.port,
		rx: formatBytes(stats?.rxBytes ?? 0),
		tx: formatBytes(stats?.txBytes ?? 0),
		handshake: stats?.lastHandshake ? splitHandshake(formatRelativeTime(stats.lastHandshake)) : null,
	};
}
