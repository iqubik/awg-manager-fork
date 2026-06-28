import { render, screen, waitFor } from '@testing-library/svelte';
import { describe, expect, it } from 'vitest';
import AwgConfigAnalyzer from './AwgConfigAnalyzer.svelte';

const SAMPLE_CONF = `[Interface]
PrivateKey = TEST_PRIVATE_KEY
Address = 10.0.0.2/32
DNS = 1.1.1.1

[Peer]
PublicKey = TEST_PUBLIC_KEY
PresharedKey = TEST_PSK
AllowedIPs = 0.0.0.0/0
Endpoint = demo.example.com:51820
PersistentKeepalive = 25
`;

describe('AwgConfigAnalyzer embedded readonly mode', () => {
	it('shows source label and keeps tunnel-save action hidden for peer modal embedding', async () => {
		render(AwgConfigAnalyzer, {
			props: {
				embedded: true,
				layoutMode: 'embedded',
				forceSingleColumn: true,
				initialRaw: SAMPLE_CONF,
				autoAnalyze: true,
				readonlySource: true,
				allowTunnelSave: false,
				sourceLabel: 'Клиент Demo peer',
			},
		});

		expect(screen.getByText('Источник конфига')).toBeTruthy();
		expect(screen.getByText('Клиент Demo peer')).toBeTruthy();
		expect(screen.queryByText('Текущий AWG-туннель')).toBeNull();
		expect(screen.queryByRole('button', { name: 'Записать в туннель' })).toBeNull();

	await waitFor(() => {
		expect(screen.getByText('Стандартный WireGuard')).toBeTruthy();
	});
	});
});
