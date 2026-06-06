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
 *   - URL ?mode= (replaceState — без новой записи в history)
 *   - localStorage
 *
 * Ошибки localStorage/history (private mode, restricted) тихо игнорируются.
 */
export function setMode(next: RouterMode): void {
  if (!isValid(next)) return;
  store.set(next);

  if (typeof window === 'undefined') return;

  try {
    const url = new URL(window.location.href);
    url.searchParams.set('mode', next);
    window.history.replaceState({}, '', url.toString());
  } catch {
    // restricted history — ignore
  }
  try {
    window.localStorage.setItem(STORAGE_KEY, next);
  } catch {
    // private mode — ignore
  }
}
