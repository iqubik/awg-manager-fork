/**
 * Signature packet adapter — thin wrapper over AmneziaWG Architect generator.
 * @see https://github.com/Vadim-Khristenko/AmneziaWG-Architect
 */

import {
	genCfg,
	type GeneratorInput,
	type MimicProfile,
} from './generator';

export type ArchitectProtocolKey =
	| 'quic_initial'
	| 'quic_0rtt'
	| 'tls_client_hello'
	| 'wireguard_noise'
	| 'dtls'
	| 'http3'
	| 'sip'
	| 'dns_query';

export type SignaturePackets = {
	i1: string;
	i2: string;
	i3: string;
	i4: string;
	i5: string;
};

/** Legacy protocol keys used in ASCEditor dropdown. */
export type LegacyProtocolKey =
	| 'quic_initial'
	| 'quic_0rtt'
	| 'tls'
	| 'dtls'
	| 'http3'
	| 'sip'
	| 'wireguard_noise'
	| 'dns_query';

const LEGACY_TO_MIMIC: Record<LegacyProtocolKey, MimicProfile> = {
	quic_initial: 'quic_initial',
	quic_0rtt: 'quic_0rtt',
	tls: 'tls_client_hello',
	dtls: 'dtls',
	http3: 'http3',
	sip: 'sip',
	wireguard_noise: 'wireguard_noise',
	dns_query: 'dns_query',
};

function baseGeneratorInput(mtu: number): GeneratorInput {
	return {
		version: '2.0',
		intensity: 'medium',
		profile: 'quic_initial',
		customHost: '',
		mimicAll: false,
		// <c> ломает старые клиенты (ErrorCode 1000) — как в Architect по умолчанию.
		useTagC: false,
		useTagT: true,
		useTagR: true,
		useTagRC: true,
		useTagRD: true,
		useBrowserFp: false,
		browserProfile: '',
		mtu,
		junkLevel: 5,
		iterCount: 0,
		routerMode: false,
		useExtremeMax: false,
	};
}

/** Generate I1–I5 CPS signature chain for a mimicry profile. */
export function generateSignaturePackets(
	protocol: LegacyProtocolKey,
	mtu: number = 1280,
): SignaturePackets {
	const profile = LEGACY_TO_MIMIC[protocol];
	const cfg = genCfg({ ...baseGeneratorInput(mtu), profile });
	return {
		i1: cfg.i1,
		i2: cfg.i2,
		i3: cfg.i3,
		i4: cfg.i4,
		i5: cfg.i5,
	};
}
