import type { SubscriptionMode, SubscriptionPreviewMember } from '$lib/types';

export type WizardKind = 'single' | 'inline' | 'url';

export interface AddTunnelWizardState {
	kind: WizardKind | 'choose';
	singleLinks: string;
	label: string;
	url: string;
	inlineText: string;
	headersText: string;
	refreshHoursStr: string;
	enabled: boolean;
	mode: SubscriptionMode;
	utUrl: string;
	utIntervalSec: number;
	utToleranceMs: number;
}

export function parseRefreshHours(value: string): number {
	return parseInt(value, 10) || 0;
}

export function isAddTunnelWizardDirty(
	state: AddTunnelWizardState,
	defaultHeadersText: string,
	defaultUrlTest: { url: string; intervalSec: number; toleranceMs: number },
): boolean {
	if (state.kind === 'choose') return false;
	if (state.kind === 'single') return state.singleLinks.trim() !== '';

	return (
		state.label.trim() !== '' ||
		state.url.trim() !== '' ||
		state.inlineText.trim() !== '' ||
		state.headersText !== defaultHeadersText ||
		state.refreshHoursStr !== '24' ||
		state.enabled !== true ||
		state.mode !== 'selector' ||
		state.utUrl !== defaultUrlTest.url ||
		state.utIntervalSec !== defaultUrlTest.intervalSec ||
		state.utToleranceMs !== defaultUrlTest.toleranceMs
	);
}

export function canRequestSubscriptionPreview(options: {
	previewing: boolean;
	mountingPreview: boolean;
	label: string;
	url: string;
}): boolean {
	return (
		!options.previewing &&
		!options.mountingPreview &&
		options.label.trim() !== '' &&
		options.url.trim() !== ''
	);
}

export function dedupePreviewMembers(
	members: SubscriptionPreviewMember[] | null | undefined,
): SubscriptionPreviewMember[] {
	const seen = new Set<string>();
	return (members ?? []).filter((member) => {
		if (seen.has(member.key)) return false;
		seen.add(member.key);
		return true;
	});
}

export function countSelectablePreviewMembers(members: SubscriptionPreviewMember[]): number {
	return members.filter((member) => member.supported !== false).length;
}

export function countExcludedSelectableMembers(
	members: SubscriptionPreviewMember[],
	excludedKeys: Set<string>,
): number {
	return members.filter((member) => member.supported !== false && excludedKeys.has(member.key)).length;
}

export function canCreateSubscriptionFromPreview(
	members: SubscriptionPreviewMember[],
	excludedKeys: Set<string>,
): boolean {
	return countSelectablePreviewMembers(members) > countExcludedSelectableMembers(members, excludedKeys);
}

export function buildExcludedSelectableKeys(members: SubscriptionPreviewMember[]): Set<string> {
	return new Set(
		members.filter((member) => member.supported !== false).map((member) => member.key),
	);
}
