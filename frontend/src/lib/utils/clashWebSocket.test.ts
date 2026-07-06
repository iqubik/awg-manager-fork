// @vitest-environment jsdom

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { createClashWS } from './clashWebSocket';

class MockWS {
	static instances: MockWS[] = [];
	url: string;
	onopen: ((ev: Event) => void) | null = null;
	onmessage: ((ev: MessageEvent) => void) | null = null;
	onerror: ((ev: Event) => void) | null = null;
	onclose: ((ev: CloseEvent) => void) | null = null;
	closed = false;
	constructor(url: string) {
		this.url = url;
		MockWS.instances.push(this);
	}
	close() {
		this.closed = true;
		this.onclose?.(new CloseEvent('close'));
	}
	emitOpen() { this.onopen?.(new Event('open')); }
	emitMessage(data: unknown) {
		this.onmessage?.(new MessageEvent('message', { data: JSON.stringify(data) }));
	}
	emitClose() { this.onclose?.(new CloseEvent('close')); }
}

beforeEach(() => {
	MockWS.instances = [];
	(globalThis as unknown as { WebSocket: typeof MockWS }).WebSocket = MockWS;
	vi.useFakeTimers();
});
afterEach(() => {
	vi.useRealTimers();
});

describe('createClashWS', () => {
	it('opens a WS to the rewritten URL and reports status transitions', () => {
		const onMsg = vi.fn();
		const onStatus = vi.fn();
		const close = createClashWS('/api/singbox/clash/connections', onMsg, onStatus);
		expect(MockWS.instances.length).toBe(1);
		expect(onStatus).toHaveBeenCalledWith('connecting');
		MockWS.instances[0].emitOpen();
		expect(onStatus).toHaveBeenCalledWith('open');
		close();
	});

	it('parses incoming messages as JSON', () => {
		const onMsg = vi.fn();
		const onStatus = vi.fn();
		createClashWS('/x', onMsg, onStatus);
		MockWS.instances[0].emitOpen();
		MockWS.instances[0].emitMessage({ hello: 'world' });
		expect(onMsg).toHaveBeenCalledWith({ hello: 'world' });
	});

	it('clean close (returned close called) does NOT reconnect', () => {
		const close = createClashWS('/x', vi.fn(), vi.fn());
		MockWS.instances[0].emitOpen();
		close();
		vi.advanceTimersByTime(60_000);
		expect(MockWS.instances.length).toBe(1);
	});

	it('unexpected close triggers backoff reconnect: 1s, 2s, 5s, then capped 10s', () => {
		const onStatus = vi.fn();
		createClashWS('/x', vi.fn(), onStatus);
		MockWS.instances[0].emitOpen();
		MockWS.instances[0].emitClose();
		expect(onStatus).toHaveBeenCalledWith('closed');

		vi.advanceTimersByTime(1000);
		expect(MockWS.instances.length).toBe(2);
		MockWS.instances[1].emitClose();

		vi.advanceTimersByTime(2000);
		expect(MockWS.instances.length).toBe(3);
		MockWS.instances[2].emitClose();

		vi.advanceTimersByTime(5000);
		expect(MockWS.instances.length).toBe(4);
		MockWS.instances[3].emitClose();

		vi.advanceTimersByTime(10_000);
		expect(MockWS.instances.length).toBe(5);
	});

	it('uses wss:// when location.protocol is https:', () => {
		const original = globalThis.location;
		Object.defineProperty(globalThis, 'location', {
			value: { protocol: 'https:', host: 'example.test' },
			configurable: true,
		});
		createClashWS('/api/x', vi.fn(), vi.fn());
		expect(MockWS.instances[0].url.startsWith('wss://')).toBe(true);
		Object.defineProperty(globalThis, 'location', { value: original, configurable: true });
	});
});
