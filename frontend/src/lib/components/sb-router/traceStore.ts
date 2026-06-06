/**
 * State + URL sync для F4b Trace Screen.
 *
 * URL contract:
 *   ?trace=1            → traceOpen=true (screen visible)
 *   ?trace=1&q=domain   → ditto + traceInput.domain pre-filled (auto-run on init)
 *   no ?trace           → traceOpen=false (fallback к FlowGraph+RulesPanel)
 *
 * API call идёт через api.singboxRouterInspectRoute — backend уже готов.
 */
import { writable, type Readable, type Writable } from 'svelte/store';
import { api } from '$lib/api/client';
import type { SingboxRouterInspectResult } from '$lib/types';

export interface TraceInput {
  domain: string;
  port?: number;
  protocol?: string;
}

function readURL(): { open: boolean; domain: string } {
  if (typeof window === 'undefined') return { open: false, domain: '' };
  const sp = new URL(window.location.href).searchParams;
  return {
    open: sp.get('trace') === '1',
    domain: sp.get('q') ?? '',
  };
}

function updateURL(open: boolean, domain: string): void {
  if (typeof window === 'undefined') return;
  try {
    const url = new URL(window.location.href);
    if (open) {
      url.searchParams.set('trace', '1');
      if (domain) {
        url.searchParams.set('q', domain);
      } else {
        url.searchParams.delete('q');
      }
    } else {
      url.searchParams.delete('trace');
      url.searchParams.delete('q');
    }
    window.history.replaceState({}, '', url.toString());
  } catch {
    /* non-browser env or restricted history — ignore */
  }
}

const initial = readURL();
const openStore = writable<boolean>(initial.open);

export const traceOpen: Readable<boolean> = { subscribe: openStore.subscribe };
export const traceInput: Writable<TraceInput> = writable<TraceInput>({ domain: initial.domain });
export const traceResult: Writable<SingboxRouterInspectResult | null> = writable<SingboxRouterInspectResult | null>(null);
export const traceLoading: Writable<boolean> = writable<boolean>(false);
export const traceError: Writable<string | null> = writable<string | null>(null);

export function openTrace(domain?: string): void {
  if (domain !== undefined) {
    traceInput.update((cur) => ({ ...cur, domain }));
  }
  openStore.set(true);
  let currentDomain = '';
  traceInput.subscribe((v) => { currentDomain = v.domain; })();
  updateURL(true, currentDomain);
}

export function closeTrace(): void {
  openStore.set(false);
  traceInput.set({ domain: '' });
  traceResult.set(null);
  traceError.set(null);
  updateURL(false, '');
}

export async function runTrace(): Promise<void> {
  let req: TraceInput = { domain: '' };
  traceInput.subscribe((v) => { req = v; })();
  if (!req.domain.trim()) return;

  traceLoading.set(true);
  traceError.set(null);
  try {
    const result = await api.singboxRouterInspectRoute({
      domain: req.domain.trim(),
      ...(req.port != null ? { port: req.port } : {}),
      ...(req.protocol ? { protocol: req.protocol } : {}),
    });
    traceResult.set(result);
    updateURL(true, req.domain.trim());
  } catch (e) {
    traceError.set(e instanceof Error ? e.message : String(e));
  } finally {
    traceLoading.set(false);
  }
}
