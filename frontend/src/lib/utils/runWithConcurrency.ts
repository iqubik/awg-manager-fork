/** Run async tasks with at most `limit` in flight. */
export async function runWithConcurrency<T>(
	items: readonly T[],
	limit: number,
	fn: (item: T) => Promise<void>,
): Promise<void> {
	if (items.length === 0) return;
	const n = Math.max(1, Math.min(limit, items.length));
	let i = 0;
	async function worker(): Promise<void> {
		while (i < items.length) {
			const item = items[i++];
			await fn(item);
		}
	}
	await Promise.all(Array.from({ length: n }, () => worker()));
}
