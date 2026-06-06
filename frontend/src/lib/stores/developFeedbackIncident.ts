import { writable } from 'svelte/store';

/** Set before navigating to diagnostics checks; consumed by ChecksTab. */
export const developFeedbackIncidentPending = writable(false);

export function requestDevelopFeedbackIncident(): void {
	developFeedbackIncidentPending.set(true);
}
