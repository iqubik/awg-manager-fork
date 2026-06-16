import { fireEvent, render, screen } from '@testing-library/svelte';
import { beforeEach, describe, expect, it } from 'vitest';
import type { ChangelogEntry } from '$lib/types';
import ChangelogRender from './ChangelogRender.svelte';

const entries: ChangelogEntry[] = [
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
				items: [
					'feat(updater): better version labels\nКомментарий: старые версии по умолчанию должны быть свёрнуты.',
				],
			},
		],
	},
];

describe('ChangelogRender', () => {
	beforeEach(() => {
		localStorage.clear();
	});

	it('initializes accordion state when entries arrive after mount', async () => {
		const view = render(ChangelogRender, {
			props: { entries: [] },
		});

		expect(screen.queryByText('Комментарий: локальная dev-сборка теперь берёт base VERSION.')).toBeNull();

		await view.rerender({ entries });

		expect(await screen.findByText('Комментарий: локальная dev-сборка теперь берёт base VERSION.')).toBeTruthy();
	});

	it('collapses older versions by default', () => {
		render(ChangelogRender, {
			props: { entries },
		});

		expect(screen.getByText('Комментарий: локальная dev-сборка теперь берёт base VERSION.')).toBeTruthy();
		expect(screen.queryByText('Комментарий: старые версии по умолчанию должны быть свёрнуты.')).toBeNull();
	});

	it('reveals an older version after click', async () => {
		render(ChangelogRender, {
			props: { entries },
		});

		await fireEvent.click(screen.getByRole('button', { name: /2.12.3.9\+r9/i }));
		expect(await screen.findByText('feat(updater): better version labels')).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: /feat\(updater\): better version labels/i }));
		expect(await screen.findByText('Комментарий: старые версии по умолчанию должны быть свёрнуты.')).toBeTruthy();
	});

	it('renders items without details as static rows instead of buttons', () => {
		render(ChangelogRender, {
			props: { entries },
		});

		expect(screen.getByText('fix(ui): compact changelog modal').closest('button')).toBeNull();
	});
});
