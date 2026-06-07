import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('$lib/api/client', () => ({
  api: {
    singboxRouterApplyPreset: vi.fn(),
    singboxRouterAddRule: vi.fn(),
    singboxRouterAddRuleSet: vi.fn(),
    expandGeoTag: vi.fn(),
  },
}));

vi.mock('$lib/utils/singboxInlineGeoExpand', () => ({
  expandGeoLinesInInput: vi.fn(async (text: string) => ({ text, warnings: [] })),
}));

import { api } from '$lib/api/client';
import { expandGeoLinesInInput } from '$lib/utils/singboxInlineGeoExpand';
import {
  resolveOutbound, submitWizard, ValidationError,
  nextCustomRuleSetTag, parseCustomList,
} from './addWizardActions';
import type { CustomMatcherFields } from './addWizardStore';
import type { TemplateGroup } from './templatesData';

const emptyCustom: CustomMatcherFields = { rulesList: '' };
const groups: TemplateGroup[] = [
  {
    category: 'services', title: 'Сервисы',
    items: [{ id: 'svc:netflix', category: 'services', presetId: 'netflix', name: 'Netflix' }],
  },
];

describe('resolveOutbound', () => {
  it('tunnel returns tag', () => {
    expect(resolveOutbound('tunnel', 'warp')).toBe('warp');
  });

  it('tunnel without tag throws', () => {
    expect(() => resolveOutbound('tunnel', null)).toThrow(ValidationError);
  });

  it('direct returns "direct"', () => {
    expect(resolveOutbound('direct', null)).toBe('direct');
  });

  it('block returns "block"', () => {
    expect(resolveOutbound('block', null)).toBe('block');
  });
});

describe('nextCustomRuleSetTag', () => {
  it('первый свободный custom-N', () => {
    expect(nextCustomRuleSetTag([])).toBe('custom-1');
    expect(nextCustomRuleSetTag(['custom-1', 'geosite-x'])).toBe('custom-2');
    expect(nextCustomRuleSetTag(['custom-2'])).toBe('custom-1');
  });
});

describe('submitWizard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    (expandGeoLinesInInput as ReturnType<typeof vi.fn>).mockImplementation(
      async (text: string) => ({ text, warnings: [] }),
    );
  });

  it('no templates AND no custom → throws ValidationError', async () => {
    await expect(submitWizard({
      selectedTemplates: [], customFields: emptyCustom,
      outboundCategory: 'tunnel', tunnelTag: 'warp', groups,
      existingRuleSetTags: [],
    })).rejects.toThrow(ValidationError);
  });

  it('templates only → submitTemplates called', async () => {
    (api.singboxRouterApplyPreset as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    const r = await submitWizard({
      selectedTemplates: ['svc:netflix'], customFields: emptyCustom,
      outboundCategory: 'tunnel', tunnelTag: 'warp', groups,
      existingRuleSetTags: [],
    });
    expect(api.singboxRouterApplyPreset).toHaveBeenCalledWith('netflix', 'warp');
    expect(r.successes).toEqual(['svc:netflix']);
  });

  it('custom only → rule_set создаётся и ссылается rule', async () => {
    (api.singboxRouterAddRuleSet as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    (api.singboxRouterAddRule as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    const r = await submitWizard({
      selectedTemplates: [],
      customFields: { rulesList: 'domain:example.com' },
      outboundCategory: 'tunnel', tunnelTag: 'warp', groups,
      existingRuleSetTags: [],
    });
    expect(api.singboxRouterAddRuleSet).toHaveBeenCalledWith(
      expect.objectContaining({ tag: 'custom-1', type: 'inline' }),
    );
    expect(api.singboxRouterAddRule).toHaveBeenCalledWith(
      expect.objectContaining({ rule_set: ['custom-1'], outbound: 'warp', action: 'route' }),
    );
    expect(r.successes).toEqual(['custom']);
  });

  it('block outbound → action=reject в rule', async () => {
    (api.singboxRouterAddRuleSet as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    (api.singboxRouterAddRule as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    const r = await submitWizard({
      selectedTemplates: [],
      customFields: { rulesList: 'domain:example.com' },
      outboundCategory: 'block', tunnelTag: null, groups,
      existingRuleSetTags: [],
    });
    expect(api.singboxRouterAddRule).toHaveBeenCalledWith(
      expect.objectContaining({ rule_set: ['custom-1'], action: 'reject' }),
    );
    expect(r.successes).toEqual(['custom']);
  });

  it('existingRuleSetTags → следующий свободный тег', async () => {
    (api.singboxRouterAddRuleSet as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    (api.singboxRouterAddRule as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    await submitWizard({
      selectedTemplates: [],
      customFields: { rulesList: 'domain:example.com' },
      outboundCategory: 'direct', tunnelTag: null, groups,
      existingRuleSetTags: ['custom-1', 'custom-2'],
    });
    expect(api.singboxRouterAddRuleSet).toHaveBeenCalledWith(
      expect.objectContaining({ tag: 'custom-3' }),
    );
  });

  it('templates + custom → both submitted, aggregated', async () => {
    (api.singboxRouterApplyPreset as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    (api.singboxRouterAddRuleSet as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    (api.singboxRouterAddRule as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    const r = await submitWizard({
      selectedTemplates: ['svc:netflix'],
      customFields: { rulesList: 'domain:example.com' },
      outboundCategory: 'tunnel', tunnelTag: 'warp', groups,
      existingRuleSetTags: [],
    });
    expect(r.successes.sort()).toEqual(['custom', 'svc:netflix']);
    expect(r.failures).toEqual([]);
  });

  it('partial failure: custom API fails', async () => {
    (api.singboxRouterApplyPreset as ReturnType<typeof vi.fn>).mockResolvedValue(undefined);
    (api.singboxRouterAddRuleSet as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('bad rule'));
    const r = await submitWizard({
      selectedTemplates: ['svc:netflix'],
      customFields: { rulesList: 'domain:example.com' },
      outboundCategory: 'tunnel', tunnelTag: 'warp', groups,
      existingRuleSetTags: [],
    });
    expect(r.successes).toEqual(['svc:netflix']);
    expect(r.failures).toEqual([{ id: 'custom', error: 'bad rule' }]);
  });

  it('tunnel category без tunnelTag → throws', async () => {
    await expect(submitWizard({
      selectedTemplates: ['svc:netflix'], customFields: emptyCustom,
      outboundCategory: 'tunnel', tunnelTag: null, groups,
      existingRuleSetTags: [],
    })).rejects.toThrow(ValidationError);
  });
});
