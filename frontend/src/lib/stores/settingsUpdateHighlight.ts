import { writable } from 'svelte/store';

const HIGHLIGHT_MS = 2800;

function createSettingsUpdateHighlight() {
	const { subscribe, set } = writable(false);
	let timer: ReturnType<typeof setTimeout> | null = null;

	return {
		subscribe,
		pulse() {
			if (timer) clearTimeout(timer);
			set(true);
			timer = setTimeout(() => {
				set(false);
				timer = null;
			}, HIGHLIGHT_MS);
		},
	};
}

/** One-shot glow on the AWGM update card (triggered from the header version badge). */
export const settingsUpdateHighlight = createSettingsUpdateHighlight();
