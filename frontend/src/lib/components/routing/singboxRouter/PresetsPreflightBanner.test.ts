import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent, screen } from '@testing-library/svelte';
import PresetsPreflightBanner from './PresetsPreflightBanner.svelte';

describe('PresetsPreflightBanner', () => {
	it('renders nothing when status is ok', () => {
		const { container } = render(PresetsPreflightBanner, {
			props: { status: 'ok', policyName: 'Policy2' },
		});
		expect(container.textContent?.trim()).toBe('');
	});

	it('renders yellow banner with CTA for no-policy', () => {
		render(PresetsPreflightBanner, {
			props: { status: 'no-policy', policyName: null },
		});
		expect(screen.getByText(/не выбрана политика/i)).toBeTruthy();
		expect(screen.getByRole('link', { name: /политик/i })).toBeTruthy();
	});

	it('renders red banner with Reset and CTA for no-policy-in-ndms', () => {
		const onReset = vi.fn();
		render(PresetsPreflightBanner, {
			props: { status: 'no-policy-in-ndms', policyName: 'Policy2', onResetPolicyName: onReset },
		});
		expect(screen.getByText(/Policy2/)).toBeTruthy();
		expect(screen.getByText(/отсутствует в роутере/i)).toBeTruthy();
		const resetBtn = screen.getByRole('button', { name: /сбросить/i });
		fireEvent.click(resetBtn);
		expect(onReset).toHaveBeenCalled();
	});

	it('renders yellow banner with policy name for no-devices', () => {
		render(PresetsPreflightBanner, {
			props: { status: 'no-devices', policyName: 'Policy2' },
		});
		expect(screen.getByText(/Policy2/)).toBeTruthy();
		expect(screen.getByText(/пуста/i)).toBeTruthy();
	});
});
