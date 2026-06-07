import { writable } from 'svelte/store';

const STORAGE_KEY = 'awg-manager-develop-feedback-fab-visible';

function readInitial(): boolean {
	if (typeof localStorage === 'undefined') return true;
	return localStorage.getItem(STORAGE_KEY) !== '0';
}

function persist(value: boolean) {
	if (typeof localStorage === 'undefined') return;
	localStorage.setItem(STORAGE_KEY, value ? '1' : '0');
}

function createDevelopFeedbackFabVisible() {
	const { subscribe, set, update } = writable<boolean>(readInitial());

	return {
		subscribe,
		set(value: boolean) {
			persist(value);
			set(value);
		},
	};
}

/** Whether the develop-channel feedback FAB is shown (persisted in localStorage). */
export const developFeedbackFabVisible = createDevelopFeedbackFabVisible();
