import { browser } from '$app/environment';
import { writable } from 'svelte/store';

const storageKey = 'awg-manager-service-letter-icons';

function clearStored(): void {
	if (!browser) return;
	try {
		localStorage.removeItem(storageKey);
	} catch {
		/* ignore */
	}
}

function createServiceLetterIconsStore() {
	const { subscribe, set } = writable<boolean>(true);

	return {
		subscribe,
		init() {
			clearStored();
			set(true);
		},
		setEnabled(_enabled: boolean) {
			clearStored();
			set(true);
		},
	};
}

/** User preference: colored monogram tiles when no custom / brand icon applies. */
export const serviceLetterIcons = createServiceLetterIconsStore();
