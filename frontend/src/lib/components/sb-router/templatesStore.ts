import { writable, type Readable, type Writable } from 'svelte/store';
import type { FilterKey } from './templatesData';

const openW: Writable<boolean> = writable(false);
const selectionW: Writable<Set<string>> = writable(new Set());
const filterW: Writable<FilterKey> = writable('all');
const queryW: Writable<string> = writable('');
const outboundW: Writable<string | null> = writable(null);

export const templatesOpen: Readable<boolean> = { subscribe: openW.subscribe };
export const templatesSelection: Readable<Set<string>> = { subscribe: selectionW.subscribe };
export const templatesFilter: Readable<FilterKey> = { subscribe: filterW.subscribe };
export const templatesQuery: Readable<string> = { subscribe: queryW.subscribe };
export const templatesOutbound: Readable<string | null> = { subscribe: outboundW.subscribe };

export function openTemplatesModal(): void {
  openW.set(true);
}

export function closeTemplatesModal(): void {
  openW.set(false);
  selectionW.set(new Set());
  filterW.set('all');
  queryW.set('');
  outboundW.set(null);
}

export function toggleTemplate(id: string): void {
  selectionW.update((set) => {
    const next = new Set(set);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    return next;
  });
}

export function clearSelection(): void {
  selectionW.set(new Set());
}

export function setFilter(key: FilterKey): void {
  filterW.set(key);
}

export function setQuery(q: string): void {
  queryW.set(q.trim());
}

export function setOutbound(tag: string | null): void {
  outboundW.set(tag);
}

export function dismissTemplatesModal(): void {
  openW.set(false);
  // Намеренно НЕ очищаем selection/filter/query/outbound —
  // вызывающий код (например, AddRuleWizard) владеет selection'ом
  // и переиспользует его после закрытия.
}

/** Preset ids from svc:… template keys (for ServiceCatalogModal). */
export function catalogIdsFromTemplatesSelection(sel: Set<string>): string[] {
  return [...sel].filter((id) => id.startsWith('svc:')).map((id) => id.slice(4));
}

/** Replace service template picks; keeps rs:… selections (expert wizard). */
export function setServiceTemplateSelection(presetIds: string[]): void {
  selectionW.update((current) => {
    const next = new Set([...current].filter((id) => !id.startsWith('svc:')));
    for (const id of presetIds) next.add(`svc:${id}`);
    return next;
  });
}
