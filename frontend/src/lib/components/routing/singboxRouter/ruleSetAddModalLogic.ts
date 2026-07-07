import {
	analyzeInlineRuleListLossy,
	stringifyInlineRuleList,
} from '$lib/utils/singboxInlineRules';
import { datInfo } from '$lib/utils/ruleSetType';
import type { SingboxRouterRuleSet } from '$lib/types';

export type RuleSetFormType = 'remote' | 'local' | 'inline' | 'geosite' | 'geoip';

export interface RuleSetDraftState {
	type: RuleSetFormType;
	format: 'binary' | 'source';
	tag: string;
	url: string;
	updateInterval: string;
	downloadDetour: string;
	path: string;
	rulesJson: string;
	inlineMode: 'list' | 'json';
	rulesList: string;
	selectedGeoTags: string[];
}

export const DEFAULT_RULES_JSON = `[
  {
    "domain_suffix": [
      "example.com"
    ]
  }
]`;

export function standardDatRuleSetTag(kind: 'geosite' | 'geoip', pickedTags: string[]): string {
	return `${kind}-${pickedTags.join('-')}`.toLowerCase().replace(/[^a-z0-9._-]+/g, '-');
}

export function createRuleSetDraftState(ruleSet?: SingboxRouterRuleSet): RuleSetDraftState {
	const dat = ruleSet ? datInfo(ruleSet) : null;
	const type: RuleSetFormType = dat?.kind ?? ruleSet?.type ?? 'remote';
	const format: 'binary' | 'source' = ruleSet?.format ?? 'binary';
	const tag = ruleSet?.tag ?? '';
	const url = ruleSet?.url ?? '';
	const updateInterval = ruleSet?.update_interval ?? '24h';
	const downloadDetour = ruleSet?.download_detour ?? '';
	const path = ruleSet?.path ?? '';
	const rulesJson = ruleSet?.rules?.length ? JSON.stringify(ruleSet.rules, null, 2) : DEFAULT_RULES_JSON;
	const inlineLossyAnalysis =
		ruleSet?.type === 'inline'
			? analyzeInlineRuleListLossy(ruleSet.rules)
			: { lossy: false, issues: [] as string[] };
	const inlineMode: 'list' | 'json' =
		ruleSet?.type === 'inline' && ruleSet?.rules?.length && inlineLossyAnalysis.lossy ? 'json' : 'list';
	const rulesList =
		type === 'inline' && ruleSet?.rules?.length ? stringifyInlineRuleList(ruleSet.rules) : '';

	return {
		type,
		format,
		tag,
		url,
		updateInterval,
		downloadDetour,
		path,
		rulesJson,
		inlineMode,
		rulesList,
		selectedGeoTags: dat?.tags ?? [],
	};
}

export function applyGeoTagToggle(options: {
	type: RuleSetFormType;
	tag: string;
	selectedGeoTags: string[];
	pickedTag: string;
}): { tag: string; selectedGeoTags: string[] } {
	const { type, pickedTag } = options;
	if ((type !== 'geosite' && type !== 'geoip') || !pickedTag.trim()) {
		return {
			tag: options.tag,
			selectedGeoTags: options.selectedGeoTags,
		};
	}

	const selectedGeoTags = options.selectedGeoTags.includes(pickedTag)
		? options.selectedGeoTags.filter((tag) => tag !== pickedTag)
		: [...options.selectedGeoTags, pickedTag];

	const shouldUpdateRuleSetTag =
		options.tag.trim() === '' ||
		(options.selectedGeoTags.length > 0 &&
			options.tag.trim() === standardDatRuleSetTag(type, options.selectedGeoTags));

	return {
		selectedGeoTags,
		tag: shouldUpdateRuleSetTag
			? selectedGeoTags.length > 0
				? standardDatRuleSetTag(type, selectedGeoTags)
				: ''
			: options.tag,
	};
}

export function savedRuleSetType(type: RuleSetFormType): SingboxRouterRuleSet['type'] {
	return type === 'geosite' || type === 'geoip' ? 'remote' : type;
}

export async function buildRuleSetPayload(options: {
	type: RuleSetFormType;
	format: 'binary' | 'source';
	tag: string;
	url: string;
	updateInterval: string;
	downloadDetour: string;
	path: string;
	parsedRules?: Record<string, unknown>[];
	selectedGeoTags: string[];
	resolveDatRuleSetURL: (kind: 'geosite' | 'geoip', tags: string[]) => Promise<string>;
}): Promise<SingboxRouterRuleSet> {
	const { type, format, tag, url, updateInterval, downloadDetour, path, parsedRules, selectedGeoTags } = options;

	let builtUrl = url.trim();
	const isDatType = type === 'geosite' || type === 'geoip';
	if (isDatType) {
		builtUrl = await options.resolveDatRuleSetURL(type, selectedGeoTags);
	}

	const finalType = savedRuleSetType(type);
	return {
		tag: tag.trim(),
		type: finalType,
		format: finalType === 'inline' ? undefined : isDatType ? 'binary' : format,
		url: finalType === 'remote' ? builtUrl : undefined,
		update_interval: finalType === 'remote' ? (isDatType ? '24h' : updateInterval) : undefined,
		download_detour: type === 'remote' && downloadDetour ? downloadDetour : undefined,
		path: finalType === 'local' ? path.trim() : undefined,
		rules: finalType === 'inline' ? parsedRules : undefined,
	};
}
