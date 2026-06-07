/**
 * Open/closed state для StatusDrawer.
 * PageShell pill onclick → openDrawer; SideDrawer onClose → closeDrawer.
 */
import { writable, type Readable } from 'svelte/store';

const store = writable<boolean>(false);

/** Readable view — для $drawerOpen в template'ах. */
export const drawerOpen: Readable<boolean> = { subscribe: store.subscribe };

export function openDrawer(): void {
  store.set(true);
}

export function closeDrawer(): void {
  store.set(false);
}

export function toggleDrawer(): void {
  store.update((v) => !v);
}
