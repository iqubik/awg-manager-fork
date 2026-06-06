import { api } from '$lib/api/client';
import type { SingboxRouterRule, SingboxRouterRuleSet } from '$lib/types';
import { parseInlineRuleList, isInlineRuleListEmpty } from '$lib/utils/singboxInlineRules';
import { expandGeoLinesInInput } from '$lib/utils/singboxInlineGeoExpand';
import { submitTemplates, type SubmitResult } from './templatesActions';
import type { TemplateGroup } from './templatesData';
import type { CustomMatcherFields, OutboundCategory } from './addWizardStore';

export class ValidationError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'ValidationError';
  }
}

export function resolveOutbound(
  category: OutboundCategory,
  tunnelTag: string | null,
): string {
  if (category === 'tunnel') {
    if (!tunnelTag) throw new ValidationError('Выберите туннель');
    return tunnelTag;
  }
  if (category === 'direct') return 'direct';
  return 'block';
}

/** Первый свободный тег custom-N среди существующих rule_set'ов. */
export function nextCustomRuleSetTag(existing: string[]): string {
  const set = new Set(existing);
  let n = 1;
  while (set.has(`custom-${n}`)) n++;
  return `custom-${n}`;
}

/** Парсит smart-list (с geo-expand) в правила inline rule_set. Бросает ValidationError. */
export async function parseCustomList(rulesList: string): Promise<Record<string, unknown>[]> {
  if (isInlineRuleListEmpty(rulesList)) throw new ValidationError('Список пуст');
  const { text } = await expandGeoLinesInInput(
    rulesList,
    async (kind, tag) => (await api.expandGeoTag(kind, tag)).lines,
  );
  const parsed = parseInlineRuleList(text);
  if (parsed.errors.length > 0) throw new ValidationError(parsed.errors.join('\n'));
  if (parsed.rules.length === 0) throw new ValidationError('Нет валидных строк');
  return parsed.rules;
}

export interface SubmitWizardArgs {
  selectedTemplates: string[];
  customFields: CustomMatcherFields;
  outboundCategory: OutboundCategory;
  tunnelTag: string | null;
  groups: TemplateGroup[];
  existingRuleSetTags: string[];
}

export async function submitWizard(args: SubmitWizardArgs): Promise<SubmitResult> {
  const outbound = resolveOutbound(args.outboundCategory, args.tunnelTag);
  const hasCustom = !isInlineRuleListEmpty(args.customFields.rulesList);

  if (args.selectedTemplates.length === 0 && !hasCustom) {
    throw new ValidationError('Выберите шаблон или опишите правило');
  }

  // Кастом валидируем ДО любых сетевых вызовов — никаких частичных провалов из-за невалидного ввода.
  let customRules: Record<string, unknown>[] | null = null;
  if (hasCustom) customRules = await parseCustomList(args.customFields.rulesList);

  let combined: SubmitResult = { successes: [], failures: [] };

  if (args.selectedTemplates.length > 0) {
    combined = await submitTemplates(args.selectedTemplates, outbound, args.groups);
  }

  if (customRules) {
    try {
      const tag = nextCustomRuleSetTag(args.existingRuleSetTags);
      const rs: SingboxRouterRuleSet = { tag, type: 'inline', rules: customRules };
      await api.singboxRouterAddRuleSet(rs);
      const rule: Partial<SingboxRouterRule> = { rule_set: [tag] };
      if (outbound === 'block') {
        rule.action = 'reject';
      } else {
        rule.outbound = outbound;
        rule.action = 'route';
      }
      await api.singboxRouterAddRule(rule as SingboxRouterRule);
      combined.successes.push('custom');
    } catch (e) {
      combined.failures.push({ id: 'custom', error: e instanceof Error ? e.message : String(e) });
    }
  }

  return combined;
}
