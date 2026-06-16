import type { ChangelogEntry } from '$lib/types';

export const CHANGELOG_ACCORDION_STORAGE_KEY = 'awgm:changelog-accordion:v3';

export type ChangelogAccordionStorage = {
	schemaVersion: 3;
	openItemByKey: Record<string, boolean>;
	openVersionByKey: Record<string, boolean>;
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

export function changelogVersionKey(version: string): string {
	return `version::${version}`;
}

export function createInitialAccordionState(entries: ChangelogEntry[]): ChangelogAccordionStorage {
	const firstVersionKey = getFirstChangelogVersionKey(entries);
	const firstItemKey = getFirstChangelogItemKey(entries);

	return {
		schemaVersion: 3,
		openItemByKey: firstItemKey ? { [firstItemKey]: true } : {},
		openVersionByKey: firstVersionKey ? { [firstVersionKey]: true } : {},
	};
}

export function getFirstChangelogVersionKey(entries: ChangelogEntry[]): string | null {
	const first = entries[0];
	return first ? changelogVersionKey(first.version) : null;
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
	const initial = createInitialAccordionState(entries);
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
			schemaVersion: 3,
			openItemByKey: { ...parsed.openItemByKey },
			openVersionByKey: { ...parsed.openVersionByKey },
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
	return state.openItemByKey[itemKey] === true;
}

export function isChangelogVersionOpen(
	state: ChangelogAccordionStorage,
	versionKey: string,
): boolean {
	return state.openVersionByKey[versionKey] === true;
}

export function toggleChangelogAccordionState(
	state: ChangelogAccordionStorage,
	itemKey: string,
): ChangelogAccordionStorage {
	return {
		schemaVersion: 3,
		openItemByKey: {
			...state.openItemByKey,
			[itemKey]: !isChangelogAccordionOpen(state, itemKey),
		},
		openVersionByKey: { ...state.openVersionByKey },
	};
}

export function toggleChangelogVersionState(
	state: ChangelogAccordionStorage,
	versionKey: string,
): ChangelogAccordionStorage {
	return {
		schemaVersion: 3,
		openItemByKey: { ...state.openItemByKey },
		openVersionByKey: {
			...state.openVersionByKey,
			[versionKey]: !isChangelogVersionOpen(state, versionKey),
		},
	};
}

function isChangelogAccordionStorage(value: unknown): value is ChangelogAccordionStorage {
	if (!value || typeof value !== 'object') return false;

	const parsed = value as {
		schemaVersion?: unknown;
		openItemByKey?: unknown;
		openVersionByKey?: unknown;
	};

	if (parsed.schemaVersion !== 3) return false;
	if (!parsed.openItemByKey || typeof parsed.openItemByKey !== 'object') return false;
	if (!parsed.openVersionByKey || typeof parsed.openVersionByKey !== 'object') return false;

	return (
		Object.values(parsed.openItemByKey).every((item) => typeof item === 'boolean') &&
		Object.values(parsed.openVersionByKey).every((item) => typeof item === 'boolean')
	);
}
