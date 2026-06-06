import { goto } from '$app/navigation';
import { writable, type Readable } from 'svelte/store';

export type RouterMode = 'beginner' | 'expert';

const STORAGE_KEY = 'awg.sb-router.mode';
const VALID: ReadonlyArray<RouterMode> = ['beginner', 'expert'];

function isValid(v: unknown): v is RouterMode {
  return typeof v === 'string' && (VALID as readonly string[]).includes(v);
}

function readFromURL(): RouterMode | null {
  if (typeof window === 'undefined') return null;
  const v = new URL(window.location.href).searchParams.get('mode');
  return isValid(v) ? v : null;
}

function readFromStorage(): RouterMode | null {
  if (typeof window === 'undefined') return null;
  try {
    const v = window.localStorage.getItem(STORAGE_KEY);
    return isValid(v) ? v : null;
  } catch {
    return null; // private mode etc.
  }
}

function readInitialMode(): RouterMode {
  return readFromURL() ?? readFromStorage() ?? 'beginner';
}

const store = writable<RouterMode>(readInitialMode());

/** Readable view — компоненты используют через `$mode`. */
export const mode: Readable<RouterMode> = { subscribe: store.subscribe };

/**
 * Обновляет mode atomically:
 *   - in-memory store
 *   - URL ?mode= (via SvelteKit goto — без новой записи в history)
 *   - localStorage
 *
 * Ошибки localStorage/navigation (private mode, restricted) тихо игнорируются.
 */
export function setMode(next: RouterMode): void {
  if (!isValid(next)) return;
  store.set(next);

  if (typeof window === 'undefined') return;

  try {
    const url = new URL(window.location.href);
    url.searchParams.set('mode', next);
    const search = url.searchParams.toString();
    const target = url.pathname + (search ? `?${search}` : '') + url.hash;
    void goto(target, { replaceState: true, keepFocus: true, noScroll: true });
  } catch {
    // restricted navigation — ignore
  }
  try {
    window.localStorage.setItem(STORAGE_KEY, next);
  } catch {
    // private mode — ignore
  }
}
