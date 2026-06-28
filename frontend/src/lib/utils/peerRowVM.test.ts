import { describe, it, expect } from 'vitest';
import { stripHostMask, endpointHost, peerStatus, buildPeerRowVM, splitEndpoint, splitHandshake, STATUS_LABEL } from './peerRowVM';
import type { ManagedPeer, ManagedPeerStats } from '$lib/types';

describe('stripHostMask', () => {
	it('срезает только /32', () => {
		expect(stripHostMask('10.20.0.2/32')).toBe('10.20.0.2');
	});
	it('не трогает прочие маски и голый ip', () => {
		expect(stripHostMask('10.20.0.0/24')).toBe('10.20.0.0/24');
		expect(stripHostMask('10.20.0.2')).toBe('10.20.0.2');
	});
});

describe('endpointHost', () => {
	it('убирает порт у ipv4', () => {
		expect(endpointHost('78.108.40.214:51820')).toBe('78.108.40.214');
	});
	it('убирает порт у ipv6 в скобках', () => {
		expect(endpointHost('[2001:db8::1]:51820')).toBe('[2001:db8::1]');
	});
	it('пустой/прочерк → тире', () => {
		expect(endpointHost('-')).toBe('—');
		expect(endpointHost('')).toBe('—');
		expect(endpointHost(undefined)).toBe('—');
	});
	it('убирает порт у hostname', () => {
		expect(endpointHost('vpn.example.com:443')).toBe('vpn.example.com');
	});
	it('нечисловой порт оставляет как есть', () => {
		expect(endpointHost('host:abc')).toBe('host:abc');
	});
	it('голый ipv6 без порта оставляет как есть', () => {
		expect(endpointHost('2001:db8::1')).toBe('2001:db8::1');
	});
});

describe('splitEndpoint', () => {
	it('сохраняет полный endpoint и выносит ipv4 порт отдельно', () => {
		expect(splitEndpoint('78.108.40.214:51820')).toEqual({
			value: '78.108.40.214:51820',
			host: '78.108.40.214',
			port: ':51820',
		});
	});

	it('сохраняет bracketed ipv6 и порт отдельно', () => {
		expect(splitEndpoint('[2001:db8::1]:51820')).toEqual({
			value: '[2001:db8::1]:51820',
			host: '[2001:db8::1]',
			port: ':51820',
		});
	});

	it('не ломает копирование у голого ipv6 без порта', () => {
		expect(splitEndpoint('2001:db8::1')).toEqual({
			value: '2001:db8::1',
			host: '2001:db8::1',
		});
	});
});

describe('splitHandshake', () => {
	it('отрезает суффикс « назад»', () => {
		expect(splitHandshake('2 дня назад')).toEqual({ main: '2 дня', suffix: 'назад' });
	});
	it('без суффикса возвращает только main', () => {
		expect(splitHandshake('только что')).toEqual({ main: 'только что' });
	});
});

describe('peerStatus', () => {
	it('disabled важнее online', () => {
		expect(peerStatus(false, true)).toBe('disabled');
	});
	it('online/offline по флагу', () => {
		expect(peerStatus(true, true)).toBe('online');
		expect(peerStatus(true, false)).toBe('offline');
		expect(peerStatus(true, null)).toBe('offline');
	});
});

describe('buildPeerRowVM', () => {
	const peer: ManagedPeer = {
		publicKey: 'ABCDEFGH12345', privateKey: '', presharedKey: '',
		description: 'Mac', tunnelIP: '10.20.0.2/32', enabled: true,
	};
	const stats: ManagedPeerStats = {
		publicKey: 'ABCDEFGH12345', endpoint: '78.108.40.214:51820',
		rxBytes: 1024, txBytes: 2048, lastHandshake: '2026-06-13T12:00:00Z', online: true,
	};
	it('маппит поля, режет /32, убирает порт', () => {
		const vm = buildPeerRowVM(peer, stats);
		expect(vm.name).toBe('Mac');
		expect(vm.ip).toBe('10.20.0.2');
		expect(vm.endpoint).toBe('78.108.40.214:51820');
		expect(vm.endpointHost).toBe('78.108.40.214');
		expect(vm.endpointPort).toBe(':51820');
		expect(vm.status).toBe('online');
		expect(vm.enabled).toBe(true);
	});
	it('имя из ключа, если нет description', () => {
		const vm = buildPeerRowVM({ ...peer, description: '' }, undefined);
		expect(vm.name).toBe('ABCDEFGH...');
		expect(vm.endpointHost).toBe('—');
		expect(vm.status).toBe('offline');
	});
	it('STATUS_LABEL маппит в верхний регистр', () => {
		expect(STATUS_LABEL.online).toBe('ONLINE');
		expect(STATUS_LABEL.offline).toBe('OFFLINE');
		expect(STATUS_LABEL.disabled).toBe('OFF');
	});
});
