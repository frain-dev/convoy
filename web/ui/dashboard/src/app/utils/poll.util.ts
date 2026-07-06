/** Default backoff window for entitlement / billing activation polls. */
export const POLL_BUDGET_MS = 5 * 60 * 1000;
/** Shorter window for post-checkout subscription verification. */
export const CHECKOUT_POLL_BUDGET_MS = 60 * 1000;
export const POLL_INITIAL_DELAY_MS = 2000;
export const POLL_MAX_DELAY_MS = 30_000;
export const POLL_BACKOFF_FACTOR = 1.6;

export interface PollOptions<T> {
	request: () => Promise<T>;
	isDone: (result: T) => boolean;
	/** Total time budget before giving up. */
	budgetMs?: number;
	initialDelayMs?: number;
	maxDelayMs?: number;
	backoffFactor?: number;
	/** When true, wait one interval before the first request. */
	delayFirst?: boolean;
}

function delay(ms: number): Promise<void> {
	return new Promise(resolve => setTimeout(resolve, ms));
}

/**
 * Poll `request` with exponential backoff until `isDone` is satisfied or the
 * budget expires.
 *
 * Failure policy: transient request errors are swallowed and polling continues.
 * The caller decides what exhausting the budget means (returns false).
 */
export async function pollUntil<T>(options: PollOptions<T>): Promise<boolean> {
	const budgetMs = options.budgetMs ?? CHECKOUT_POLL_BUDGET_MS;
	const initialDelayMs = options.initialDelayMs ?? POLL_INITIAL_DELAY_MS;
	const maxDelayMs = options.maxDelayMs ?? POLL_MAX_DELAY_MS;
	const backoffFactor = options.backoffFactor ?? POLL_BACKOFF_FACTOR;
	const deadline = Date.now() + budgetMs;
	let waitMs = options.delayFirst ? initialDelayMs : 0;

	while (Date.now() < deadline) {
		if (waitMs > 0) {
			await delay(Math.min(waitMs, Math.max(0, deadline - Date.now())));
			if (Date.now() >= deadline) break;
		}

		try {
			const result = await options.request();
			if (options.isDone(result)) return true;
		} catch {
			// Swallow transient errors and keep polling.
		}

		waitMs = Math.min(
			waitMs === 0 ? initialDelayMs : waitMs * backoffFactor,
			maxDelayMs
		);
	}

	return false;
}
