import { writable, type Readable, type Writable } from 'svelte/store';
import { closeTrace } from './traceStore';

export type OutboundCategory = 'tunnel' | 'direct' | 'block';

export interface CustomMatcherFields {
  rulesList: string;
}

function emptyCustom(): CustomMatcherFields {
  return { rulesList: '' };
}

const openW: Writable<boolean> = writable(false);
const categoryW: Writable<OutboundCategory | null> = writable(null);
const tunnelW: Writable<string | null> = writable(null);
const customW: Writable<CustomMatcherFields> = writable(emptyCustom());

export const addWizardOpen: Readable<boolean> = { subscribe: openW.subscribe };
export const wizardOutboundCategory: Readable<OutboundCategory | null> = { subscribe: categoryW.subscribe };
export const wizardTunnelTag: Readable<string | null> = { subscribe: tunnelW.subscribe };
export const wizardCustom: Readable<CustomMatcherFields> = { subscribe: customW.subscribe };

function setUrl(open: boolean) {
  if (typeof window === 'undefined') return;
  const url = new URL(window.location.href);
  if (open) {
    url.searchParams.set('add', '1');
    // wizard and trace are mutually exclusive — close trace
    url.searchParams.delete('trace');
    url.searchParams.delete('q');
  } else {
    url.searchParams.delete('add');
  }
  window.history.replaceState({}, '', url);
}

export function openAddWizard(): void {
  // Close trace if open
  closeTrace();
  openW.set(true);
  setUrl(true);
}

export function closeAddWizard(): void {
  openW.set(false);
  categoryW.set(null);
  tunnelW.set(null);
  customW.set(emptyCustom());
  setUrl(false);
}

export function setOutboundCategory(c: OutboundCategory | null): void {
  categoryW.set(c);
}

export function setTunnelTag(tag: string | null): void {
  tunnelW.set(tag);
}

export function updateCustomField<K extends keyof CustomMatcherFields>(
  key: K,
  value: CustomMatcherFields[K],
): void {
  customW.update((c) => ({ ...c, [key]: value }));
}

export function resetWizardState(): void {
  // НЕ закрываем wizard. Очищаем выбор и кастом, чтобы создать следующее правило.
  categoryW.set(null);
  tunnelW.set(null);
  customW.set(emptyCustom());
}

// Module init: read URL on load
if (typeof window !== 'undefined') {
  const params = new URLSearchParams(window.location.search);
  if (params.get('add') === '1') {
    openW.set(true);
    // если и trace=1 — strip его (wizard wins)
    if (params.has('trace') || params.has('q')) {
      const url = new URL(window.location.href);
      url.searchParams.delete('trace');
      url.searchParams.delete('q');
      window.history.replaceState({}, '', url);
    }
  }
}
