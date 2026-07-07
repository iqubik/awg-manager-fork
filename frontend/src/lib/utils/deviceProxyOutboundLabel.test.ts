import { describe, expect, it } from 'vitest';
import {
	outboundDegradedText,
	outboundName,
	outboundNowTag,
} from './deviceProxyOutboundLabel';
import type { DeviceProxyRuntime } from '$lib/types';

function rt(partial: Partial<DeviceProxyRuntime> = {}): DeviceProxyRuntime {
	return { alive: false, activeTag: '', defaultTag: '', ...partial };
}

describe('outboundName', () => {
	it('конфиг первичен — runtime.activeTag не влияет на имя', () => {
		const input = {
			selectedOutbound: 'vpn',
			runtime: rt({ alive: true, activeTag: 'awg-awg10', defaultTag: 'awg-awg10' }),
		};
		expect(outboundName(input)).toBe('vpn');
	});

	it('без selectedOutbound берёт runtime.defaultTag', () => {
		expect(outboundName({ selectedOutbound: '', runtime: rt({ defaultTag: 'proxy-a' }) })).toBe(
			'proxy-a',
		);
	});

	it('без данных — «—»', () => {
		expect(outboundName({ selectedOutbound: '', runtime: null })).toBe('—');
		expect(outboundName({ selectedOutbound: '' })).toBe('—');
	});
});

describe('outboundNowTag', () => {
	it('показывает activeTag когда движок жив и тег отличается от имени', () => {
		const input = {
			selectedOutbound: 'vpn',
			runtime: rt({ alive: true, activeTag: 'awg-awg10' }),
		};
		expect(outboundNowTag(input)).toBe('awg-awg10');
	});

	it('прячет пометку когда activeTag совпадает с именем', () => {
		const input = { selectedOutbound: 'vpn', runtime: rt({ alive: true, activeTag: 'vpn' }) };
		expect(outboundNowTag(input)).toBeNull();
	});

	it('прячет пометку когда движок не жив (activeTag устарел)', () => {
		const input = {
			selectedOutbound: 'vpn',
			runtime: rt({ alive: false, activeTag: 'awg-awg10' }),
		};
		expect(outboundNowTag(input)).toBeNull();
	});

	it('нет runtime — нет пометки', () => {
		expect(outboundNowTag({ selectedOutbound: 'vpn' })).toBeNull();
	});
});

describe('outboundDegradedText', () => {
	it('деградация: показывает fallback', () => {
		const input = {
			selectedOutbound: 'vpn',
			runtime: rt({ degradedOutbound: 'vpn', fallbackTag: 'awg-awg10' }),
		};
		expect(outboundDegradedText(input)).toBe('выход недоступен — через awg-awg10');
	});

	it('деградация без fallbackTag — direct', () => {
		const input = { selectedOutbound: 'vpn', runtime: rt({ degradedOutbound: 'vpn' }) };
		expect(outboundDegradedText(input)).toBe('выход недоступен — через direct');
	});

	it('нет деградации — null', () => {
		expect(outboundDegradedText({ selectedOutbound: 'vpn', runtime: rt() })).toBeNull();
		expect(outboundDegradedText({ selectedOutbound: 'vpn' })).toBeNull();
	});
});

describe('degradation vs drift semantics', () => {
	it('в degraded-сценарии основным остаётся configured label, fallback идёт отдельным текстом', () => {
		const input = {
			selectedOutbound: 'vpn',
			runtime: rt({
				alive: true,
				activeTag: 'awg-awg10',
				defaultTag: 'awg-awg10',
				degradedOutbound: 'vpn',
				fallbackTag: 'awg-awg10',
			}),
		};
		expect(outboundName(input)).toBe('vpn');
		expect(outboundDegradedText(input)).toBe('выход недоступен — через awg-awg10');
		expect(outboundNowTag(input)).toBe('awg-awg10');
	});

	it('при обычном drift label остаётся configured/default, active показывается отдельно', () => {
		const input = {
			selectedOutbound: 'vpn',
			runtime: rt({
				alive: true,
				defaultTag: 'vpn',
				activeTag: 'proxy-a',
			}),
		};
		expect(outboundName(input)).toBe('vpn');
		expect(outboundNowTag(input)).toBe('proxy-a');
		expect(outboundDegradedText(input)).toBeNull();
	});
});
