import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import { writable } from 'svelte/store';
import AddTunnelWizard from './AddTunnelWizard.svelte';

const { gotoMock, previewSubscriptionMock } = vi.hoisted(() => ({
	gotoMock: vi.fn(),
	previewSubscriptionMock: vi.fn(),
}));

vi.mock('$app/navigation', () => ({
	goto: gotoMock,
}));

vi.mock('$lib/api/client', () => ({
	api: {
		previewSubscription: previewSubscriptionMock,
		singboxImportLinks: vi.fn(),
		createSubscription: vi.fn(),
		singboxRouterListOutbounds: vi.fn(),
	},
}));

vi.mock('$lib/stores/singbox', () => ({
	singboxStatus: writable({ data: { installed: true } }),
	singboxTunnels: {
		applyMutationResponse: vi.fn(),
	},
}));

vi.mock('$lib/stores/subscriptions', () => ({
	subscriptionsStore: {
		refetch: vi.fn(),
	},
}));

vi.mock('$lib/stores/singboxRouter', () => ({
	singboxRouter: {
		applyOutbounds: vi.fn(),
	},
}));

describe('AddTunnelWizard', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		previewSubscriptionMock.mockResolvedValue([
			{
				key: 'demo-key',
				label: 'Demo node',
				protocol: 'vless',
				server: 'demo.example.com',
				port: 443,
				supported: true,
			},
		]);
		vi.spyOn(window, 'requestAnimationFrame').mockImplementation((cb: FrameRequestCallback) => {
			cb(0);
			return 1;
		});
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	it('resets loading and switches to preview after successful URL preview fetch', async () => {
		render(AddTunnelWizard, {
			props: { open: true, preselect: 'url', onclose: vi.fn() },
		});

		const inputs = await screen.findAllByRole('textbox');
		await fireEvent.input(inputs[0], { target: { value: 'Provider Demo' } });
		await fireEvent.input(inputs[1], { target: { value: 'https://example.com/sub.txt' } });

		const nextButton = screen.getByRole('button', { name: 'Далее' });
		await fireEvent.click(nextButton);

		await waitFor(() => {
			expect(previewSubscriptionMock).toHaveBeenCalledTimes(1);
		});

		expect(await screen.findByText('Выбор серверов')).toBeTruthy();
		expect(screen.getByText('Demo node')).toBeTruthy();
		expect(screen.queryByText('Загрузка...')).toBeNull();
	});

	it('keeps next button disabled while preview is mounting and prevents duplicate preview request', async () => {
		previewSubscriptionMock.mockResolvedValueOnce([
			{
				key: 'demo-key',
				label: 'Demo node',
				protocol: 'vless',
				server: 'demo.example.com',
				port: 443,
				supported: true,
			},
		]);
		vi.spyOn(window, 'requestAnimationFrame').mockImplementation(() => {
			return 1;
		});

		render(AddTunnelWizard, {
			props: { open: true, preselect: 'url', onclose: vi.fn() },
		});

		const inputs = await screen.findAllByRole('textbox');
		await fireEvent.input(inputs[0], { target: { value: 'Provider Demo' } });
		await fireEvent.input(inputs[1], { target: { value: 'https://example.com/sub.txt' } });

		const nextButton = screen.getByRole('button', { name: 'Далее' }) as HTMLButtonElement;
		const membersPromise = Promise.resolve().then(async () => {
			await fireEvent.click(nextButton);
		});
		await membersPromise;
		await waitFor(() => {
			expect(previewSubscriptionMock).toHaveBeenCalledTimes(1);
		});

		expect(nextButton.disabled).toBe(true);
		await fireEvent.click(nextButton);
		expect(previewSubscriptionMock).toHaveBeenCalledTimes(1);
	});

	it('shows all-unsupported preview rows and keeps create disabled', async () => {
		previewSubscriptionMock.mockResolvedValueOnce([
			{
				key: 'bad-1',
				label: 'Bad proto',
				protocol: 'foo',
				server: '',
				port: 0,
				supported: false,
				reason: 'unsupported outbound protocol',
			},
		]);

		render(AddTunnelWizard, {
			props: { open: true, preselect: 'url', onclose: vi.fn() },
		});

		const inputs = await screen.findAllByRole('textbox');
		await fireEvent.input(inputs[0], { target: { value: 'Provider Demo' } });
		await fireEvent.input(inputs[1], { target: { value: 'https://example.com/sub.txt' } });

		await fireEvent.click(screen.getByRole('button', { name: 'Далее' }));

		expect(await screen.findByText('Не поддерживается')).toBeTruthy();
		expect(screen.getByText('unsupported outbound protocol')).toBeTruthy();
		expect(screen.getByText('В подписке нет поддерживаемых серверов для добавления.')).toBeTruthy();

		const createButton = screen.getByRole('button', { name: 'Создать — оставить 0' }) as HTMLButtonElement;
		expect(createButton.disabled).toBe(true);
	});
});
