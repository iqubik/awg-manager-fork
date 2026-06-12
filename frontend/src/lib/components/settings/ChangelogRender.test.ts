import { render, screen } from '@testing-library/svelte';
import { describe, expect, it, beforeEach } from 'vitest';
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

	it('renders items without details as static rows instead of buttons', () => {
		render(ChangelogRender, {
			props: { entries },
		});

		expect(screen.getByText('fix(ui): compact changelog modal').closest('button')).toBeNull();
	});
});
