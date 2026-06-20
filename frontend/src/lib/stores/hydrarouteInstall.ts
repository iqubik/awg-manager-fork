import { writable } from 'svelte/store';
import type { HydraRouteInstallProgressEvent } from '$lib/api/events';

function createHydraRouteInstallStore() {
	const { subscribe, set } = writable<HydraRouteInstallProgressEvent | null>(null);
	let dropTimer: ReturnType<typeof setTimeout> | null = null;

	function clearDropTimer() {
		if (dropTimer) {
			clearTimeout(dropTimer);
			dropTimer = null;
		}
	}

	return {
		subscribe,
		ingest(ev: HydraRouteInstallProgressEvent) {
			clearDropTimer();
			set(ev);
			if (ev.phase === 'done' || ev.phase === 'error') {
				dropTimer = setTimeout(() => {
					set(null);
					dropTimer = null;
				}, 2000);
			}
		},
		clear() {
			clearDropTimer();
			set(null);
		},
	};
}

export const hydraRouteInstallProgress = createHydraRouteInstallStore();
