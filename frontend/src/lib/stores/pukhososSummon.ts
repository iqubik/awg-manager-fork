import { writable } from 'svelte/store';

/** Инкремент запускает патруль пухососа на странице настроек. */
export const pukhososSummonTick = writable(0);

export function summonPukhosos() {
	pukhososSummonTick.update((n) => n + 1);
}
