export const POLL_MAX_ATTEMPTS = 30;
export const POLL_INTERVAL_MS = 2000;

export interface PollOptions<T> {
	request: () => Promise<T>;
	isDone: (result: T) => boolean;
	maxAttempts?: number;
	intervalMs?: number;
	/** When true, wait one interval before the first request (status polling). */
	delayFirst?: boolean;
}

function delay(ms: number): Promise<void> {
	return new Promise(resolve => setTimeout(resolve, ms));
}

/**
 * Poll `request` until `isDone` is satisfied or attempts run out.
 *
 * Failure policy: transient request errors are swallowed and polling continues,
 * matching the original empty-catch loops. The caller decides what exhausting
 * all attempts means (returns false), so timeout handling stays explicit.
 */
export async function pollUntil<T>(options: PollOptions<T>): Promise<boolean> {
	const maxAttempts = options.maxAttempts ?? POLL_MAX_ATTEMPTS;
	const intervalMs = options.intervalMs ?? POLL_INTERVAL_MS;

	for (let attempt = 0; attempt < maxAttempts; attempt++) {
		if (options.delayFirst) await delay(intervalMs);
		try {
			const result = await options.request();
			if (options.isDone(result)) return true;
		} catch {
			// Swallow transient errors and keep polling.
		}
		if (!options.delayFirst) await delay(intervalMs);
	}

	return false;
}
