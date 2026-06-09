export function maskSensitive(value: unknown): string {
	const raw = value == null ? '' : String(value);
	return raw.trim() ? '********' : '';
}
