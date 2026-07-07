export interface TraceUrlState {
	open: boolean;
	domain: string;
}

export function readTraceUrlState(href: string): TraceUrlState {
	const searchParams = new URL(href, 'http://localhost').searchParams;

	return {
		open: searchParams.get('trace') === '1',
		domain: searchParams.get('q') ?? '',
	};
}

export function buildTraceUrl(href: string, open: boolean, domain: string): string {
	const url = new URL(href, 'http://localhost');

	if (open) {
		url.searchParams.set('trace', '1');
		if (domain) {
			url.searchParams.set('q', domain);
		} else {
			url.searchParams.delete('q');
		}
	} else {
		url.searchParams.delete('trace');
		url.searchParams.delete('q');
	}

	return `${url.pathname}${url.search}${url.hash}`;
}
