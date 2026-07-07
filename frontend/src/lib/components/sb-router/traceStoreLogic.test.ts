import { describe, expect, it } from 'vitest';
import { buildTraceUrl, readTraceUrlState } from './traceStoreLogic';

describe('traceStoreLogic', () => {
	it('reads closed default state from URL without trace params', () => {
		expect(readTraceUrlState('/routing?tab=singbox')).toEqual({
			open: false,
			domain: '',
		});
	});

	it('reads open state and query from URL', () => {
		expect(readTraceUrlState('/routing?tab=singbox&trace=1&q=netflix.com')).toEqual({
			open: true,
			domain: 'netflix.com',
		});
	});

	it('builds open trace URL with query', () => {
		expect(buildTraceUrl('/routing?tab=singbox', true, 'youtube.com')).toBe(
			'/routing?tab=singbox&trace=1&q=youtube.com',
		);
	});

	it('removes only trace params when closing and keeps unrelated params', () => {
		expect(
			buildTraceUrl('/routing?tab=singbox&trace=1&q=netflix.com&other=keep', false, ''),
		).toBe('/routing?tab=singbox&other=keep');
	});
});
