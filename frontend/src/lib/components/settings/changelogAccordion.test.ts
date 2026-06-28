import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { ChangelogEntry } from '$lib/types';
import {
	CHANGELOG_ACCORDION_STORAGE_KEY,
	changelogItemKey,
	changelogVersionKey,
	createInitialAccordionState,
	getFirstChangelogVersionKey,
	isChangelogVersionOpen,
	readChangelogAccordionState,
	splitChangelogItem,
	toggleChangelogAccordionState,
	toggleChangelogVersionState,
} from './changelogAccordion';

const sampleEntries: ChangelogEntry[] = [
	{
		version: '2.12.3.10+r1',
		date: '2026-06-12',
		groups: [
			{
				heading: 'Исправлено',
				items: [
					'fix(dev): align local build version with VERSION file\nКомментарий: локальная dev-сборка теперь берёт base VERSION.',
					'fix(ui): compact changelog modal',
				],
			},
		],
	},
	{
		version: '2.12.3.9+r9',
		date: '2026-06-10',
		groups: [
			{
				heading: 'Добавлено',
				items: ['feat(updater): better version labels'],
			},
		],
	},
];

describe('changelogAccordion', () => {
	beforeEach(() => {
		localStorage.clear();
		vi.restoreAllMocks();
	});

	it('splits item into title and details', () => {
		expect(splitChangelogItem('fix(dev): subject\n  Комментарий: body text\n\n')).toEqual({
			title: 'fix(dev): subject',
			details: 'Комментарий: body text',
		});
	});

	it('opens only the first version and first item in the initial state', () => {
		expect(createInitialAccordionState(sampleEntries)).toEqual({
			schemaVersion: 3,
			openItemByKey: {
				[changelogItemKey('2.12.3.10+r1', 'Исправлено', 'fix(dev): align local build version with VERSION file')]: true,
			},
			openVersionByKey: {
				[changelogVersionKey('2.12.3.10+r1')]: true,
			},
		});
	});

	it('keeps older versions closed by default', () => {
		const state = createInitialAccordionState(sampleEntries);
		expect(isChangelogVersionOpen(state, changelogVersionKey('2.12.3.10+r1'))).toBe(true);
		expect(isChangelogVersionOpen(state, changelogVersionKey('2.12.3.9+r9'))).toBe(false);
	});

	it('persists initial v3 state when storage is empty', () => {
		const state = readChangelogAccordionState(sampleEntries);
		expect(JSON.parse(localStorage.getItem(CHANGELOG_ACCORDION_STORAGE_KEY) ?? 'null')).toEqual(state);
	});

	it('resets v2 storage to v3 initial state', () => {
		localStorage.setItem(
			CHANGELOG_ACCORDION_STORAGE_KEY,
			JSON.stringify({ schemaVersion: 2, openByKey: { broken: true } }),
		);

		const state = readChangelogAccordionState(sampleEntries);
		expect(state).toEqual(createInitialAccordionState(sampleEntries));
		expect(JSON.parse(localStorage.getItem(CHANGELOG_ACCORDION_STORAGE_KEY) ?? 'null')).toEqual(state);
	});

	it('toggles one item accordion key without depending on list indexes', () => {
		const itemKey = changelogItemKey(
			'2.12.3.10+r1',
			'Исправлено',
			'fix(ui): compact changelog modal',
		);
		const toggled = toggleChangelogAccordionState(createInitialAccordionState(sampleEntries), itemKey);

		expect(toggled.openItemByKey[itemKey]).toBe(true);
	});

	it('toggles an older version and persists state shape', () => {
		const versionKey = changelogVersionKey('2.12.3.9+r9');
		const toggled = toggleChangelogVersionState(createInitialAccordionState(sampleEntries), versionKey);
		expect(isChangelogVersionOpen(toggled, versionKey)).toBe(true);
		expect(toggled.openVersionByKey[changelogVersionKey('2.12.3.10+r1')]).toBe(true);
	});

	it('returns the first changelog version key', () => {
		expect(getFirstChangelogVersionKey(sampleEntries)).toBe(changelogVersionKey('2.12.3.10+r1'));
	});

	it('keeps persisted state and does not auto-open new versions', () => {
		const persistedItemKey = changelogItemKey(
			'2.12.3.9+r9',
			'Добавлено',
			'feat(updater): better version labels',
		);
		const persistedVersionKey = changelogVersionKey('2.12.3.9+r9');
		localStorage.setItem(
			CHANGELOG_ACCORDION_STORAGE_KEY,
			JSON.stringify({
				schemaVersion: 3,
				openItemByKey: { [persistedItemKey]: true },
				openVersionByKey: { [persistedVersionKey]: true },
			}),
		);

		expect(readChangelogAccordionState(sampleEntries)).toEqual({
			schemaVersion: 3,
			openItemByKey: { [persistedItemKey]: true },
			openVersionByKey: { [persistedVersionKey]: true },
		});
	});
});
