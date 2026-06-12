import type { ChangelogEntry } from '$lib/types';

export const CHANGELOG_ACCORDION_STORAGE_KEY = 'awgm:changelog-accordion:v2';

export type ChangelogAccordionStorage = {
	schemaVersion: 2;
	openByKey: Record<string, boolean>;
};

export function splitChangelogItem(item: string): { title: string; details: string } {
	const lines = item
		.split('\n')
		.map((line) => line.trim())
		.filter(Boolean);

	return {
		title: lines[0] ?? '',
		details: lines.slice(1).join('\n'),
	};
}

export function changelogItemKey(version: string, groupTitle: string, title: string): string {
	return `${version}::${groupTitle}::${title}`;
}

export function createInitialAccordionState(firstKey: string | null): ChangelogAccordionStorage {
	return {
		schemaVersion: 2,
		openByKey: firstKey ? { [firstKey]: true } : {},
	};
}

export function getFirstChangelogItemKey(entries: ChangelogEntry[]): string | null {
	for (const entry of entries) {
		for (const group of entry.groups) {
			if (!group.heading) continue;
			for (const item of group.items) {
				const { title } = splitChangelogItem(item);
				if (!title) continue;
				return changelogItemKey(entry.version, group.heading, title);
			}
		}
	}
	return null;
}

export function readChangelogAccordionState(entries: ChangelogEntry[]): ChangelogAccordionStorage {
	const initial = createInitialAccordionState(getFirstChangelogItemKey(entries));
	if (typeof window === 'undefined') return initial;

	try {
		const raw = window.localStorage.getItem(CHANGELOG_ACCORDION_STORAGE_KEY);
		if (!raw) {
			persistChangelogAccordionState(initial);
			return initial;
		}

		const parsed: unknown = JSON.parse(raw);
		if (!isChangelogAccordionStorage(parsed)) {
			persistChangelogAccordionState(initial);
			return initial;
		}

		return {
			schemaVersion: 2,
			openByKey: { ...parsed.openByKey },
		};
	} catch {
		persistChangelogAccordionState(initial);
		return initial;
	}
}

export function persistChangelogAccordionState(state: ChangelogAccordionStorage): void {
	if (typeof window === 'undefined') return;
	try {
		window.localStorage.setItem(CHANGELOG_ACCORDION_STORAGE_KEY, JSON.stringify(state));
	} catch {
		// localStorage can be unavailable; keep the in-memory state.
	}
}

export function isChangelogAccordionOpen(
	state: ChangelogAccordionStorage,
	itemKey: string,
): boolean {
	return state.openByKey[itemKey] === true;
}

export function toggleChangelogAccordionState(
	state: ChangelogAccordionStorage,
	itemKey: string,
): ChangelogAccordionStorage {
	return {
		schemaVersion: 2,
		openByKey: {
			...state.openByKey,
			[itemKey]: !isChangelogAccordionOpen(state, itemKey),
		},
	};
}

function isChangelogAccordionStorage(value: unknown): value is ChangelogAccordionStorage {
	if (!value || typeof value !== 'object') return false;

	const parsed = value as {
		schemaVersion?: unknown;
		openByKey?: unknown;
	};

	if (parsed.schemaVersion !== 2) return false;
	if (!parsed.openByKey || typeof parsed.openByKey !== 'object') return false;

	return Object.values(parsed.openByKey).every((item) => typeof item === 'boolean');
}
