import { describe, it, expect, beforeEach, vi } from 'vitest';
import type { ChangelogEntry } from '$lib/types';
import {
	CHANGELOG_ACCORDION_STORAGE_KEY,
	changelogItemKey,
	createInitialAccordionState,
	getFirstChangelogItemKey,
	readChangelogAccordionState,
	splitChangelogItem,
	toggleChangelogAccordionState,
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
		expect(
			splitChangelogItem('fix(dev): subject\n  Комментарий: body text\n\n'),
		).toEqual({
			title: 'fix(dev): subject',
			details: 'Комментарий: body text',
		});
	});

	it('opens only the first item in the initial state', () => {
		const firstKey = getFirstChangelogItemKey(sampleEntries);
		expect(createInitialAccordionState(firstKey)).toEqual({
			schemaVersion: 2,
			openByKey: {
				[changelogItemKey('2.12.3.10+r1', 'Исправлено', 'fix(dev): align local build version with VERSION file')]: true,
			},
		});
	});

	it('falls back to initial state when storage is empty', () => {
		expect(readChangelogAccordionState(sampleEntries)).toEqual(
			createInitialAccordionState(getFirstChangelogItemKey(sampleEntries)),
		);
	});

	it('persists initial v2 state when storage is empty', () => {
		const state = readChangelogAccordionState(sampleEntries);
		expect(JSON.parse(localStorage.getItem(CHANGELOG_ACCORDION_STORAGE_KEY) ?? 'null')).toEqual(state);
	});

	it('falls back to initial state when storage schema is old', () => {
		localStorage.setItem(
			CHANGELOG_ACCORDION_STORAGE_KEY,
			JSON.stringify({ schemaVersion: 1, openByKey: { broken: true } }),
		);
		expect(readChangelogAccordionState(sampleEntries)).toEqual(
			createInitialAccordionState(getFirstChangelogItemKey(sampleEntries)),
		);
	});

	it('migrates old storage schema to persisted v2 state', () => {
		localStorage.setItem(
			CHANGELOG_ACCORDION_STORAGE_KEY,
			JSON.stringify({ schemaVersion: 1, openByKey: { broken: true } }),
		);

		const state = readChangelogAccordionState(sampleEntries);
		expect(JSON.parse(localStorage.getItem(CHANGELOG_ACCORDION_STORAGE_KEY) ?? 'null')).toEqual(state);
	});

	it('keeps persisted state and does not auto-open unknown items', () => {
		const persistedKey = changelogItemKey(
			'2.12.3.9+r9',
			'Добавлено',
			'feat(updater): better version labels',
		);
		localStorage.setItem(
			CHANGELOG_ACCORDION_STORAGE_KEY,
			JSON.stringify({
				schemaVersion: 2,
				openByKey: { [persistedKey]: true },
			}),
		);

		expect(readChangelogAccordionState(sampleEntries)).toEqual({
			schemaVersion: 2,
			openByKey: { [persistedKey]: true },
		});
	});

	it('toggles one accordion key without depending on list indexes', () => {
		const itemKey = changelogItemKey(
			'2.12.3.10+r1',
			'Исправлено',
			'fix(ui): compact changelog modal',
		);
		const toggled = toggleChangelogAccordionState(
			createInitialAccordionState(getFirstChangelogItemKey(sampleEntries)),
			itemKey,
		);

		expect(toggled.openByKey[itemKey]).toBe(true);
	});
});
