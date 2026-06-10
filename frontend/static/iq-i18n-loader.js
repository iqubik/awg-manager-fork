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

	const script = document.createElement('script');
	script.defer = true;
	script.src = runtimeUrl;
	script.setAttribute('data-iq-i18n-runtime', '1');

	script.onerror = () => {
		if (window.localStorage?.getItem('iq_i18n_debug') === '1') {
			console.warn('[iq-i18n] proprietary runtime is not available in this build');
		}
	};

	document.head.appendChild(script);
})();
