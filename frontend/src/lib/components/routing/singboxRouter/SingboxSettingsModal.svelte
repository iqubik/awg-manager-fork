<script lang="ts">
	import type { Snippet } from 'svelte';
	import SideDrawer from '$lib/components/ui/SideDrawer.svelte';
	import './singboxSettingsForm.css';

	interface Props {
		title: string;
		onClose: () => void;
		/** По умолчанию true — для {#if}-монтирования; для bindable open передайте явно. */
		open?: boolean;
		size?: 'sm' | 'md' | 'lg' | 'xl' | 'wide';
		bodyLayout?: 'default' | 'fill';
		hasUnsavedChanges?: () => boolean;
		closeOnBackdrop?: boolean;
		children: Snippet;
		actions?: Snippet;
	}

	let {
		title,
		onClose,
		open = true,
		size = 'md',
		bodyLayout = 'default',
		hasUnsavedChanges,
		closeOnBackdrop,
		children,
		actions,
	}: Props = $props();

	const widthBySize = {
		sm: 420,
		md: 520,
		lg: 640,
		xl: 760,
		wide: 960,
	} as const;
</script>

<SideDrawer
	{open}
	onClose={onClose}
	{title}
	width={widthBySize[size]}
	bodyClass={bodyLayout === 'fill' ? 'drawer-body-fill' : ''}
	{hasUnsavedChanges}
	{closeOnBackdrop}
	footer={actions}
>
	<div class="sbr-settings-form">
		{@render children()}
	</div>
</SideDrawer>

<style>
	/*
	 * Shared UI widgets (Dropdown, ChipMultiSelect, SegmentedControl, editors)
	 * ship scoped styles with --color-bg-primary. Override here so every control
	 * in sing-box modals matches native inputs (--sbr-control-* on .sbr-settings-form).
	 */

	.sbr-settings-form :global(.field.full-width) {
		width: 100%;
		min-width: 0;
	}

	/* Dropdown: Type, Detour, Strategy, DNS server, update interval, outbound, … */
	.sbr-settings-form :global(.field .control .trigger) {
		background: var(--sbr-control-bg);
		border: 1px solid var(--sbr-control-border);
		border-radius: var(--sbr-control-radius);
		color: var(--sbr-control-color);
		font-size: var(--sbr-control-font-size);
		padding: var(--sbr-control-padding);
		box-sizing: border-box;
	}

	.sbr-settings-form :global(.field .control .trigger:hover:not(:disabled)) {
		border-color: var(--border-hover, var(--color-border-hover));
		background: var(--sbr-control-bg);
	}

	.sbr-settings-form :global(.field .control .trigger:focus-visible),
	.sbr-settings-form :global(.field .control .trigger.open) {
		border-color: var(--accent, var(--color-accent));
		outline: none;
	}

	.sbr-settings-form :global(.field .control .trigger-placeholder),
	.sbr-settings-form :global(.field .control .chevron) {
		color: var(--muted-text, var(--color-text-muted));
	}

	/* ChipMultiSelect: rule_set tags */
	.sbr-settings-form :global(.field .picker .chips) {
		background: var(--sbr-control-bg);
		border: 1px solid var(--sbr-control-border);
		border-radius: var(--sbr-control-radius);
		box-sizing: border-box;
		width: 100%;
	}

	/* GeoTagPicker search */
	.sbr-settings-form :global(.picker-search),
	.sbr-settings-form :global(.field .form-input) {
		background: var(--sbr-control-bg);
		border: 1px solid var(--sbr-control-border);
		border-radius: var(--sbr-control-radius);
		color: var(--sbr-control-color);
		font-size: var(--sbr-control-font-size);
		padding: var(--sbr-control-padding);
		box-sizing: border-box;
		width: 100%;
	}

	/* SegmentedControl: action, type, format, urltest/selector, … */
	.sbr-settings-form :global(.segmented-control) {
		background: var(--sbr-control-bg);
		border-color: var(--sbr-control-border);
	}

	/* JSON / list editors */
	.sbr-settings-form :global(.rules-json-editor),
	.sbr-settings-form :global(.rules-editor) {
		background: var(--sbr-control-bg);
		border: 1px solid var(--sbr-control-border);
		border-radius: var(--sbr-control-radius);
	}

	.sbr-settings-form :global(.rules-editor .line-numbers) {
		background: color-mix(in srgb, var(--sbr-control-bg) 85%, var(--sbr-control-border));
		border-color: var(--sbr-control-border);
	}
</style>
