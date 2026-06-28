(() => {
	'use strict';

	const currentScript = document.currentScript;
	const runtimeUrl =
		currentScript?.dataset?.iqI18nRuntime ||
		'/iq-i18n-runtime.min.js';

	if (!runtimeUrl || typeof runtimeUrl !== 'string') {
		return;
	}

	if (document.querySelector('script[data-iq-i18n-runtime="1"]')) {
		return;
	}

	const debug = () => {
		try {
			return window.localStorage?.getItem('iq_i18n_debug') === '1';
		} catch {
			return false;
		}
	};

	const isHtmlFallback = (contentType, source) => {
		const type = String(contentType || '').toLowerCase();
		const head = String(source || '').trimStart().slice(0, 64).toLowerCase();
		return (
			type.includes('text/html') ||
			head.startsWith('<!doctype') ||
			head.startsWith('<html') ||
			head.startsWith('<')
		);
	};

	(async () => {
		try {
			const response = await fetch(runtimeUrl, {
				cache: 'no-store',
				headers: { Accept: 'application/javascript,text/javascript,*/*' },
			});

			if (!response.ok) {
				if (debug()) {
					console.warn('[iq-i18n] runtime is not available:', response.status);
				}
				return;
			}

			const contentType = response.headers.get('content-type') || '';
			const source = await response.text();

			if (!source.trim() || isHtmlFallback(contentType, source)) {
				if (debug()) {
					console.warn('[iq-i18n] skipped non-JS runtime response');
				}
				return;
			}

			const script = document.createElement('script');
			script.defer = true;
			script.text = `${source}\n//# sourceURL=${runtimeUrl}`;
			script.setAttribute('data-iq-i18n-runtime', '1');
			document.head.appendChild(script);
		} catch (error) {
			if (debug()) {
				console.warn('[iq-i18n] failed to load optional runtime', error);
			}
		}
	})();
})();
