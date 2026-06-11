import type { TunnelReferencedError } from '$lib/types';
import { outboundReferenced } from '$lib/stores/outboundReferenced';

export const OUTBOUND_REFERENCED_CODE = 'tunnel_referenced';

export type OutboundReferencedError = Error & { details: TunnelReferencedError };

export function isOutboundReferencedError(e: unknown): e is OutboundReferencedError {
	return e instanceof Error && e.message === OUTBOUND_REFERENCED_CODE && 'details' in e;
}

/** Opens the shared referenced-outbound modal; returns true when handled. */
export function showOutboundReferencedError(
	e: unknown,
	name: string,
	entityLabel: string,
): boolean {
	if (!isOutboundReferencedError(e)) return false;
	outboundReferenced.show(e.details, name, entityLabel);
	return true;
}
