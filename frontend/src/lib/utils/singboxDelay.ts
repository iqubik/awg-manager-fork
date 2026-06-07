export const SINGBOX_DELAY_OK_MS = 200;
export const SINGBOX_DELAY_SLOW_MS = 500;

export type SingboxDelayState = 'ok' | 'slow' | 'fail' | 'unknown' | 'stopped';

export interface SingboxDelayInput {
	latest: number | undefined;
	history: number[];
	running?: boolean;
}

export function singboxDelayState(input: SingboxDelayInput): SingboxDelayState {
	const { latest, history, running } = input;
	if (running === false) return 'stopped';
	if (latest === undefined || latest < 0) return 'unknown';
	const hasConsecutiveTimeout =
		history.length >= 2 &&
		history[history.length - 1] <= 0 &&
		history[history.length - 2] <= 0;
	if (latest <= 0) return hasConsecutiveTimeout ? 'fail' : 'slow';
	if (latest < SINGBOX_DELAY_OK_MS) return 'ok';
	if (latest < SINGBOX_DELAY_SLOW_MS) return 'slow';
	return 'slow';
}

export function singboxDelayLabel(state: SingboxDelayState, latest?: number): string {
	if (state === 'stopped') return 'stopped';
	if (state === 'unknown') return '—';
	if (state === 'fail') return 'timeout';
	if (latest !== undefined && latest <= 0) return 'check...';
	if (latest !== undefined && latest > 0) return `${latest}ms`;
	return '—';
}

export function singboxDelayFromHistory(
	history: number[],
	options?: { running?: boolean },
): { state: SingboxDelayState; label: string; latest: number | undefined } {
	const latest = history.length > 0 ? history[history.length - 1] : undefined;
	const state = singboxDelayState({ latest, history, ...options });
	const label = singboxDelayLabel(state, latest);
	return { state, label, latest };
}
