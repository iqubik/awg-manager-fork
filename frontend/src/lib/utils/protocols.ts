// ---------------------------------------------------------------------------
// AWG Signature Packet Generators
//
// CPS I1–I5 generation is delegated to AmneziaWG Architect (MIT):
// https://github.com/Vadim-Khristenko/AmneziaWG-Architect
//
// Total of all I1-I5 fields must stay under 4096 bytes.
// ---------------------------------------------------------------------------

import {
	generateSignaturePackets,
	type LegacyProtocolKey,
	type SignaturePackets,
} from '$lib/utils/awg-architect/signature';

export type ProtocolKey = LegacyProtocolKey;

export const protocols: Record<ProtocolKey, { name: string; description: string }> = {
	quic_initial: { name: 'QUIC Initial', description: 'RFC 9000 — основной протокол для обхода DPI' },
	quic_0rtt: { name: 'QUIC 0-RTT', description: 'Early Data — возобновление сессии' },
	tls: { name: 'TLS 1.3', description: 'Client Hello — HTTPS handshake' },
	dtls: { name: 'DTLS 1.3', description: 'WebRTC, VoIP — медиа-трафик' },
	http3: { name: 'HTTP/3', description: 'QUIC с расширенными типами пакетов' },
	sip: { name: 'SIP', description: 'VoIP сигнализация — REGISTER' },
	wireguard_noise: { name: 'Noise_IK', description: 'WireGuard handshake — нативный силуэт' },
	dns_query: { name: 'DNS Query', description: 'UDP DNS-запрос (A/AAAA)' },
};

export type { SignaturePackets };

const CPS_TAG_RE = /<(\w+)(?:\s+([^>]*))?>/g;

/**
 * Calculate byte size of a CPS pattern (I1–I5).
 * Counts payload inside <b>, <r>, <rc>, <rd> and fixed 4-byte <c>/<t> tags.
 */
export function calcByteSize(pattern: string): number {
	if (!pattern) return 0;

	let total = 0;
	let m: RegExpExecArray | null;
	CPS_TAG_RE.lastIndex = 0;

	while ((m = CPS_TAG_RE.exec(pattern)) !== null) {
		const tag = m[1].toLowerCase();
		const arg = (m[2] ?? '').trim();

		if (tag === 'b') {
			const hexMatch = arg.match(/0x([0-9a-fA-F]*)/);
			if (hexMatch) total += hexMatch[1].length / 2;
		} else if (tag === 'r' || tag === 'rc' || tag === 'rd') {
			const n = Number.parseInt(arg, 10);
			if (Number.isFinite(n) && n > 0) total += n;
		} else if (tag === 'c' || tag === 't') {
			total += 4;
		}
	}

	return total;
}

/** Calculate total byte size across all I1-I5. */
export function calcTotalSize(packets: SignaturePackets): number {
	return (
		calcByteSize(packets.i1) +
		calcByteSize(packets.i2) +
		calcByteSize(packets.i3) +
		calcByteSize(packets.i4) +
		calcByteSize(packets.i5)
	);
}

export function getSignaturePackets(protocol: ProtocolKey, mtu: number = 1280): SignaturePackets {
	return generateSignaturePackets(protocol, mtu);
}
