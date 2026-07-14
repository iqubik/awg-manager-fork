import { createPersistedFlag } from './persisted';

const store = createPersistedFlag('awg-manager-experimental-settings-unlocked', false);

/** Hidden «Экспериментальное» block in settings — toggled via version-badge easter egg. */
export const experimentalSettingsUnlocked = {
	subscribe: store.subscribe,
	set: store.set,
	toggle() {
		store.update((current) => !current);
	},
};
