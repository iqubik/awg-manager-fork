import { describe, expect, it } from 'vitest';
import { buildAddWizardUrl } from './addWizardStoreLogic';

describe('addWizardStoreLogic', () => {
	it('builds open wizard URL in singbox tab and clears trace params', () => {
		expect(buildAddWizardUrl('/routing?tab=ip&trace=1&q=discord.com', true)).toBe(
			'/routing?tab=singbox&add=1',
		);
	});

	it('builds closed wizard URL and removes add/edit params', () => {
		expect(buildAddWizardUrl('/routing?tab=singbox&add=1&edit=1&other=keep', false)).toBe(
			'/routing?tab=singbox&other=keep',
		);
	});
});
