import { describe, it, expect, beforeEach, vi } from 'vitest';
import { get } from 'svelte/store';

function resetEnv(url: string) {
  window.history.replaceState({}, '', url);
  vi.resetModules();
}

describe('addWizardStore', () => {
  beforeEach(() => {
    resetEnv('/');
  });

  it('default state: closed, all empty', async () => {
    const m = await import('./addWizardStore');
    expect(get(m.addWizardOpen)).toBe(false);
    expect(get(m.wizardOutboundCategory)).toBe(null);
    expect(get(m.wizardTunnelTag)).toBe(null);
    const c = get(m.wizardCustom);
    expect(c.rulesList).toBe('');
  });

  it('openAddWizard sets URL ?add=1 + open=true', async () => {
    const m = await import('./addWizardStore');
    m.openAddWizard();
    expect(get(m.addWizardOpen)).toBe(true);
    expect(window.location.search).toContain('add=1');
  });

  it('closeAddWizard removes URL + clears all state', async () => {
    const m = await import('./addWizardStore');
    m.openAddWizard();
    m.setOutboundCategory('tunnel');
    m.setTunnelTag('warp');
    m.updateCustomField('rulesList', 'a.com');
    m.closeAddWizard();
    expect(get(m.addWizardOpen)).toBe(false);
    expect(get(m.wizardOutboundCategory)).toBe(null);
    expect(get(m.wizardTunnelTag)).toBe(null);
    expect(get(m.wizardCustom).rulesList).toBe('');
    expect(window.location.search).not.toContain('add=1');
  });

  it('setOutboundCategory updates', async () => {
    const m = await import('./addWizardStore');
    m.setOutboundCategory('tunnel');
    expect(get(m.wizardOutboundCategory)).toBe('tunnel');
    m.setOutboundCategory('block');
    expect(get(m.wizardOutboundCategory)).toBe('block');
    m.setOutboundCategory(null);
    expect(get(m.wizardOutboundCategory)).toBe(null);
  });

  it('setTunnelTag updates', async () => {
    const m = await import('./addWizardStore');
    m.setTunnelTag('warp');
    expect(get(m.wizardTunnelTag)).toBe('warp');
    m.setTunnelTag(null);
    expect(get(m.wizardTunnelTag)).toBe(null);
  });

  it('updateCustomField пишет rulesList', async () => {
    const m = await import('./addWizardStore');
    m.updateCustomField('rulesList', '*.netflix.com\n8.8.8.8');
    expect(get(m.wizardCustom).rulesList).toBe('*.netflix.com\n8.8.8.8');
  });

  it('resetWizardState очищает rulesList', async () => {
    const m = await import('./addWizardStore');
    m.updateCustomField('rulesList', 'foo.com');
    m.resetWizardState();
    expect(get(m.wizardCustom).rulesList).toBe('');
  });

  it('resetWizardState keeps open, clears selection/category/tunnel/custom', async () => {
    const m = await import('./addWizardStore');
    m.openAddWizard();
    m.setOutboundCategory('tunnel');
    m.setTunnelTag('warp');
    m.updateCustomField('rulesList', 'a.com');
    m.resetWizardState();
    expect(get(m.addWizardOpen)).toBe(true);
    expect(get(m.wizardOutboundCategory)).toBe(null);
    expect(get(m.wizardTunnelTag)).toBe(null);
    expect(get(m.wizardCustom).rulesList).toBe('');
  });

  it('module init с URL ?add=1 не восстанавливает визард', async () => {
    resetEnv('/?add=1');
    const m = await import('./addWizardStore');
    expect(get(m.addWizardOpen)).toBe(false);
  });

  it('openEditWizard: prefill + edit state', async () => {
    const m = await import('./addWizardStore');
    m.openEditWizard(5, {
      editMode: 'inline',
      rulesList: 'foo.com',
      outboundCategory: 'tunnel',
      tunnelTag: 'warp',
      existingInlineRuleSetTag: 'custom-1',
      wasInlineText: false,
    });
    expect(get(m.addWizardOpen)).toBe(true);
    expect(get(m.wizardEditRuleIndex)).toBe(5);
    expect(get(m.wizardEditMode)).toBe('inline');
    expect(get(m.wizardExistingInlineRuleSetTag)).toBe('custom-1');
    expect(get(m.wizardWasInlineText)).toBe(false);
    expect(get(m.wizardCustom).rulesList).toBe('foo.com');
    expect(get(m.wizardOutboundCategory)).toBe('tunnel');
    expect(get(m.wizardTunnelTag)).toBe('warp');
  });

  it('openEditWizard external mode', async () => {
    const m = await import('./addWizardStore');
    m.openEditWizard(2, {
      editMode: 'external',
      rulesList: '',
      outboundCategory: 'block',
      tunnelTag: null,
      wasInlineText: false,
    });
    expect(get(m.wizardEditMode)).toBe('external');
    expect(get(m.wizardOutboundCategory)).toBe('block');
  });

  it('closeAddWizard clears edit state', async () => {
    const m = await import('./addWizardStore');
    m.openEditWizard(1, {
      editMode: 'inline',
      rulesList: 'a.com',
      outboundCategory: 'direct',
      tunnelTag: null,
      wasInlineText: true,
    });
    m.closeAddWizard();
    expect(get(m.wizardEditRuleIndex)).toBe(null);
    expect(get(m.wizardEditMode)).toBe(null);
    expect(get(m.wizardWasInlineText)).toBe(false);
  });

  it('openAddWizard clears prior edit state', async () => {
    const m = await import('./addWizardStore');
    m.openEditWizard(9, {
      editMode: 'inline',
      rulesList: 'x.com',
      outboundCategory: 'tunnel',
      tunnelTag: 'warp',
    });
    m.openAddWizard();
    expect(get(m.wizardEditRuleIndex)).toBe(null);
    expect(get(m.wizardEditMode)).toBe(null);
  });
});
