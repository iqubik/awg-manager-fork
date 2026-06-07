import type { AwgValue } from '$lib/components/ui/VersionBadge.svelte';
import type { ASCParams, ASCParamsExtended } from '$lib/types';

const RANGE_PATTERN = /^\d+-\d+$/;

function isRange(value: string | undefined | null): boolean {
	const trimmed = (value ?? '').trim();
	return trimmed !== '' && RANGE_PATTERN.test(trimmed);
}

function hasAnySignaturePacket(params: ASCParams): boolean {
	const ext = params as ASCParamsExtended;
	return !!(ext.i1 || ext.i2 || ext.i3 || ext.i4 || ext.i5);
}

/** Mirrors backend config.ClassifyAWGVersion — AWG 2.0 → 1.5 → 1.0 → WG. */
export function classifyAwgVersionFromAsc(params: ASCParams | null | undefined): AwgValue {
	if (!params) return 'wg';

	if (isRange(params.h1) || isRange(params.h2) || isRange(params.h3) || isRange(params.h4)) {
		return 'awg2.0';
	}
	if (hasAnySignaturePacket(params)) {
		return 'awg1.5';
	}
	if (params.h1 && params.h2 && params.h3 && params.h4) {
		return 'awg1.0';
	}
	return 'wg';
}
