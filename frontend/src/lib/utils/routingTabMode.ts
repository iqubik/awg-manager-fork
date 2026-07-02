export function resolveRoutingMode(mode: string | null | undefined): 'tproxy' | 'fakeip-tun' {
	return mode === 'fakeip-tun' ? 'fakeip-tun' : 'tproxy';
}

export function isTProxyTabMuted(mode: string | null | undefined): boolean {
	return resolveRoutingMode(mode) === 'fakeip-tun';
}

export function isFakeIPTabMuted(mode: string | null | undefined): boolean {
	return resolveRoutingMode(mode) !== 'fakeip-tun';
}
