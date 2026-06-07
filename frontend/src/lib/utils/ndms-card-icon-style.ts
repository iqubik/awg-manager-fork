import type { SettingsSectionIconMode } from '$lib/stores/settingsSectionIconMode';
import { DEFAULT_ICON_TILE_BG } from '$lib/utils/icon-tile-background';

export type NdmsCardIconStyle = {
	background: string;
	foreground: string;
};

/** Globe fallback in routing lists (letter icons off, no brand art). */
export function resolveNeutralServiceIconStyle(
	mode: SettingsSectionIconMode,
): NdmsCardIconStyle {
	return resolveNdmsCardIconStyle(mode, '#22c55e');
}

export function resolveNdmsCardIconStyle(
	mode: SettingsSectionIconMode,
	accentColor: string,
): NdmsCardIconStyle {
	switch (mode) {
		case 'strict':
			return {
				background: 'transparent',
				foreground: 'var(--color-text-secondary, var(--text-muted))',
			};
		case 'vivid':
			return {
				background: accentColor,
				foreground: '#fff',
			};
		case 'harmonious':
		default:
			return {
				background: DEFAULT_ICON_TILE_BG,
				foreground: 'var(--color-text-secondary, var(--text-muted))',
			};
	}
}
