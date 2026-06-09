import { describe, expect, it } from 'vitest';
import {
	computeRuleCardBadgeBudget,
	countVisibleBadges,
	RULE_CARD_TUNNEL_BUDGET_MAX,
	ruleCardTunnelBudgetCap,
} from './fittingBadgeLayout';

describe('countVisibleBadges', () => {
	const base = {
		arrowWidth: 14,
		overflowChipWidth: 28,
		gap: 6,
	};

	it('shows all badges when row fits', () => {
		expect(
			countVisibleBadges({
				...base,
				badgeWidths: [120, 130, 125],
				availableWidth: 500,
			}),
		).toBe(3);
	});

	it('collapses with +N when space is tight', () => {
		expect(
			countVisibleBadges({
				...base,
				badgeWidths: [120, 130, 125],
				availableWidth: 180,
			}),
		).toBeLessThan(3);
	});

	it('expands again after widening (no stale collapsed state in math)', () => {
		const widths = [100, 100, 100];
		expect(countVisibleBadges({ ...base, badgeWidths: widths, availableWidth: 160 })).toBe(1);
		expect(countVisibleBadges({ ...base, badgeWidths: widths, availableWidth: 500 })).toBe(3);
	});
});

describe('ruleCardTunnelBudgetCap', () => {
	it('on desktop uses 40vw', () => {
		expect(ruleCardTunnelBudgetCap(1200, false)).toBe(480);
		expect(ruleCardTunnelBudgetCap(800, false)).toBe(320);
	});

	it('on mobile keeps 420px cap', () => {
		expect(ruleCardTunnelBudgetCap(400, true)).toBe(420);
	});
});

describe('computeRuleCardBadgeBudget', () => {
	it('uses slack in main column, not collapsed trail width', () => {
		// card 1000px, content ends at 300, buttons 60, pad 14, gap 12
		const budget = computeRuleCardBadgeBudget({
			cardRight: 1000,
			paddingRight: 14,
			mainContentRight: 300,
			columnGap: 12,
			buttonsWidth: 60,
		});
		expect(budget).toBe(RULE_CARD_TUNNEL_BUDGET_MAX);
	});

	it('matches old trail-only budget when main content fills the cell', () => {
		const budget = computeRuleCardBadgeBudget({
			cardRight: 800,
			paddingRight: 14,
			mainContentRight: 720,
			columnGap: 12,
			buttonsWidth: 60,
		});
		// 800 - 14 - 720 - 12 - 60 - 8 = -14 → 0
		expect(budget).toBe(0);
	});
});
