/**
 * Open/closed state для SourceDrawer (источник трафика / NDMS policy).
 */
import { writable, type Readable } from 'svelte/store';

const store = writable<boolean>(false);

export const sourceDrawerOpen: Readable<boolean> = { subscribe: store.subscribe };

export function openSourceDrawer(): void {
  store.set(true);
}

export function closeSourceDrawer(): void {
  store.set(false);
}
