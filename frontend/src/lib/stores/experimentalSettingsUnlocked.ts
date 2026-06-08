import { writable } from 'svelte/store';

const STORAGE_KEY = 'awg-manager-experimental-settings-unlocked';

function readInitial(): boolean {
	if (typeof localStorage === 'undefined') return false;
	return localStorage.getItem(STORAGE_KEY) === '1';
}

function persist(value: boolean) {
	if (typeof localStorage === 'undefined') return;
	localStorage.setItem(STORAGE_KEY, value ? '1' : '0');
}

function createExperimentalSettingsUnlocked() {
	const { subscribe, set, update } = writable<boolean>(readInitial());

	return {
		subscribe,
		toggle() {
			update((current) => {
				const next = !current;
				persist(next);
				return next;
			});
		},
		set(value: boolean) {
			persist(value);
			set(value);
		},
	};
}

/** Hidden «Экспериментальное» block in settings — toggled via version-badge easter egg. */
export const experimentalSettingsUnlocked = createExperimentalSettingsUnlocked();
