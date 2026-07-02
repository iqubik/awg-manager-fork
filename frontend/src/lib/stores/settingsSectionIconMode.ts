import { createPersistedStore } from './persisted';

export type SettingsSectionIconMode = 'none' | 'strict' | 'harmonious' | 'vivid';

export const SETTINGS_SECTION_ICON_MODE_LABELS: Record<SettingsSectionIconMode, string> = {
	none: 'Без иконок',
	strict: 'Строгая',
	harmonious: 'Гармоничная',
	vivid: 'Красочная',
};

const DEFAULT_MODE: SettingsSectionIconMode = 'harmonious';

function isValidMode(value: string): value is SettingsSectionIconMode {
	return value === 'none' || value === 'strict' || value === 'harmonious' || value === 'vivid';
}

const store = createPersistedStore<SettingsSectionIconMode>('awg-manager-settings-section-icon-mode', {
	defaultValue: DEFAULT_MODE,
	deserialize: (raw) => (isValidMode(raw) ? raw : DEFAULT_MODE),
	serialize: (mode) => mode,
});

export const settingsSectionIconMode = {
	subscribe: store.subscribe,
	init: store.init,
	setMode: store.set,
};
