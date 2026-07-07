export function buildAddWizardUrl(href: string, open: boolean): string {
	const url = new URL(href, 'http://localhost');

	url.searchParams.set('tab', 'singbox');

	if (open) {
		url.searchParams.set('add', '1');
		url.searchParams.delete('trace');
		url.searchParams.delete('q');
	} else {
		url.searchParams.delete('add');
		url.searchParams.delete('edit');
	}

	return `${url.pathname}${url.search}${url.hash}`;
}
