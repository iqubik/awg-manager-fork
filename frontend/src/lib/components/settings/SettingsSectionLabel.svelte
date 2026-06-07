<script lang="ts" module>
	import { Router as LucideIconRef } from 'lucide-svelte';

	export type SettingsSectionTone =
		| 'blue'
		| 'green'
		| 'purple'
		| 'orange'
		| 'pink'
		| 'teal'
		| 'indigo'
		| 'red'
		| 'slate';

	export type SettingsSectionIcon = typeof LucideIconRef;

	/** Palette for tone prop — used in vivid icon mode. */
	export const SETTINGS_SECTION_TONE_COLORS: Record<SettingsSectionTone, string> = {
		blue: '#0077ff',
		green: '#00a650',
		purple: '#8b5cf6',
		orange: '#ff8a00',
		pink: '#ff4d7e',
		teal: '#00acc1',
		indigo: '#5c6bc0',
		red: '#ff5252',
		slate: '#78909c',
	};
</script>

<script lang="ts">
	import { settingsSectionIconMode } from '$lib/stores/settingsSectionIconMode';

	interface Props {
		label: string;
		icon: SettingsSectionIcon;
		tone?: SettingsSectionTone;
		/** Card header row with spacing before content. */
		header?: boolean;
		/** Compact row for collapse buttons and toolbars. */
		inline?: boolean;
	}

	let { label, icon: Icon, tone = 'blue', header = false, inline = false }: Props = $props();

	const iconMode = $derived($settingsSectionIconMode);
	const toneColor = $derived(SETTINGS_SECTION_TONE_COLORS[tone]);
</script>

<div class="settings-section-label" class:header class:inline style:--tone-color={toneColor}>
	<span
		class="icon-badge"
		class:mode-strict={iconMode === 'strict'}
		class:mode-harmonious={iconMode === 'harmonious'}
		class:mode-vivid={iconMode === 'vivid'}
		data-tone={tone}
		aria-hidden="true"
	>
		{#key iconMode}
			<Icon size={18} strokeWidth={2.25} color="currentColor" />
		{/key}
	</span>
	<span class="label-wrap">
		<span class="label-text">{label}</span>
		<span class="label-divider" class:hidden={iconMode === 'strict'} aria-hidden="true"></span>
	</span>
</div>

<style>
	.settings-section-label {
		display: flex;
		align-items: center;
		gap: 0.75rem;
		min-width: 0;
		--section-icon-surface: var(--color-settings-control-bg, var(--color-bg-tertiary, var(--bg-secondary)));
	}

	.settings-section-label.header {
		margin-bottom: 0.5rem;
	}

	.icon-badge {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 2rem;
		height: 2rem;
		border-radius: 0.625rem;
		flex-shrink: 0;
		margin-top: 0.0625rem;
	}

	.icon-badge.mode-strict {
		color: var(--color-text-secondary, var(--text-muted));
		background: transparent;
	}

	.icon-badge.mode-harmonious {
		color: var(--color-text-secondary, var(--text-muted));
		background: var(--section-icon-surface);
	}

	.icon-badge.mode-vivid {
		color: #fff;
		background: var(--tone-color);
	}

	.label-wrap {
		display: inline-flex;
		flex-direction: column;
		align-items: stretch;
		gap: 0.3125rem;
		min-width: 0;
		max-width: 100%;
	}

	.label-text {
		font-size: 0.9375rem;
		font-weight: 600;
		line-height: 1.25;
		color: var(--text, var(--color-text));
		min-width: 0;
	}

	.label-divider {
		width: 100%;
		height: 2px;
		border-radius: 1px;
	}

	.settings-section-label:has(.mode-harmonious) .label-divider {
		background: var(--section-icon-surface);
	}

	.label-divider.hidden {
		display: none;
	}

	.settings-section-label:has(.mode-vivid) .label-divider {
		background: var(--tone-color);
	}

	.settings-section-label.inline .label-text {
		font-size: 0.9375rem;
	}
</style>
