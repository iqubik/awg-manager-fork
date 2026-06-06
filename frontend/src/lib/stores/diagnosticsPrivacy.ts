import { writable } from 'svelte/store';

const STORAGE_KEY = 'awgm.diagnostics.sanitizeLogs';

function readInitial(): boolean {
  if (typeof localStorage === 'undefined') return true;
  const raw = localStorage.getItem(STORAGE_KEY);
  if (raw === null) return true;
  return raw !== '0';
}

function persist(value: boolean) {
  if (typeof localStorage === 'undefined') return;
  localStorage.setItem(STORAGE_KEY, value ? '1' : '0');
}

function createDiagnosticsSanitized() {
  const { subscribe, set, update } = writable<boolean>(readInitial());

  return {
    subscribe,
    set(value: boolean) {
      persist(value);
      set(value);
    },
    toggle() {
      update((value) => {
        const next = !value;
        persist(next);
        return next;
      });
    },
  };
}

export const diagnosticsSanitized = createDiagnosticsSanitized();

export function toggleDiagnosticsSanitized() {
  diagnosticsSanitized.toggle();
}
