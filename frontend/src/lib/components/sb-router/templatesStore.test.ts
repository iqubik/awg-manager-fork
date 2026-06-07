import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import {
  templatesOpen, templatesSelection, templatesFilter, templatesQuery, templatesOutbound,
  openTemplatesModal, closeTemplatesModal, toggleTemplate, clearSelection,
  setFilter, setQuery, setOutbound,
  catalogIdsFromTemplatesSelection, setServiceTemplateSelection,
} from './templatesStore';

describe('templatesStore', () => {
  beforeEach(() => {
    closeTemplatesModal();
  });

  it('default state: closed, empty selection, filter=all, query empty, outbound null', () => {
    expect(get(templatesOpen)).toBe(false);
    expect(get(templatesSelection).size).toBe(0);
    expect(get(templatesFilter)).toBe('all');
    expect(get(templatesQuery)).toBe('');
    expect(get(templatesOutbound)).toBe(null);
  });

  it('openTemplatesModal sets open=true', () => {
    openTemplatesModal();
    expect(get(templatesOpen)).toBe(true);
  });

  it('closeTemplatesModal resets everything', () => {
    openTemplatesModal();
    toggleTemplate('svc:x');
    setFilter('services');
    setQuery('net');
    setOutbound('warp');
    closeTemplatesModal();
    expect(get(templatesOpen)).toBe(false);
    expect(get(templatesSelection).size).toBe(0);
    expect(get(templatesFilter)).toBe('all');
    expect(get(templatesQuery)).toBe('');
    expect(get(templatesOutbound)).toBe(null);
  });

  it('toggleTemplate adds id when not present', () => {
    toggleTemplate('svc:x');
    expect(get(templatesSelection).has('svc:x')).toBe(true);
  });

  it('toggleTemplate removes id when present', () => {
    toggleTemplate('svc:x');
    toggleTemplate('svc:x');
    expect(get(templatesSelection).has('svc:x')).toBe(false);
  });

  it('toggleTemplate keeps multiple ids', () => {
    toggleTemplate('svc:x');
    toggleTemplate('rs:y');
    const sel = get(templatesSelection);
    expect(sel.size).toBe(2);
    expect(sel.has('svc:x')).toBe(true);
    expect(sel.has('rs:y')).toBe(true);
  });

  it('clearSelection empties selection without affecting other state', () => {
    openTemplatesModal();
    toggleTemplate('svc:x');
    setFilter('services');
    clearSelection();
    expect(get(templatesSelection).size).toBe(0);
    expect(get(templatesOpen)).toBe(true);
    expect(get(templatesFilter)).toBe('services');
  });

  it('setFilter updates filter value', () => {
    setFilter('services');
    expect(get(templatesFilter)).toBe('services');
    setFilter('rulesets');
    expect(get(templatesFilter)).toBe('rulesets');
    setFilter('all');
    expect(get(templatesFilter)).toBe('all');
  });

  it('setQuery trims whitespace', () => {
    setQuery('  netflix  ');
    expect(get(templatesQuery)).toBe('netflix');
  });

  it('setOutbound stores tag or null', () => {
    setOutbound('warp');
    expect(get(templatesOutbound)).toBe('warp');
    setOutbound(null);
    expect(get(templatesOutbound)).toBe(null);
  });

  it('catalogIdsFromTemplatesSelection extracts svc ids', () => {
    toggleTemplate('svc:netflix');
    toggleTemplate('rs:geoip-ru');
    expect(catalogIdsFromTemplatesSelection(get(templatesSelection))).toEqual(['netflix']);
  });

  it('setServiceTemplateSelection replaces svc keys and keeps rs', () => {
    toggleTemplate('rs:geoip-ru');
    toggleTemplate('svc:old');
    setServiceTemplateSelection(['netflix', 'youtube']);
    const sel = get(templatesSelection);
    expect(sel.has('rs:geoip-ru')).toBe(true);
    expect(sel.has('svc:old')).toBe(false);
    expect(sel.has('svc:netflix')).toBe(true);
    expect(sel.has('svc:youtube')).toBe(true);
  });
});
