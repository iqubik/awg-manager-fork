import { browser } from '$app/environment';
import { writable } from 'svelte/store';

export type SettingsSectionIconMode = 'none' | 'strict' | 'harmonious' | 'vivid';

export const SETTINGS_SECTION_ICON_MODE_LABELS: Record<SettingsSectionIconMode, string> = {
	none: 'Без иконок',
	strict: 'Строгая',
	harmonious: 'Гармоничная',
	vivid: 'Красочная',
};

const storageKey = 'awg-manager-settings-section-icon-mode';
const DEFAULT_MODE: SettingsSectionIconMode = 'harmonious';

function isValidMode(value: string | null): value is SettingsSectionIconMode {
	return value === 'none' || value === 'strict' || value === 'harmonious' || value === 'vivid';
}

function readStored(): SettingsSectionIconMode {
	if (!browser) return DEFAULT_MODE;
	try {
		const raw = localStorage.getItem(storageKey);
		return isValidMode(raw) ? raw : DEFAULT_MODE;
	} catch {
		return DEFAULT_MODE;
	}
}

function writeStored(mode: SettingsSectionIconMode): void {
	if (!browser) return;
	try {
		localStorage.setItem(storageKey, mode);
	} catch {
		/* ignore quota / private mode */
	}
}

function createSettingsSectionIconModeStore() {
	const { subscribe, set } = writable<SettingsSectionIconMode>(readStored());

	return {
		subscribe,
		init() {
			set(readStored());
		},
		setMode(mode: SettingsSectionIconMode) {
			set(mode);
			writeStored(mode);
		},
	};
}

export const settingsSectionIconMode = createSettingsSectionIconModeStore();
