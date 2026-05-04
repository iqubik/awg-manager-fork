import { describe, it, expect } from 'vitest';
import { parseHeadersText, serializeHeaders } from './headersParser';

describe('parseHeadersText', () => {
	it('parses valid header lines', () => {
		const got = parseHeadersText('User-Agent: Happ\nX-Device-OS: iOS');
		expect(got).toEqual([
			{ name: 'User-Agent', value: 'Happ' },
			{ name: 'X-Device-OS', value: 'iOS' },
		]);
	});

	it('skips comments and empty lines', () => {
		const got = parseHeadersText('# comment\n\nUser-Agent: X');
		expect(got).toEqual([{ name: 'User-Agent', value: 'X' }]);
	});

	it('skips malformed lines', () => {
		const got = parseHeadersText('no-colon\nValid: y');
		expect(got).toEqual([{ name: 'Valid', value: 'y' }]);
	});

	it('roundtrips via serializeHeaders', () => {
		const orig = [
			{ name: 'A', value: '1' },
			{ name: 'B', value: '2' },
		];
		expect(parseHeadersText(serializeHeaders(orig))).toEqual(orig);
	});
});
