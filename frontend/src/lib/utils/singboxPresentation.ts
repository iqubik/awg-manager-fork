export function getSingboxProtocolLabel(protocol?: string): string {
	switch (protocol) {
		case 'vless':
			return 'VLESS';
		case 'hysteria2':
			return 'Hysteria2';
		case 'trojan':
			return 'Trojan';
		case 'shadowsocks':
			return 'Shadowsocks';
		case 'naive':
			return 'Naive';
		case 'mieru':
			return 'Mieru';
		default:
			return protocol ? protocol.charAt(0).toUpperCase() + protocol.slice(1) : '—';
	}
}
