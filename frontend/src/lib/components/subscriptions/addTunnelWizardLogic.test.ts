import { describe, expect, it } from 'vitest';
import type { SubscriptionPreviewMember } from '$lib/types';
import {
	buildExcludedSelectableKeys,
	canCreateSubscriptionFromPreview,
	canRequestSubscriptionPreview,
	countExcludedSelectableMembers,
	countSelectablePreviewMembers,
	dedupePreviewMembers,
	isAddTunnelWizardDirty,
	parseRefreshHours,
} from './addTunnelWizardLogic';

const previewMembers: SubscriptionPreviewMember[] = [
	{
		key: 'demo-key',
		label: 'Demo node',
		protocol: 'vless',
		server: 'demo.example.com',
		port: 443,
		supported: true,
	},
	{
		key: 'bad-1',
		label: 'Bad proto',
		protocol: 'foo',
		server: '',
		port: 0,
		supported: false,
		reason: 'unsupported outbound protocol',
	},
];

describe('addTunnelWizardLogic', () => {
	it('parses refresh hours safely', () => {
		expect(parseRefreshHours('24')).toBe(24);
		expect(parseRefreshHours('')).toBe(0);
		expect(parseRefreshHours('foo')).toBe(0);
	});

	it('tracks dirty state only after real edits', () => {
		expect(
			isAddTunnelWizardDirty(
				{
					kind: 'url',
					singleLinks: '',
					label: '',
					url: '',
					inlineText: '',
					headersText: 'preset',
					refreshHoursStr: '24',
					enabled: true,
					mode: 'selector',
					utUrl: 'https://gstatic',
					utIntervalSec: 60,
					utToleranceMs: 50,
				},
				'preset',
				{ url: 'https://gstatic', intervalSec: 60, toleranceMs: 50 },
			),
		).toBe(false);

		expect(
			isAddTunnelWizardDirty(
				{
					kind: 'url',
					singleLinks: '',
					label: 'Provider Demo',
					url: '',
					inlineText: '',
					headersText: 'preset',
					refreshHoursStr: '24',
					enabled: true,
					mode: 'selector',
					utUrl: 'https://gstatic',
					utIntervalSec: 60,
					utToleranceMs: 50,
				},
				'preset',
				{ url: 'https://gstatic', intervalSec: 60, toleranceMs: 50 },
			),
		).toBe(true);
	});

	it('enables preview only when label and url exist and no preview is in progress', () => {
		expect(
			canRequestSubscriptionPreview({
				previewing: false,
				mountingPreview: false,
				label: '',
				url: 'https://example.com/sub.txt',
			}),
		).toBe(false);

		expect(
			canRequestSubscriptionPreview({
				previewing: false,
				mountingPreview: false,
				label: 'Provider Demo',
				url: 'https://example.com/sub.txt',
			}),
		).toBe(true);

		expect(
			canRequestSubscriptionPreview({
				previewing: true,
				mountingPreview: false,
				label: 'Provider Demo',
				url: 'https://example.com/sub.txt',
			}),
		).toBe(false);
	});

	it('dedupes preview members by key', () => {
		const deduped = dedupePreviewMembers([
			previewMembers[0]!,
			previewMembers[0]!,
			previewMembers[1]!,
		]);
		expect(deduped).toHaveLength(2);
		expect(deduped.map((member) => member.key)).toEqual(['demo-key', 'bad-1']);
	});

	it('counts selectable members and excluded keys correctly', () => {
		const excluded = buildExcludedSelectableKeys(previewMembers);

		expect(countSelectablePreviewMembers(previewMembers)).toBe(1);
		expect(countExcludedSelectableMembers(previewMembers, excluded)).toBe(1);
		expect(canCreateSubscriptionFromPreview(previewMembers, excluded)).toBe(false);
		expect(canCreateSubscriptionFromPreview(previewMembers, new Set())).toBe(true);
	});
});
