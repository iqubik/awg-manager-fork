import { api } from '$lib/api/client';
import type { TemplateGroup, TemplateItem } from './templatesData';

export interface SubmitResult {
  successes: string[];
  failures: Array<{ id: string; error: string }>;
}

function findItem(groups: TemplateGroup[], id: string): TemplateItem | undefined {
  for (const g of groups) {
    const found = g.items.find((it) => it.id === id);
    if (found) return found;
  }
  return undefined;
}

async function applyOne(item: TemplateItem, outboundOrBlock: string): Promise<void> {
  if (item.category === 'services') {
    const outbound = outboundOrBlock === 'block' ? '' : outboundOrBlock;
    await api.singboxRouterApplyPreset(item.presetId, outbound);
    return;
  }
  // rulesets
  if (outboundOrBlock === 'block') {
    await api.singboxRouterAddRule({
      rule_set: [item.tag],
      action: 'reject',
    });
  } else {
    await api.singboxRouterAddRule({
      rule_set: [item.tag],
      outbound: outboundOrBlock,
      action: 'route',
    });
  }
}

export async function submitTemplates(
  selection: string[],
  outboundOrBlock: string,
  groups: TemplateGroup[],
): Promise<SubmitResult> {
  // Последовательно: каждый ApplyPreset — read/modify/write конфига без блокировки.
  // Параллельные вызовы гоняются и в конфиге остаётся только последний пресет.
  const successes: string[] = [];
  const failures: Array<{ id: string; error: string }> = [];

  for (const id of selection) {
    const item = findItem(groups, id);
    if (!item) {
      failures.push({ id, error: 'template not found' });
      continue;
    }
    try {
      await applyOne(item, outboundOrBlock);
      successes.push(id);
    } catch (e) {
      const msg = e instanceof Error ? e.message : String(e);
      failures.push({ id, error: msg });
    }
  }

  return { successes, failures };
}
